#!/usr/bin/env sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
VERSION=$(tr -d '\r\n' < "$ROOT/VERSION")
fail() { echo "version consistency check failed: $*" >&2; exit 1; }
[ -n "$VERSION" ] || fail "VERSION is empty"

grep -Fq 'VERSION ?= $(shell cat VERSION)' "$ROOT/Makefile" || fail "Makefile does not use VERSION file"
grep -Fq "var Version = \"$VERSION\"" "$ROOT/internal/overlay/node.go" || fail "overlay default version differs"
grep -Fq "var Version = \"$VERSION\"" "$ROOT/internal/ops/model.go" || fail "ops default version differs"
grep -Fq "\"version\": \"$VERSION\"" "$ROOT/web/package.json" || fail "web package version differs"
grep -Fq "\"version\": \"$VERSION\"" "$ROOT/docs-site/package.json" || fail "docs-site package version differs"
grep -Fq "orElse(\"$VERSION\")" "$ROOT/android/app/build.gradle.kts" || fail "Android fallback version differs"
for file in Dockerfile Dockerfile.agent Dockerfile.beacon Dockerfile.control Dockerfile.docs Dockerfile.sidecar; do
  grep -Fq "ARG VERSION=$VERSION" "$ROOT/$file" || fail "$file default version differs"
done
grep -Fq 'VERSION="${VERSION:-$(cat "$ROOT/VERSION")}"' "$ROOT/scripts/build-release.sh" || fail "build-release.sh does not use VERSION file"
grep -Fq 'VERSION="${VERSION:-$(cat "$ROOT/VERSION")}"' "$ROOT/scripts/build-android.sh" || fail "build-android.sh does not use VERSION file"

echo "KnotRoute version consistency: $VERSION"
