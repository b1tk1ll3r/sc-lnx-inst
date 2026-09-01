#!/usr/bin/env bash
set -euo pipefail

: "${GITEA_API_URL:?GITEA_API_URL is required}"
: "${GITEA_REPOSITORY:?GITEA_REPOSITORY is required (owner/repo)}"
: "${GITEA_TOKEN:?GITEA_TOKEN is required}"

api_base="${GITEA_API_URL%/}/repos/${GITEA_REPOSITORY}"
auth=( -H "Authorization: token ${GITEA_TOKEN}" )

select_json_backend() {
  case "${CITIZEN_JSON_BACKEND:-auto}" in
    python|python3)
      command -v python3 >/dev/null 2>&1 || { echo "python3 is required by CITIZEN_JSON_BACKEND" >&2; exit 2; }
      printf '%s\n' python
      ;;
    jq)
      command -v jq >/dev/null 2>&1 || { echo "jq is required by CITIZEN_JSON_BACKEND" >&2; exit 2; }
      printf '%s\n' jq
      ;;
    auto|"")
      if command -v python3 >/dev/null 2>&1; then
        printf '%s\n' python
      elif command -v jq >/dev/null 2>&1; then
        printf '%s\n' jq
      else
        echo "Gitea release helper needs python3 or jq" >&2
        exit 2
      fi
      ;;
    *)
      echo "Unknown CITIZEN_JSON_BACKEND=${CITIZEN_JSON_BACKEND}" >&2
      exit 2
      ;;
  esac
}

JSON_BACKEND="$(select_json_backend)"

urlencode() {
  if [[ "$JSON_BACKEND" == python ]]; then
    python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1], safe=""))' "$1"
  else
    jq -rn --arg value "$1" '$value | @uri'
  fi
}

release_json() {
  local tag="$1" enc
  enc="$(urlencode "$tag")"
  curl -fsS "${auth[@]}" "${api_base}/releases/tags/${enc}"
}

release_payload() {
  local tag="$1" title="$2" body_file="$3" target="${4:-}" body=""
  [[ -f "$body_file" ]] && body="$(cat "$body_file")"
  if [[ "$JSON_BACKEND" == python ]]; then
    python3 - "$tag" "$title" "$body_file" "$target" <<'PY'
import json, pathlib, sys
body_path = pathlib.Path(sys.argv[3])
body = body_path.read_text(encoding='utf-8') if body_path.exists() else ''
print(json.dumps({
    'tag_name': sys.argv[1],
    'name': sys.argv[2],
    'body': body,
    'draft': False,
    'prerelease': False,
    'target_commitish': sys.argv[4],
}))
PY
  else
    jq -cn \
      --arg tag "$tag" \
      --arg title "$title" \
      --arg body "$body" \
      --arg target "$target" \
      '{tag_name:$tag,name:$title,body:$body,draft:false,prerelease:false,target_commitish:$target}'
  fi
}

json_release_id() {
  if [[ "$JSON_BACKEND" == python ]]; then
    python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])'
  else
    jq -er '.id'
  fi
}

json_matching_asset_ids() {
  local name="$1"
  if [[ "$JSON_BACKEND" == python ]]; then
    python3 -c 'import json,sys; name=sys.argv[1]; [print(a["id"]) for a in json.load(sys.stdin).get("assets",[]) if a.get("name")==name]' "$name"
  else
    jq -er --arg name "$name" '.assets[]? | select(.name == $name) | .id' 2>/dev/null || true
  fi
}

json_downloadable_assets_tsv() {
  if [[ "$JSON_BACKEND" == python ]]; then
    python3 -c '
import json, sys
release = json.load(sys.stdin)
for asset in release.get("assets", []):
    name = asset.get("name", "")
    url = asset.get("browser_download_url", "")
    if not name or not url or name == "SHA256SUMS.txt":
        continue
    if name.endswith(".deb") or name.endswith(".rpm") or name.endswith(".pkg.tar.zst") or name.endswith("-linux-amd64.tar.gz"):
        print(f"{name}\t{url}")
'
  else
    jq -r '
      .assets[]?
      | select(.name != null and .browser_download_url != null and .name != "SHA256SUMS.txt")
      | select(.name | endswith(".deb") or endswith(".rpm") or endswith(".pkg.tar.zst") or endswith("-linux-amd64.tar.gz"))
      | [.name, .browser_download_url]
      | @tsv
    '
  fi
}

ensure_release() {
  local tag="$1" title="$2" body_file="$3" target="${4:-}"
  local payload current id
  payload="$(release_payload "$tag" "$title" "$body_file" "$target")"
  if current="$(release_json "$tag" 2>/dev/null)"; then
    id="$(printf '%s' "$current" | json_release_id)"
    curl -fsS "${auth[@]}" -H 'Content-Type: application/json' \
      -X PATCH -d "$payload" "${api_base}/releases/${id}" >/dev/null
    echo "Updated Gitea release ${tag}"
    return 0
  fi
  curl -fsS "${auth[@]}" -H 'Content-Type: application/json' \
    -X POST -d "$payload" "${api_base}/releases" >/dev/null
  echo "Created Gitea release ${tag}"
}

upload_asset() {
  local tag="$1" file="$2"
  [[ -s "$file" ]] || { echo "Asset missing/empty: $file" >&2; exit 1; }
  local rel id name enc existing_ids
  rel="$(release_json "$tag")"
  id="$(printf '%s' "$rel" | json_release_id)"
  name="$(basename "$file")"
  enc="$(urlencode "$name")"
  existing_ids="$(printf '%s' "$rel" | json_matching_asset_ids "$name")"
  if [[ -n "$existing_ids" ]]; then
    while IFS= read -r aid; do
      [[ -n "$aid" ]] || continue
      curl -fsS "${auth[@]}" -X DELETE "${api_base}/releases/${id}/assets/${aid}" >/dev/null
    done <<< "$existing_ids"
  fi
  curl -fsS "${auth[@]}" -X POST \
    -F "attachment=@${file}" \
    "${api_base}/releases/${id}/assets?name=${enc}" >/dev/null
  echo "Uploaded ${name}"
}

download_assets() {
  local tag="$1" dir="$2"
  mkdir -p "$dir"
  local rel name url
  rel="$(release_json "$tag")"
  while IFS=$'\t' read -r name url; do
    [[ -n "$name" && -n "$url" ]] || continue
    curl -fsSL "${auth[@]}" "$url" -o "$dir/$name"
    echo "$name"
  done < <(printf '%s' "$rel" | json_downloadable_assets_tsv)
}

case "${1:-}" in
  ensure)
    [[ $# -ge 4 ]] || { echo "usage: $0 ensure TAG TITLE BODY_FILE [TARGET]" >&2; exit 2; }
    ensure_release "$2" "$3" "$4" "${5:-}"
    ;;
  upload)
    [[ $# -eq 3 ]] || { echo "usage: $0 upload TAG FILE" >&2; exit 2; }
    upload_asset "$2" "$3"
    ;;
  download-assets)
    [[ $# -eq 3 ]] || { echo "usage: $0 download-assets TAG DIR" >&2; exit 2; }
    download_assets "$2" "$3"
    ;;
  *)
    echo "usage: $0 {ensure|upload|download-assets} ..." >&2
    exit 2
    ;;
esac
