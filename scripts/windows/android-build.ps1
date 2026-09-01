param(
  [switch]$Release,
  [Alias('Help')][switch]$ShowHelp
)

. (Join-Path $PSScriptRoot '_common.ps1')
if ($ShowHelp) { Write-Host '用法: .\android-build.ps1 [-Release] [-Help]'; exit 0 }
Require-Command 'npm'
$wrapper = Require-AndroidEnvironment
& (Join-Path $PSScriptRoot 'web-build.ps1')
if (-not $?) { Fail 'Web 构建失败。' }
$hadCap = Test-Path Env:CAP_SERVER_URL
$oldCap = [Environment]::GetEnvironmentVariable('CAP_SERVER_URL', 'Process')
try {
  Remove-Item Env:CAP_SERVER_URL -ErrorAction SilentlyContinue
  Invoke-MobileNpm -Arguments @('exec', '--', 'cap', 'sync', 'android')
  $task = if ($Release) { 'assembleRelease' } else { 'assembleDebug' }
  Invoke-MobileCommand -Command $wrapper -Arguments @($task)
} finally {
  if ($hadCap) { $env:CAP_SERVER_URL = $oldCap } else { Remove-Item Env:CAP_SERVER_URL -ErrorAction SilentlyContinue }
}
if ($Release) { Write-Warn 'release 产物未签名；签名需由外部发布流程处理，本脚本不读取 keystore。' }
