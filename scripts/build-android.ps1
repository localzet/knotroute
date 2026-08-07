$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Version = if ($env:VERSION) { $env:VERSION } else { "3.0.0" }
if (-not $env:ANDROID_HOME) { throw "ANDROID_HOME must point to the Android SDK" }
if (-not (Get-Command gomobile -ErrorAction SilentlyContinue)) { throw "gomobile is required: go install golang.org/x/mobile/cmd/gomobile@latest" }
$Gradle = if (Test-Path "$Root/android/gradlew.bat") { "$Root/android/gradlew.bat" } elseif (Get-Command gradle -ErrorAction SilentlyContinue) { (Get-Command gradle).Source } else { throw "Gradle 9.5+ is required (Android Studio can provide it)" }
New-Item -ItemType Directory -Force "$Root/android/app/libs", "$Root/dist" | Out-Null
Push-Location $Root
try { gomobile bind -target=android -androidapi 26 -o android/app/libs/knotroute-client.aar ./mobile/knotmobile } finally { Pop-Location }
Push-Location "$Root/android"
try { & $Gradle --no-daemon :app:assembleDebug :app:assembleRelease } finally { Pop-Location }
Copy-Item "$Root/android/app/build/outputs/apk/debug/app-debug.apk" "$Root/dist/knotroute_${Version}_android_debug.apk"
Copy-Item "$Root/android/app/build/outputs/apk/release/app-release-unsigned.apk" "$Root/dist/knotroute_${Version}_android_release-unsigned.apk"
Copy-Item "$Root/android/app/libs/knotroute-client.aar" "$Root/dist/knotroute-client_${Version}_android.aar"
