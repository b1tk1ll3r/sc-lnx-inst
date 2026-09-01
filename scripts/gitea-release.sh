#!/usr/bin/env bash
set -euo pipefail

: "${GITEA_API_URL:?GITEA_API_URL is required}"
: "${GITEA_REPOSITORY:?GITEA_REPOSITORY is required (owner/repo)}"
: "${GITEA_TOKEN:?GITEA_TOKEN is required}"

api_base="${GITEA_API_URL%/}/repos/${GITEA_REPOSITORY}"
auth=( -H "Authorization: token ${GITEA_TOKEN}" )

urlencode() {
  python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1], safe=""))' "$1"
}

release_json() {
  local tag="$1" enc
  enc="$(urlencode "$tag")"
  curl -fsS "${auth[@]}" "${api_base}/releases/tags/${enc}"
}

release_payload() {
  local tag="$1" title="$2" body_file="$3" target="${4:-}"
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
}

ensure_release() {
  local tag="$1" title="$2" body_file="$3" target="${4:-}"
  local payload current id
  payload="$(release_payload "$tag" "$title" "$body_file" "$target")"
  if current="$(release_json "$tag" 2>/dev/null)"; then
    id="$(printf '%s' "$current" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
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
  id="$(printf '%s' "$rel" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
  name="$(basename "$file")"
  enc="$(urlencode "$name")"
  existing_ids="$(printf '%s' "$rel" | python3 -c 'import json,sys; name=sys.argv[1]; [print(a["id"]) for a in json.load(sys.stdin).get("assets",[]) if a.get("name")==name]' "$name")"
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
  local rel
  rel="$(release_json "$tag")"
  printf '%s' "$rel" | python3 -c '
import json, os, pathlib, sys, urllib.request
out = pathlib.Path(sys.argv[1])
release = json.load(sys.stdin)
for asset in release.get("assets", []):
    name = asset.get("name", "")
    if not name or name == "SHA256SUMS.txt":
        continue
    if not (name.endswith(".deb") or name.endswith(".rpm") or name.endswith(".pkg.tar.zst") or name.endswith("-linux-amd64.tar.gz")):
        continue
    req = urllib.request.Request(asset["browser_download_url"])
    token = os.environ.get("GITEA_TOKEN", "")
    if token:
        req.add_header("Authorization", f"token {token}")
    with urllib.request.urlopen(req, timeout=120) as r, open(out / name, "wb") as f:
        while True:
            block = r.read(1024 * 1024)
            if not block:
                break
            f.write(block)
    print(name)
' "$dir"
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
