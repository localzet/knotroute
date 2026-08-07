$ErrorActionPreference = "Stop"
$Version = if ($env:VERSION) { $env:VERSION } else { "1.0.0" }
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
  Copy-Item README.md,LICENSE $temp
  if ($goos -eq "windows") { Compress-Archive -Path "$temp/*" -DestinationPath (Join-Path $Out "$name.zip") -Force }
  else { tar -C $temp -czf (Join-Path $Out "$name.tar.gz") . }
  Remove-Item -Recurse -Force $temp
}
Get-ChildItem $Out -File | Get-FileHash -Algorithm SHA256 | ForEach-Object { "$($_.Hash.ToLower())  $($_.Path | Split-Path -Leaf)" } | Set-Content (Join-Path $Out "SHA256SUMS")
