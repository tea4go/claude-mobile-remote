param(
  [Alias('Host')][string]$BindHost = '127.0.0.1',
  [ValidateRange(1, 65535)][int]$Port = 5173,
  [Alias('Help')][switch]$ShowHelp,
  [Parameter(ValueFromRemainingArguments = $true)][string[]]$NpmArgs
)

. (Join-Path $PSScriptRoot '_common.ps1')
if ($ShowHelp) { Write-Host '用法: .\3_web_dev.ps1 [-Host 地址] [-Port 端口] [其他 Vite 参数]'; exit 0 }
Require-Command 'npm'
$arguments = @('run', 'dev', '--', '--host', $BindHost, '--port', $Port.ToString())
if ($NpmArgs) { $arguments += $NpmArgs }
Invoke-MobileNpm -Arguments $arguments
