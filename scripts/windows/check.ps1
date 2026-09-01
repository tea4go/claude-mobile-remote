param(
  [Alias('Help')][switch]$ShowHelp,
  [switch]$Android
)

. (Join-Path $PSScriptRoot '_common.ps1')

function Show-Usage {
  Write-Host '用法: .\check.ps1 [-Android] [-Help]'
  Write-Host '  -Android  额外检查 Android SDK、adb、emulator 和项目 SDK 组件。'
  Write-Host '  -Help     显示帮助。'
}

if ($ShowHelp) { Show-Usage; exit 0 }

$errors = New-Object System.Collections.Generic.List[string]
function Check-Tool([string]$Name, [string]$Hint) {
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    Write-Warn "缺少 $Name。$Hint"
    [void]$errors.Add($Name)
    return
  }
  Write-Info "已找到 $Name"
}
function Check-Major([string]$Name, [int]$Minimum, [string]$Hint) {
  Check-Tool $Name $Hint
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) { return }
  $line = (& $Name '--version' 2>$null | Select-Object -First 1).ToString()
  $match = [regex]::Match($line, '(\d+)')
  if (-not $match.Success -or [int]$match.Groups[1].Value -lt $Minimum) {
    Write-Warn "$Name 版本过低：$line；要求主版本 >= $Minimum。"
    [void]$errors.Add("$Name version")
  } else { Write-Info "$Name 版本满足要求：$line" }
}

Check-Major 'go' 1 '请从 go.dev 安装 Go 1.25+。'
if (Get-Command go -ErrorAction SilentlyContinue) {
  $goLine = (& go version 2>$null | Out-String).Trim()
  $goMatch = [regex]::Match($goLine, 'go(\d+)\.(\d+)')
  if ($goMatch.Success -and ([int]$goMatch.Groups[1].Value -lt 1 -or ([int]$goMatch.Groups[1].Value -eq 1 -and [int]$goMatch.Groups[2].Value -lt 25))) {
    Write-Warn "Go 版本低于 1.25：$goLine"; [void]$errors.Add('go version')
  }
}
Check-Major 'node' 20 '请从 nodejs.org 安装 Node.js 20 LTS+。'
Check-Tool 'npm' 'npm 随 Node.js 安装。'

if ($Android) {
  $androidErrors = Test-AndroidEnvironment
  foreach ($message in $androidErrors) { Write-Warn $message; [void]$errors.Add('android') }
  if (-not (Get-GradleWrapper)) { Write-Warn '缺少 Android Gradle wrapper。'; [void]$errors.Add('gradlew') }
}

if ($errors.Count -gt 0) { Write-Host "检查未通过：$($errors.Count) 项。可使用 winget、Android Studio 或官方安装包修复。" -ForegroundColor Yellow; exit 1 }
Write-Host '环境检查通过。' -ForegroundColor Green
