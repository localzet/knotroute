$ErrorActionPreference = "Stop"
if (Get-Service KnotRoute -ErrorAction SilentlyContinue) {
  sc.exe stop KnotRoute | Out-Null
  sc.exe delete KnotRoute | Out-Null
  Write-Host "KnotRoute service removed. Data in $env:ProgramData\KnotRoute was preserved."
} else {
  Write-Host "KnotRoute service is not installed."
}
