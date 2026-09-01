param(
  [switch]$SkipTest,
  [switch]$SkipLint,
  [Alias('Help')][switch]$ShowHelp
)

. (Join-Path $PSScriptRoot '_common.ps1')
if ($ShowHelp) { Write-Host '用法: .\web-build.ps1 [-SkipTest] [-SkipLint] [-Help]'; exit 0 }
Require-Command 'npm'
if (-not $SkipTest) { Invoke-MobileNpm -Arguments @('run', 'test') }
if (-not $SkipLint) { Invoke-MobileNpm -Arguments @('run', 'lint') }
Invoke-MobileNpm -Arguments @('run', 'build')
