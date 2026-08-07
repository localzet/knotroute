$ErrorActionPreference = "Stop"
$Version = if ($env:VERSION) { $env:VERSION } else { "2.0.0" }
$Out = if ($env:OUT) { $env:OUT } else { "dist" }
New-Item -ItemType Directory -Force $Out | Out-Null
$targets = @("windows/amd64", "windows/arm64", "linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64")
foreach ($target in $targets) {
  $parts = $target.Split("/"); $goos = $parts[0]; $goarch = $parts[1]
  $ext = if ($goos -eq "windows") { ".exe" } else { "" }
  $name = "knotroute_${Version}_${goos}_${goarch}"
  $temp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid())
  New-Item -ItemType Directory $temp | Out-Null
  $env:CGO_ENABLED="0"; $env:GOOS=$goos; $env:GOARCH=$goarch
  go build -trimpath -ldflags="-s -w" -o (Join-Path $temp "knotroute$ext") ./cmd/knotroute
  if ($goos -eq "windows") {
    go build -trimpath -ldflags="-s -w -H=windowsgui" -o (Join-Path $temp "knotroute-desktop.exe") ./cmd/knotroute-desktop
    go build -trimpath -ldflags="-s -w -H=windowsgui" -o (Join-Path $temp "knotroute-service.exe") ./cmd/knotroute-service
    Copy-Item packaging/windows/Install-KnotRoute.ps1,packaging/windows/Uninstall-KnotRoute.ps1 $temp
    New-Item -ItemType Directory -Force (Join-Path $temp "service") | Out-Null
    Copy-Item packaging/windows/install-service.ps1,packaging/windows/uninstall-service.ps1 (Join-Path $temp "service")
    Copy-Item docs/windows.md (Join-Path $temp "README-WINDOWS.md")
  }
  Copy-Item README.md,LICENSE $temp
  if ($goos -eq "windows") { Compress-Archive -Path "$temp/*" -DestinationPath (Join-Path $Out "$name.zip") -Force }
  else { tar -C $temp -czf (Join-Path $Out "$name.tar.gz") . }
  Remove-Item -Recurse -Force $temp
}
Get-ChildItem $Out -File | Get-FileHash -Algorithm SHA256 | ForEach-Object { "$($_.Hash.ToLower())  $($_.Path | Split-Path -Leaf)" } | Set-Content (Join-Path $Out "SHA256SUMS")
