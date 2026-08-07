#!/usr/bin/env sh
set -eu
BIN="${BIN:-./bin/knotroute}"
ROOT="${TMPDIR:-/tmp}/knotroute-demo-$$"
mkdir -p "$ROOT"/{a,b,c}
cleanup(){
  [ -n "${A_PID:-}" ] && kill "$A_PID" 2>/dev/null || true
  [ -n "${B_PID:-}" ] && kill "$B_PID" 2>/dev/null || true
  [ -n "${C_PID:-}" ] && kill "$C_PID" 2>/dev/null || true
  [ -n "${ECHO_PID:-}" ] && kill "$ECHO_PID" 2>/dev/null || true
  rm -rf "$ROOT"
}
trap cleanup EXIT INT TERM

[ -x "$BIN" ] || { echo "build first: make build" >&2; exit 1; }
"$BIN" init --config "$ROOT/b/knotroute.json" --listen 127.0.0.1:17447 --dashboard ""
"$BIN" init --config "$ROOT/c/knotroute.json" --listen 127.0.0.1:17448 --dashboard ""
"$BIN" init --config "$ROOT/a/knotroute.json" --listen 127.0.0.1:17449 --dashboard 127.0.0.1:18484
C_ID=$("$BIN" id --config "$ROOT/c/knotroute.json")
python3 -m http.server 18080 --bind 127.0.0.1 --directory "$ROOT" >/dev/null 2>&1 & ECHO_PID=$!
python3 - "$ROOT" "$C_ID" <<'PY'
import json, pathlib, sys
root=pathlib.Path(sys.argv[1]); c_id=sys.argv[2]
def patch(name, fn):
    p=root/name/'knotroute.json'; d=json.loads(p.read_text()); fn(d); p.write_text(json.dumps(d,indent=2)+'\n')
patch('c',lambda d:(d.__setitem__('peers',[{'address':'127.0.0.1:17447'}]),d.__setitem__('services',[{'name':'web','target':'127.0.0.1:18080','allow':['*']}])) )
patch('a',lambda d:(d.__setitem__('peers',[{'address':'127.0.0.1:17447'}]),d.__setitem__('forwards',[{'listen':'127.0.0.1:18081','node':c_id,'service':'web'}])) )
PY
"$BIN" run --config "$ROOT/b/knotroute.json" >"$ROOT/b.log" 2>&1 & B_PID=$!
"$BIN" run --config "$ROOT/c/knotroute.json" >"$ROOT/c.log" 2>&1 & C_PID=$!
"$BIN" run --config "$ROOT/a/knotroute.json" >"$ROOT/a.log" 2>&1 & A_PID=$!
sleep 2
printf 'A dashboard: http://127.0.0.1:18484\nMulti-hop HTTP forward: http://127.0.0.1:18081\nPress Ctrl+C to stop.\n'
wait "$A_PID"
