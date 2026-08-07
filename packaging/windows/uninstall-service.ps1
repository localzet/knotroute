param(
  [string]$InstallDir = "$env:ProgramFiles\KnotRoute",
  [switch]$RemoveData
)
$ErrorActionPreference = "Stop"
if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  throw "Run this script from an elevated PowerShell window."
}
if (Get-Service KnotRoute -ErrorAction SilentlyContinue) {
  sc.exe stop KnotRoute | Out-Null
  Start-Sleep -Seconds 1
  sc.exe delete KnotRoute | Out-Null
}
Remove-Item $InstallDir -Recurse -Force -ErrorAction SilentlyContinue
if ($RemoveData) { Remove-Item "$env:ProgramData\KnotRoute" -Recurse -Force -ErrorAction SilentlyContinue }
Write-Host "KnotRoute service removed."
