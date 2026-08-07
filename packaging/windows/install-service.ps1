param(
  [string]$ServiceBinary = "$PSScriptRoot\..\knotroute-service.exe",
  [string]$DaemonBinary = "$PSScriptRoot\..\knotroute.exe",
  [string]$InstallDir = "$env:ProgramFiles\KnotRoute",
  [string]$DataDir = "$env:ProgramData\KnotRoute"
)
$ErrorActionPreference = "Stop"
$IsRu = [Globalization.CultureInfo]::CurrentUICulture.TwoLetterISOLanguageName -eq "ru"
function T([string]$En, [string]$Ru) { if ($IsRu) { $Ru } else { $En } }
if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  throw (T "Run this script from an elevated PowerShell window." "Запустите этот скрипт в PowerShell от имени администратора.")
}
New-Item -ItemType Directory -Force $InstallDir,$DataDir | Out-Null
Copy-Item (Resolve-Path $DaemonBinary) (Join-Path $InstallDir "knotroute.exe") -Force
Copy-Item (Resolve-Path $ServiceBinary) (Join-Path $InstallDir "knotroute-service.exe") -Force
$Daemon = Join-Path $InstallDir "knotroute.exe"
$Wrapper = Join-Path $InstallDir "knotroute-service.exe"
$Config = Join-Path $DataDir "knotroute.json"
if (-not (Test-Path $Config)) { & $Daemon init --config $Config }
$Command = '"{0}" --config "{1}"' -f $Wrapper,$Config
if (Get-Service KnotRoute -ErrorAction SilentlyContinue) {
  sc.exe stop KnotRoute | Out-Null
  Start-Sleep -Seconds 1
  sc.exe delete KnotRoute | Out-Null
  Start-Sleep -Seconds 1
}
sc.exe create KnotRoute binPath= $Command start= auto DisplayName= "KnotRoute Overlay" | Out-Null
sc.exe description KnotRoute (T "Encrypted multi-hop private service overlay" "Зашифрованная многохоповая overlay-сеть приватных сервисов") | Out-Null
sc.exe failure KnotRoute reset= 86400 actions= restart/3000/restart/10000/""/0 | Out-Null
sc.exe start KnotRoute | Out-Null
Write-Host (T "KnotRoute service installed." "Служба KnotRoute установлена.")
Write-Host (T "Config: $Config" "Конфигурация: $Config")
Write-Host (T "Dashboard: http://127.0.0.1:8484" "Панель: http://127.0.0.1:8484")
