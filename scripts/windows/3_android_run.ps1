param(
  [switch]$Live,
  [switch]$Dist,
  [string]$Avd,
  [ValidateRange(1, 65535)][int]$Port = 5173,
  [Alias('Help')][switch]$ShowHelp
)

. (Join-Path $PSScriptRoot '_common.ps1')
if ($ShowHelp) { Write-Host '用法: .\3_android_run.ps1 (-Live | -Dist) [-Avd 名称] [-Port 端口]'; exit 0 }
if (($Live -and $Dist) -or (-not $Live -and -not $Dist)) { Fail '必须且只能指定一种模式：-Live 或 -Dist。' }
Require-Command 'npm'
$wrapper = Require-AndroidEnvironment
$configuredAvd = [Environment]::GetEnvironmentVariable('AVD', 'Process')
$selectedAvd = if ($Avd) { $Avd } elseif ($configuredAvd) { $configuredAvd } else {
  $avds = @(Get-AvdNames)
  if (-not $avds) { Fail '未找到任何 AVD。请用 Android Studio 的 Device Manager 创建 AVD 后再运行。' }
  if ($avds.Count -gt 1) { Fail "检测到多个 AVD，请用 -Avd 明确指定：$($avds -join ', ')。" }
  $avds[0]
}

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
    & (Join-Path $PSScriptRoot '4_web_build.ps1')
    if (-not $?) { Fail 'Web 构建失败。' }
    Remove-Item Env:CAP_SERVER_URL -ErrorAction SilentlyContinue
    Invoke-MobileNpm -Arguments @('exec', '--', 'cap', 'sync', 'android')
  }
  Invoke-MobileCommand -Command $wrapper -Arguments @('installDebug')
  $appId = Get-AndroidAppId
  if (-not $appId) { Fail '无法从 app/build.gradle 读取 applicationId，无法启动 App。' }
  Start-AndroidApp -AppId $appId
  if ($Live -and -not (Test-TcpPort $Port)) {
    Write-Warn "未检测到端口 $Port 上的 Web dev server，App 页面将无法加载。请另开终端运行 scripts/windows/3_web_dev.ps1 -Port $Port。"
  }
} finally {
  if ($hadCap) { $env:CAP_SERVER_URL = $oldCap } else { Remove-Item Env:CAP_SERVER_URL -ErrorAction SilentlyContinue }
  if ($hadSerial) { $env:ANDROID_SERIAL = $oldSerial } else { Remove-Item Env:ANDROID_SERIAL -ErrorAction SilentlyContinue }
}
