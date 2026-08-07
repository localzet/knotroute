param(
  [string]$InstallDir = "$env:LOCALAPPDATA\Programs\KnotRoute",
  [switch]$RemoveIdentity
)
$ErrorActionPreference = "Stop"
$DataDir = Join-Path $env:LOCALAPPDATA "KnotRoute"
# Stop the tray first so installed binaries are not locked. The daemon is asked
# to shut down gracefully through its local management API below.
Get-Process knotroute-desktop -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
try { Invoke-RestMethod -Method Post -ContentType "application/json" -Body "{}" -Uri "http://127.0.0.1:8484/api/shutdown" -TimeoutSec 2 | Out-Null; Start-Sleep -Milliseconds 500 } catch {}
reg.exe delete "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v KnotRoute /f 2>$null | Out-Null
$StateFile = Join-Path $DataDir "proxy-state.json"
if (Test-Path $StateFile) {
  $State = Get-Content $StateFile -Raw | ConvertFrom-Json
  if ($State.enabled) {
    $Key = "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings"
    if ($State.had_auto_config) { reg.exe add $Key /v AutoConfigURL /t REG_SZ /d $State.auto_config_url /f | Out-Null }
    else { reg.exe delete $Key /v AutoConfigURL /f 2>$null | Out-Null }
  }
}
try {
  Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
public static class KnotRouteWinInet {
  [DllImport("wininet.dll", SetLastError=true)] public static extern bool InternetSetOption(IntPtr hInternet, int dwOption, IntPtr lpBuffer, int dwBufferLength);
}
"@ -ErrorAction SilentlyContinue
  [KnotRouteWinInet]::InternetSetOption([IntPtr]::Zero, 95, [IntPtr]::Zero, 0) | Out-Null
  [KnotRouteWinInet]::InternetSetOption([IntPtr]::Zero, 39, [IntPtr]::Zero, 0) | Out-Null
  [KnotRouteWinInet]::InternetSetOption([IntPtr]::Zero, 37, [IntPtr]::Zero, 0) | Out-Null
} catch {}
Remove-Item (Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\KnotRoute.lnk") -Force -ErrorAction SilentlyContinue
Remove-Item (Join-Path ([Environment]::GetFolderPath("Desktop")) "KnotRoute.lnk") -Force -ErrorAction SilentlyContinue
Remove-Item $InstallDir -Recurse -Force -ErrorAction SilentlyContinue
if ($RemoveIdentity) { Remove-Item $DataDir -Recurse -Force -ErrorAction SilentlyContinue }
Write-Host "KnotRoute removed. Identity and configuration were preserved unless -RemoveIdentity was specified."
