$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Version = if ($env:VERSION) { $env:VERSION } else { "3.0.0" }
$Out = if ($env:OUT) { $env:OUT } else { "dist" }
$OutDir = if ([System.IO.Path]::IsPathRooted($Out)) { $Out } else { Join-Path $Root $Out }
New-Item -ItemType Directory -Force $OutDir | Out-Null
$targets = @("windows/amd64", "windows/arm64", "linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64")
$ldflags = "-s -w -X github.com/localzet/knotroute/internal/overlay.Version=$Version"

Push-Location $Root
try {
  foreach ($target in $targets) {
    $parts = $target.Split("/")
    $goos = $parts[0]
    $goarch = $parts[1]
    $ext = if ($goos -eq "windows") { ".exe" } else { "" }
    $name = "knotroute_${Version}_${goos}_${goarch}"
    $temp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid())
    New-Item -ItemType Directory $temp | Out-Null
    try {
      $env:CGO_ENABLED = "0"
      $env:GOOS = $goos
      $env:GOARCH = $goarch
      go build -trimpath -ldflags=$ldflags -o (Join-Path $temp "knotroute$ext") ./cmd/knotroute
      go build -trimpath -ldflags=$ldflags -o (Join-Path $temp "knotroute-beacon$ext") ./cmd/knotroute-beacon
      go build -trimpath -ldflags=$ldflags -o (Join-Path $temp "knotroute-sidecar$ext") ./cmd/knotroute-sidecar

      if ($goos -eq "windows") {
        go build -trimpath -ldflags="$ldflags -H=windowsgui" -o (Join-Path $temp "knotroute-desktop.exe") ./cmd/knotroute-desktop
        go build -trimpath -ldflags="$ldflags -H=windowsgui" -o (Join-Path $temp "knotroute-service.exe") ./cmd/knotroute-service
        Copy-Item packaging/windows/Install-KnotRoute.ps1, packaging/windows/Uninstall-KnotRoute.ps1 $temp
        New-Item -ItemType Directory -Force (Join-Path $temp "service") | Out-Null
        Copy-Item packaging/windows/install-service.ps1, packaging/windows/uninstall-service.ps1 (Join-Path $temp "service")
        Copy-Item docs/windows.md (Join-Path $temp "README-WINDOWS.md")
      }

      Copy-Item README.md, LICENSE $temp
      $zipPath = Join-Path $OutDir "$name.zip"
      $tarPath = Join-Path $OutDir "$name.tar.gz"
      Remove-Item $zipPath, $tarPath -Force -ErrorAction SilentlyContinue
      if ($goos -eq "windows") {
        Compress-Archive -Path "$temp/*" -DestinationPath $zipPath -Force
      } else {
        tar -C $temp -czf $tarPath .
      }
    } finally {
      Remove-Item -Recurse -Force $temp -ErrorAction SilentlyContinue
    }
  }

  $sourcePath = Join-Path $OutDir "knotroute_${Version}_source.zip"
  Remove-Item $sourcePath -Force -ErrorAction SilentlyContinue
  $sourceTemp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid())
  $sourceRoot = Join-Path $sourceTemp "knotroute-$Version"
  New-Item -ItemType Directory -Force $sourceRoot | Out-Null
  try {
    tar -C $Root --exclude=.git --exclude=dist --exclude=bin --exclude=android/.gradle --exclude=android/app/build --exclude=android/app/libs/*.aar --exclude=web/node_modules --exclude=internal/overlay/peers.json --exclude=*.log --exclude=*.tmp -cf - . | tar -C $sourceRoot -xf -
    Compress-Archive -Path $sourceRoot -DestinationPath $sourcePath -Force
  } finally {
    Remove-Item -Recurse -Force $sourceTemp -ErrorAction SilentlyContinue
  }

  $checksumPath = Join-Path $OutDir "SHA256SUMS"
  Remove-Item $checksumPath -Force -ErrorAction SilentlyContinue
  Get-ChildItem $OutDir -File |
    Where-Object { $_.Name -ne "SHA256SUMS" } |
    Sort-Object Name |
    Get-FileHash -Algorithm SHA256 |
    ForEach-Object { "$($_.Hash.ToLower())  $($_.Path | Split-Path -Leaf)" } |
    Set-Content -Encoding ascii $checksumPath
} finally {
  Pop-Location
}

Write-Host "Release artifacts written to $OutDir"
