#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
PID=""
cleanup() {
  [[ -z "$PID" ]] || kill "$PID" >/dev/null 2>&1 || true
  rm -rf "$TMP"
}
trap cleanup EXIT

cat > "$TMP/server.py" <<'PY'
import http.server, json, re, urllib.parse
assets=[]
release=None
next_asset=1
class H(http.server.BaseHTTPRequestHandler):
    def log_message(self,*a): pass
    def sendj(self, code, obj):
        b=json.dumps(obj).encode(); self.send_response(code); self.send_header('Content-Type','application/json'); self.send_header('Content-Length',str(len(b))); self.end_headers(); self.wfile.write(b)
    def rel(self):
        if release is None: return None
        r=dict(release)
        r['assets']=[{'id':a['id'],'name':a['name'],'size':len(a['data']),'browser_download_url':f'http://127.0.0.1:{self.server.server_address[1]}/download/{urllib.parse.quote(a["name"])}'} for a in assets]
        return r
    def do_GET(self):
        path=urllib.parse.urlparse(self.path).path
        if path.endswith('/releases/tags/v1.1.1'):
            if release is None: self.send_error(404); return
            self.sendj(200,self.rel()); return
        if path.startswith('/download/'):
            name=urllib.parse.unquote(path.split('/download/',1)[1])
            for a in assets:
                if a['name']==name:
                    b=a['data']; self.send_response(200); self.send_header('Content-Length',str(len(b))); self.end_headers(); self.wfile.write(b); return
            self.send_error(404); return
        self.send_error(404)
    def do_POST(self):
        global release,next_asset
        parsed=urllib.parse.urlparse(self.path); path=parsed.path
        if path.endswith('/releases'):
            n=int(self.headers.get('Content-Length','0')); payload=json.loads(self.rfile.read(n) or b'{}')
            release={'id':1,'tag_name':payload['tag_name'],'name':payload.get('name','')}; self.sendj(201,self.rel()); return
        if re.search(r'/releases/\d+/assets$',path):
            n=int(self.headers.get('Content-Length','0')); body=self.rfile.read(n)
            name=urllib.parse.parse_qs(parsed.query).get('name',['asset'])[0]
            pos=body.find(b'\r\n\r\n'); data=body[pos+4:] if pos>=0 else body
            ct=self.headers.get('Content-Type',''); boundary=ct.split('boundary=',1)[1].encode() if 'boundary=' in ct else b''
            marker=b'\r\n--'+boundary
            if boundary and marker in data: data=data.split(marker,1)[0]
            a={'id':next_asset,'name':name,'data':data}; next_asset+=1; assets.append(a)
            self.sendj(201,{'id':a['id'],'name':name}); return
        self.send_error(404)
    def do_PATCH(self):
        global release
        path=urllib.parse.urlparse(self.path).path
        if re.search(r'/releases/\d+$', path):
            n=int(self.headers.get('Content-Length','0')); payload=json.loads(self.rfile.read(n) or b'{}')
            release={'id':1,'tag_name':payload['tag_name'],'name':payload.get('name','')}
            self.sendj(200,self.rel()); return
        self.send_error(404)
    def do_DELETE(self):
        m=re.search(r'/releases/\d+/assets/(\d+)$',urllib.parse.urlparse(self.path).path)
        if m:
            aid=int(m.group(1)); assets[:]=[a for a in assets if a['id']!=aid]
            self.send_response(204); self.end_headers(); return
        self.send_error(404)
httpd=http.server.ThreadingHTTPServer(('127.0.0.1',0),H)
print(httpd.server_address[1], flush=True)
httpd.serve_forever()
PY

python3 "$TMP/server.py" > "$TMP/port" &
PID=$!
for _ in $(seq 1 50); do [[ -s "$TMP/port" ]] && break; sleep .05; done
PORT="$(cat "$TMP/port")"
export GITEA_API_URL="http://127.0.0.1:${PORT}/api/v1"
export GITEA_REPOSITORY="owner/repo"
export GITEA_TOKEN="test-token"

echo first > "$TMP/citizen-launcher_1.1.1_amd64.deb"
"$ROOT/scripts/gitea-release.sh" ensure v1.1.1 'Citizen Launcher 1.1.1' "$ROOT/RELEASE_NOTES.md" deadbeef
"$ROOT/scripts/gitea-release.sh" ensure v1.1.1 'Citizen Launcher 1.1.1' "$ROOT/RELEASE_NOTES.md" deadbeef
"$ROOT/scripts/gitea-release.sh" upload v1.1.1 "$TMP/citizen-launcher_1.1.1_amd64.deb"
echo replacement > "$TMP/citizen-launcher_1.1.1_amd64.deb"
"$ROOT/scripts/gitea-release.sh" upload v1.1.1 "$TMP/citizen-launcher_1.1.1_amd64.deb"
"$ROOT/scripts/gitea-release.sh" download-assets v1.1.1 "$TMP/download"
cmp "$TMP/citizen-launcher_1.1.1_amd64.deb" "$TMP/download/citizen-launcher_1.1.1_amd64.deb"
echo 'Gitea release helper verification: OK'
