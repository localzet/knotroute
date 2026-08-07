param(
  [string]$InstallDir = "$env:ProgramFiles\KnotRoute",
  [switch]$RemoveData
)
$ErrorActionPreference = "Stop"
$IsRu = [Globalization.CultureInfo]::CurrentUICulture.TwoLetterISOLanguageName -eq "ru"
function T([string]$En, [string]$Ru) { if ($IsRu) { $Ru } else { $En } }
if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  throw (T "Run this script from an elevated PowerShell window." "Запустите этот скрипт в PowerShell от имени администратора.")
}
if (Get-Service KnotRoute -ErrorAction SilentlyContinue) {
  sc.exe stop KnotRoute | Out-Null
  Start-Sleep -Seconds 1
  sc.exe delete KnotRoute | Out-Null
}
Remove-Item $InstallDir -Recurse -Force -ErrorAction SilentlyContinue
if ($RemoveData) { Remove-Item "$env:ProgramData\KnotRoute" -Recurse -Force -ErrorAction SilentlyContinue }
Write-Host (T "KnotRoute service removed." "Служба KnotRoute удалена.")
