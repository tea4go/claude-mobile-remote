param(
  [Alias('Help')][switch]$ShowHelp,
  [Parameter(ValueFromRemainingArguments = $true)][string[]]$CliArgs
)

. (Join-Path $PSScriptRoot '_common.ps1')
if ($ShowHelp) { Write-Host '用法: .\cli-run.ps1 [Claude Mobile Remote CLI 参数]'; exit 0 }
Require-Command 'go'
Ensure-GoModule
Push-Location $script:RepoRoot
try { Invoke-External -FilePath 'go' -Arguments (@('run', './cmd/claude-mobile-remote') + @($CliArgs)) -HideArguments }
finally { Pop-Location }
