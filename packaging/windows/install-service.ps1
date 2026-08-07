param(
  [string]$Binary = "$PSScriptRoot\..\..\knotroute.exe",
  [string]$DataDir = "$env:ProgramData\KnotRoute"
)
$ErrorActionPreference = "Stop"
$Binary = (Resolve-Path $Binary).Path
New-Item -ItemType Directory -Force $DataDir | Out-Null
$Config = Join-Path $DataDir "knotroute.json"
if (-not (Test-Path $Config)) { & $Binary init --config $Config }
$Command = '"{0}" run --config "{1}"' -f $Binary,$Config
if (Get-Service KnotRoute -ErrorAction SilentlyContinue) {
  sc.exe stop KnotRoute | Out-Null
  sc.exe delete KnotRoute | Out-Null
  Start-Sleep -Seconds 1
}
sc.exe create KnotRoute binPath= $Command start= auto DisplayName= "KnotRoute Overlay" | Out-Null
sc.exe description KnotRoute "Encrypted multi-hop private service overlay" | Out-Null
sc.exe failure KnotRoute reset= 86400 actions= restart/3000/restart/10000/""/0 | Out-Null
sc.exe start KnotRoute | Out-Null
Write-Host "KnotRoute service installed. Config: $Config"
