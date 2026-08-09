param(
  [Parameter(Mandatory = $true)][string]$Version,
  [ValidateSet("amd64", "arm64")][string]$Arch = "amd64",
  [string]$Out = "dist"
)
$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$OutDir = if ([IO.Path]::IsPathRooted($Out)) { $Out } else { Join-Path $Root $Out }
New-Item -ItemType Directory -Force $OutDir | Out-Null
$Tmp = Join-Path $env:RUNNER_TEMP "knotroute-windows-$Arch"
if (Test-Path $Tmp) { Remove-Item -Recurse -Force $Tmp }
New-Item -ItemType Directory -Force $Tmp | Out-Null

$env:GOOS = "windows"
$env:GOARCH = $Arch
$env:CGO_ENABLED = "0"
$Ld = "-s -w -X github.com/localzet/knotroute/internal/overlay.Version=$Version"
$OpsLd = "-s -w -X github.com/localzet/knotroute/internal/ops.Version=$Version"

Push-Location $Root
try {
  go build -trimpath -ldflags $Ld -o (Join-Path $Tmp "knotroute-cli.exe") ./cmd/knotroute
  go build -trimpath -ldflags $Ld -o (Join-Path $Tmp "knotroute-beacon.exe") ./cmd/knotroute-beacon
  go build -trimpath -ldflags $Ld -o (Join-Path $Tmp "knotroute-sidecar.exe") ./cmd/knotroute-sidecar
  go build -trimpath -ldflags $OpsLd -o (Join-Path $Tmp "knotroute-control.exe") ./cmd/knotroute-control
  go build -trimpath -ldflags $OpsLd -o (Join-Path $Tmp "knotroute-agent.exe") ./cmd/knotroute-agent
  go build -trimpath -ldflags "$Ld -H=windowsgui" -o (Join-Path $Tmp "knotroute-service.exe") ./cmd/knotroute-service

  Push-Location (Join-Path $Root "cmd/knotroute-desktop")
  try {
    wails build -clean -platform "windows/$Arch" -webview2 embed -trimpath -ldflags $Ld -o KnotRoute.exe
    Copy-Item "build/bin/KnotRoute.exe" (Join-Path $Tmp "KnotRoute.exe") -Force
  } finally { Pop-Location }

  Copy-Item "packaging/windows/Install-KnotRoute.ps1" $Tmp
  Copy-Item "packaging/windows/Uninstall-KnotRoute.ps1" $Tmp
  New-Item -ItemType Directory -Force (Join-Path $Tmp "service") | Out-Null
  Copy-Item "packaging/windows/install-service.ps1" (Join-Path $Tmp "service")
  Copy-Item "packaging/windows/uninstall-service.ps1" (Join-Path $Tmp "service")
  Copy-Item "docs/windows.md" (Join-Path $Tmp "README-WINDOWS.md")
  Copy-Item "README.md" $Tmp
  Copy-Item "LICENSE" $Tmp

  $Zip = Join-Path $OutDir "knotroute_${Version}_windows_${Arch}.zip"
  if (Test-Path $Zip) { Remove-Item -Force $Zip }
  Compress-Archive -Path (Join-Path $Tmp "*") -DestinationPath $Zip -CompressionLevel Optimal
  Write-Host "Windows release written to $Zip"
} finally {
  Pop-Location
}
