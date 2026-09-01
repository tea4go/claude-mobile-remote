Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$script:ScriptDir = $PSScriptRoot
$script:RepoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..')).Path
$script:MobileRoot = Join-Path $script:RepoRoot 'apps\mobile'

function Write-Info([string]$Message) { Write-Host "[INFO] $Message" -ForegroundColor Cyan }
function Write-Warn([string]$Message) { Write-Host "[WARN] $Message" -ForegroundColor Yellow }
function Fail([string]$Message) { throw $Message }

function Require-Command([string]$Name, [string]$Hint = '') {
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    if ($Hint) { Fail "缺少命令 '$Name'。$Hint" }
    Fail "缺少命令 '$Name'。请先安装后重开 PowerShell。"
  }
}

function Invoke-External {
  param(
    [Parameter(Mandatory = $true)][string]$FilePath,
    [object[]]$Arguments = @(),
    [switch]$HideArguments
  )
  if ($HideArguments) { Write-Info ("运行: {0}（参数已隐藏）" -f $FilePath) }
  else { Write-Info ("运行: {0} {1}" -f $FilePath, ($Arguments -join ' ')) }
  & $FilePath @Arguments
  if ($LASTEXITCODE -ne 0) { Fail "命令失败（exit code $LASTEXITCODE）: $FilePath" }
}

function Ensure-GoModule {
  $module = Join-Path $script:RepoRoot 'go.mod'
  if (-not (Test-Path -LiteralPath $module)) {
    $base = Join-Path $script:RepoRoot 'go.mod.base'
    if (-not (Test-Path -LiteralPath $base)) { Fail "缺少 go.mod.base，无法恢复 go.mod。" }
    Copy-Item -LiteralPath $base -Destination $module
    Write-Info '已从 go.mod.base 恢复 go.mod（原有 go.mod 不会覆盖）。'
  }
}

function Invoke-MobileNpm {
  param([Parameter(Mandatory = $true)][object[]]$Arguments)
  if (-not (Test-Path -LiteralPath $script:MobileRoot)) { Fail "找不到 Mobile 项目: $script:MobileRoot" }
  Push-Location $script:MobileRoot
  try { Invoke-External -FilePath 'npm' -Arguments $Arguments }
  finally { Pop-Location }
}

function Invoke-MobileCommand {
  param(
    [Parameter(Mandatory = $true)][string]$Command,
    [object[]]$Arguments = @()
  )
  if (-not (Test-Path -LiteralPath $script:MobileRoot)) { Fail "找不到 Mobile 项目: $script:MobileRoot" }
  Push-Location $script:MobileRoot
  try { Invoke-External -FilePath $Command -Arguments $Arguments }
  finally { Pop-Location }
}

function Get-AndroidHome {
  foreach ($name in @('ANDROID_HOME', 'ANDROID_SDK_ROOT')) {
    $value = [Environment]::GetEnvironmentVariable($name)
    if (-not [string]::IsNullOrWhiteSpace($value) -and (Test-Path -LiteralPath $value)) {
      return (Resolve-Path -LiteralPath $value).Path
    }
  }
  return $null
}

function Get-AndroidTool([string]$Name) {
  $home = Get-AndroidHome
  if ($home) {
    $relative = switch ($Name) {
      'adb' { 'platform-tools\adb.exe' }
      'emulator' { 'emulator\emulator.exe' }
      'sdkmanager' { 'cmdline-tools\latest\bin\sdkmanager.bat' }
      default { $null }
    }
    if ($relative) {
      $candidate = Join-Path $home $relative
      if (Test-Path -LiteralPath $candidate) { return $candidate }
    }
  }
  $command = Get-Command $Name -ErrorAction SilentlyContinue
  if ($command) { return $command.Source }
  return $null
}

function Get-GradleWrapper {
  $bat = Join-Path $script:MobileRoot 'android\gradlew.bat'
  if (Test-Path -LiteralPath $bat) { return $bat }
  $shell = Join-Path $script:MobileRoot 'android\gradlew'
  if (Test-Path -LiteralPath $shell) { return $shell }
  return $null
}

function Test-AndroidEnvironment {
  $errors = New-Object System.Collections.Generic.List[string]
  $home = Get-AndroidHome
  if (-not $home) { [void]$errors.Add('未找到 ANDROID_HOME 或 ANDROID_SDK_ROOT 指向的 Android SDK。') }
  foreach ($tool in @('adb', 'emulator')) {
    if (-not (Get-AndroidTool $tool)) { [void]$errors.Add("未找到 $tool，请通过 Android Studio 或 command-line tools 安装。") }
  }
  if (-not (Get-AndroidTool 'sdkmanager')) { [void]$errors.Add('未找到 sdkmanager，请安装 Android SDK Command-line Tools。') }
  $variables = Join-Path $script:MobileRoot 'android\variables.gradle'
  if ($home -and (Test-Path -LiteralPath $variables)) {
    $content = Get-Content -LiteralPath $variables -Raw
    $match = [regex]::Match($content, 'compileSdkVersion\s*=\s*(\d+)')
    if ($match.Success) {
      $platform = Join-Path $home ("platforms\android-{0}" -f $match.Groups[1].Value)
      if (-not (Test-Path -LiteralPath $platform)) { [void]$errors.Add("缺少 SDK platform android-$($match.Groups[1].Value)。请用 sdkmanager 安装。") }
    }
  }
  if ($home) {
    $buildTools = Join-Path $home 'build-tools'
    $requiredBuildTools = $null
    if (Test-Path -LiteralPath $variables) {
      $match = [regex]::Match($content, "buildToolsVersion\s*=\s*['`\"]([^'`\"]+)['`\"]")
      if ($match.Success) { $requiredBuildTools = $match.Groups[1].Value }
    }
    $buildToolsPath = if ($requiredBuildTools) { Join-Path $buildTools $requiredBuildTools } else { $null }
    if ($requiredBuildTools -and -not (Test-Path -LiteralPath $buildToolsPath)) {
      [void]$errors.Add("缺少 Android build-tools $requiredBuildTools，请用 sdkmanager 安装。")
    } elseif (-not $requiredBuildTools -and (-not (Test-Path -LiteralPath $buildTools) -or @(Get-ChildItem -LiteralPath $buildTools -Directory -ErrorAction SilentlyContinue).Count -eq 0)) {
      [void]$errors.Add('缺少 Android build-tools，请通过 Android Studio 或 sdkmanager 安装。')
    }
  }
  return $errors
}

function Require-AndroidEnvironment {
  $errors = Test-AndroidEnvironment
  if ($errors.Count -gt 0) { Fail (($errors -join [Environment]::NewLine) + [Environment]::NewLine + '安装建议：Android Studio SDK Manager 或官方 command-line tools。') }
  $wrapper = Get-GradleWrapper
  if (-not $wrapper) { Fail '找不到 apps/mobile/android/gradlew.bat（或 gradlew）。请确认 Capacitor Android 工程已生成。' }
  return $wrapper
}

function Get-AvdNames {
  $emulator = Get-AndroidTool 'emulator'
  if (-not $emulator) { return @() }
  return @(& $emulator '-list-avds' 2>$null | Where-Object { $_ -and $_.Trim() })
}

function Start-AndroidAvd([string]$AvdName) {
  $emulator = Get-AndroidTool 'emulator'
  $adb = Get-AndroidTool 'adb'
  if (-not $emulator -or -not $adb) { Fail '缺少 emulator 或 adb，无法启动 Android 模拟器。' }
  $available = Get-AvdNames
  if ($available -notcontains $AvdName) { Fail "找不到 AVD '$AvdName'。可用 AVD: $($available -join ', ')。请使用 -Avd 或设置 AVD。" }
  $serial = Get-AvdSerial $adb $AvdName
  if ($serial) { $env:ANDROID_SERIAL = $serial; return }
  Write-Info "启动 Android AVD: $AvdName"
  Start-Process -FilePath $emulator -ArgumentList @('-avd', $AvdName) -WindowStyle Hidden
  $ready = $false
  for ($i = 0; $i -lt 60; $i++) {
    Start-Sleep -Seconds 2
    $serial = Get-AvdSerial $adb $AvdName
    if ($serial) {
      $boot = (& $adb -s $serial shell getprop sys.boot_completed 2>$null | Out-String).Trim()
      if ($boot -match '1') { $ready = $true; $env:ANDROID_SERIAL = $serial; break }
    }
  }
  if (-not $ready) { Fail "Android AVD '$AvdName' 在 120 秒内未启动完成。" }
}

function Get-AvdSerial([string]$Adb, [string]$AvdName) {
  $rows = & $Adb devices 2>$null
  foreach ($row in $rows) {
    $parts = ($row -split '\s+')
    if ($parts.Count -ge 2 -and $parts[1] -eq 'device') {
      $running = (& $Adb -s $parts[0] shell getprop ro.boot.qemu.avd_name 2>$null | Out-String).Trim()
      if ($running -eq $AvdName) { return $parts[0] }
    }
  }
  return $null
}
