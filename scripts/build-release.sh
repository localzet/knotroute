#!/usr/bin/env sh
set -eu
VERSION="${VERSION:-1.0.0}"
OUT="${OUT:-dist}"
mkdir -p "$OUT"
for target in linux/amd64 linux/arm64 windows/amd64 windows/arm64 darwin/amd64 darwin/arm64; do
  GOOS=${target%/*}; GOARCH=${target#*/}; EXT=""
  [ "$GOOS" = windows ] && EXT=".exe"
  NAME="knotroute_${VERSION}_${GOOS}_${GOARCH}"
  TMP=$(mktemp -d)
  echo "building $NAME"
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -ldflags="-s -w" -o "$TMP/knotroute$EXT" ./cmd/knotroute
  cp README.md LICENSE "$TMP/"
  if [ "$GOOS" = windows ]; then
    (cd "$TMP" && zip -q -r "$OLDPWD/$OUT/$NAME.zip" .)
  else
    tar -C "$TMP" -czf "$OUT/$NAME.tar.gz" .
  fi
  rm -rf "$TMP"
done
sha256sum "$OUT"/* > "$OUT/SHA256SUMS"
