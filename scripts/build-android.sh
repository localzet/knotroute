#!/usr/bin/env sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
VERSION="${VERSION:-3.0.0}"
: "${ANDROID_HOME:?ANDROID_HOME must point to the Android SDK}"
command -v gomobile >/dev/null 2>&1 || { echo "gomobile is required: go install golang.org/x/mobile/cmd/gomobile@latest" >&2; exit 1; }
if [ -x "$ROOT/android/gradlew" ]; then
  GRADLE="$ROOT/android/gradlew"
elif command -v gradle >/dev/null 2>&1; then
  GRADLE="$(command -v gradle)"
else
  echo "Gradle 9.5+ is required (Android Studio can provide it)" >&2
  exit 1
fi
mkdir -p "$ROOT/android/app/libs" "$ROOT/dist"
cd "$ROOT"
gomobile bind -target=android -androidapi 26 -o android/app/libs/knotroute-client.aar ./mobile/knotmobile
cd android
"$GRADLE" --no-daemon :app:assembleDebug :app:assembleRelease
cp app/build/outputs/apk/debug/app-debug.apk "$ROOT/dist/knotroute_${VERSION}_android_debug.apk"
cp app/build/outputs/apk/release/app-release-unsigned.apk "$ROOT/dist/knotroute_${VERSION}_android_release-unsigned.apk"
cp app/libs/knotroute-client.aar "$ROOT/dist/knotroute-client_${VERSION}_android.aar"
