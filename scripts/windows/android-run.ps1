param(
  [switch]$Live,
  [switch]$Dist,
  [string]$Avd,
  [ValidateRange(1, 65535)][int]$Port = 5173,
  [Alias('Help')][switch]$ShowHelp
)

. (Join-Path $PSScriptRoot '_common.ps1')
if ($ShowHelp) { Write-Host '用法: .\android-run.ps1 (-Live | -Dist) [-Avd 名称] [-Port 端口]'; exit 0 }
if (($Live -and $Dist) -or (-not $Live -and -not $Dist)) { Fail '必须且只能指定一种模式：-Live 或 -Dist。' }
Require-Command 'npm'
$wrapper = Require-AndroidEnvironment
$configuredAvd = [Environment]::GetEnvironmentVariable('AVD', 'Process')
$selectedAvd = if ($Avd) { $Avd } elseif ($configuredAvd) { $configuredAvd } else { 'pixel7' }

$hadCap = Test-Path Env:CAP_SERVER_URL
$oldCap = [Environment]::GetEnvironmentVariable('CAP_SERVER_URL', 'Process')
$hadSerial = Test-Path Env:ANDROID_SERIAL
$oldSerial = [Environment]::GetEnvironmentVariable('ANDROID_SERIAL', 'Process')
try {
  Start-AndroidAvd $selectedAvd
  if ($Live) {
    $env:CAP_SERVER_URL = "http://10.0.2.2:$Port"
    Invoke-MobileNpm -Arguments @('exec', '--', 'cap', 'sync', 'android')
  } else {
    & (Join-Path $PSScriptRoot 'web-build.ps1')
    if (-not $?) { Fail 'Web 构建失败。' }
    Remove-Item Env:CAP_SERVER_URL -ErrorAction SilentlyContinue
    Invoke-MobileNpm -Arguments @('exec', '--', 'cap', 'sync', 'android')
  }
  Invoke-MobileCommand -Command $wrapper -Arguments @('installDebug')
} finally {
  if ($hadCap) { $env:CAP_SERVER_URL = $oldCap } else { Remove-Item Env:CAP_SERVER_URL -ErrorAction SilentlyContinue }
  if ($hadSerial) { $env:ANDROID_SERIAL = $oldSerial } else { Remove-Item Env:ANDROID_SERIAL -ErrorAction SilentlyContinue }
}
