#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
VERSION="${VERSION:-dev}"
OUT="${OUT:-dist}"
case "$OUT" in
  /*) OUT_DIR="$OUT" ;;
  *) OUT_DIR="$ROOT/$OUT" ;;
esac

mkdir -p "$OUT_DIR"
cd "$ROOT"
LDFLAGS="-s -w -X github.com/localzet/knotroute/internal/overlay.Version=$VERSION"
OPS_LDFLAGS="-s -w -X github.com/localzet/knotroute/internal/ops.Version=$VERSION"

for target in linux/amd64 linux/arm64 windows/amd64 windows/arm64 darwin/amd64 darwin/arm64; do
  GOOS=${target%/*}
  GOARCH=${target#*/}
  EXT=""
  [ "$GOOS" = windows ] && EXT=".exe"
  NAME="knotroute_${VERSION}_${GOOS}_${GOARCH}"
  TMP=$(mktemp -d)
  trap 'rm -rf "$TMP"' EXIT INT TERM

  echo "building $NAME"
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -ldflags="$LDFLAGS" -o "$TMP/knotroute$EXT" ./cmd/knotroute
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -ldflags="$LDFLAGS" -o "$TMP/knotroute-beacon$EXT" ./cmd/knotroute-beacon
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -ldflags="$LDFLAGS" -o "$TMP/knotroute-sidecar$EXT" ./cmd/knotroute-sidecar
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -ldflags="$OPS_LDFLAGS" -o "$TMP/knotroute-control$EXT" ./cmd/knotroute-control
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -ldflags="$OPS_LDFLAGS" -o "$TMP/knotroute-agent$EXT" ./cmd/knotroute-agent

  if [ "$GOOS" = windows ]; then
    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -ldflags="$LDFLAGS -H=windowsgui" -o "$TMP/knotroute-desktop.exe" ./cmd/knotroute-desktop
    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -ldflags="$LDFLAGS -H=windowsgui" -o "$TMP/knotroute-service.exe" ./cmd/knotroute-service
    cp packaging/windows/Install-KnotRoute.ps1 packaging/windows/Uninstall-KnotRoute.ps1 "$TMP/"
    mkdir -p "$TMP/service"
    cp packaging/windows/install-service.ps1 packaging/windows/uninstall-service.ps1 "$TMP/service/"
    cp docs/windows.md "$TMP/README-WINDOWS.md"
  fi

  cp README.md LICENSE "$TMP/"
  rm -f "$OUT_DIR/$NAME.zip" "$OUT_DIR/$NAME.tar.gz"
  if [ "$GOOS" = windows ]; then
    (cd "$TMP" && zip -q -r "$OUT_DIR/$NAME.zip" .)
  else
    tar -C "$TMP" -czf "$OUT_DIR/$NAME.tar.gz" .
  fi

  rm -rf "$TMP"
  trap - EXIT INT TERM
done

SOURCE_TMP=$(mktemp -d)
trap 'rm -rf "$SOURCE_TMP"' EXIT INT TERM
SOURCE_DIR="$SOURCE_TMP/knotroute-$VERSION"
mkdir -p "$SOURCE_DIR"
tar -C "$ROOT" \
  --exclude='./.git' \
  --exclude='./dist' \
  --exclude='./bin' \
  --exclude='./android/.gradle' \
  --exclude='./android/app/build' \
  --exclude='./android/app/libs/*.aar' \
  --exclude='./web/node_modules' \
  --exclude='./internal/overlay/peers.json' \
  --exclude='*.log' \
  --exclude='*.tmp' \
  -cf - . | tar -C "$SOURCE_DIR" -xf -
rm -f "$OUT_DIR/knotroute_${VERSION}_source.zip"
(cd "$SOURCE_TMP" && zip -q -r "$OUT_DIR/knotroute_${VERSION}_source.zip" "knotroute-$VERSION")
rm -rf "$SOURCE_TMP"
trap - EXIT INT TERM

(
  cd "$OUT_DIR"
  rm -f SHA256SUMS
  find . -maxdepth 1 -type f ! -name SHA256SUMS -printf '%f\0' | sort -z | xargs -0 sha256sum > SHA256SUMS
)

echo "release artifacts written to $OUT_DIR"
