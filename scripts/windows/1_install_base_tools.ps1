param(
  [Alias('Help')][switch]$ShowHelp
)
. (Join-Path $PSScriptRoot '_common.ps1')
if ($ShowHelp) { Write-Host '用法: .\1_install_base_tools.ps1'; exit 0 }

foreach ($tool in @('go', 'node', 'npm')) {
  if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
    if (Get-Command winget -ErrorAction SilentlyContinue) {
      $id = if ($tool -eq 'go') { 'GoLang.Go' } elseif ($tool -eq 'node') { 'OpenJS.NodeJS.LTS' } else { $null }
      if ($id) { Write-Info "缺少 $tool，可执行: winget install --id $id" }
    }
    Fail "缺少 $tool。请安装后重新打开 PowerShell。"
  }
}
Ensure-GoModule
Push-Location $script:RepoRoot
try { Invoke-GoWithFallback -Arguments @('mod', 'tidy') } finally { Pop-Location }
Invoke-MobileNpm -Arguments @('ci')
Write-Host '基础工具和项目依赖安装完成。' -ForegroundColor Green
