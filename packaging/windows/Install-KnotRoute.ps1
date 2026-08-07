param(
  [string]$InstallDir = "$env:LOCALAPPDATA\Programs\KnotRoute",
  [switch]$NoDesktopShortcut
)
$ErrorActionPreference = "Stop"
$Source = Split-Path -Parent $MyInvocation.MyCommand.Path
$Daemon = Join-Path $Source "knotroute.exe"
$Desktop = Join-Path $Source "knotroute-desktop.exe"
if (-not (Test-Path $Daemon) -or -not (Test-Path $Desktop)) { throw "Place this script next to knotroute.exe and knotroute-desktop.exe." }
New-Item -ItemType Directory -Force $InstallDir | Out-Null
Copy-Item $Daemon,$Desktop (Join-Path $InstallDir ".") -Force
$Shell = New-Object -ComObject WScript.Shell
$StartMenu = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\KnotRoute.lnk"
$Shortcut = $Shell.CreateShortcut($StartMenu)
$Shortcut.TargetPath = Join-Path $InstallDir "knotroute-desktop.exe"
$Shortcut.WorkingDirectory = $InstallDir
$Shortcut.Description = "KnotRoute private overlay"
$Shortcut.Save()
if (-not $NoDesktopShortcut) {
  $DesktopLink = Join-Path ([Environment]::GetFolderPath("Desktop")) "KnotRoute.lnk"
  $Shortcut = $Shell.CreateShortcut($DesktopLink)
  $Shortcut.TargetPath = Join-Path $InstallDir "knotroute-desktop.exe"
  $Shortcut.WorkingDirectory = $InstallDir
  $Shortcut.Description = "KnotRoute private overlay"
  $Shortcut.Save()
}
Start-Process (Join-Path $InstallDir "knotroute-desktop.exe")
Write-Host "KnotRoute installed for the current user: $InstallDir"
