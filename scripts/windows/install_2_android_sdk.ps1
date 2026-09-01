param(
  [string]$SdkRoot,
  [Alias('Help')][switch]$ShowHelp
)
. (Join-Path $PSScriptRoot '_common.ps1')
if ($ShowHelp) { Write-Host '用法: .\install_2_android_sdk.ps1 [-SdkRoot 路径]'; exit 0 }

if ([string]::IsNullOrWhiteSpace($SdkRoot)) { $SdkRoot = [Environment]::GetEnvironmentVariable('ANDROID_HOME', 'Process') }
if ([string]::IsNullOrWhiteSpace($SdkRoot)) { $SdkRoot = Join-Path $env:LOCALAPPDATA 'Android\Sdk' }
$SdkRoot = $SdkRoot.Trim('"')
New-Item -ItemType Directory -Force -Path $SdkRoot | Out-Null
$cmdline = Join-Path $SdkRoot 'cmdline-tools\latest\bin\sdkmanager.bat'

if (-not (Test-Path -LiteralPath $cmdline)) {
  $zip = Join-Path $env:TEMP ("android-cmdline-tools-{0}.zip" -f ([guid]::NewGuid().ToString('N')))
  $tmp = Join-Path $env:TEMP ("android-cmdline-tools-{0}" -f ([guid]::NewGuid().ToString('N')))
  $url = 'https://dl.google.com/android/repository/commandlinetools-win-11076708_latest.zip'
  try {
    Write-Info '下载 Android command-line tools。'
    Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing
    Expand-Archive -LiteralPath $zip -DestinationPath $tmp -Force
    $source = Join-Path $tmp 'cmdline-tools'
    $latest = Join-Path $SdkRoot 'cmdline-tools\latest'
    New-Item -ItemType Directory -Force -Path (Split-Path $latest) | Out-Null
    if (Test-Path -LiteralPath $latest) { Remove-Item -LiteralPath $latest -Recurse -Force }
    Move-Item -LiteralPath $source -Destination $latest
  } finally {
    Remove-Item -LiteralPath $zip -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
  }
}
if (-not (Test-Path -LiteralPath $cmdline)) { Fail "安装后仍找不到 sdkmanager.bat: $cmdline" }
Require-Command 'java'
$java = (& java -version 2>&1 | Out-String)
if ($java -notmatch 'version "(1\.)?(1[7-9]|[2-9][0-9])') { Fail '需要 Java 17 或更高版本。' }
$packages = @('platform-tools', 'platforms;android-36', 'build-tools;36.0.0')
$yes = Join-Path $env:TEMP ("android-sdk-yes-{0}.txt" -f ([guid]::NewGuid().ToString('N')))
try {
  1..2500 | ForEach-Object { 'y' } | Set-Content -LiteralPath $yes -Encoding ASCII
  $args = @("--sdk_root=$SdkRoot", '--licenses')
  cmd.exe /c "type `"$yes`" | `"$cmdline`" $($args -join ' ')" | Out-Host
  if ($LASTEXITCODE -ne 0) { Fail "sdkmanager license 接受失败（exit code $LASTEXITCODE）。" }
  & $cmdline "--sdk_root=$SdkRoot" @packages
  if ($LASTEXITCODE -ne 0) { Fail "sdkmanager 安装组件失败（exit code $LASTEXITCODE）。" }
} finally { Remove-Item -LiteralPath $yes -Force -ErrorAction SilentlyContinue }
[Environment]::SetEnvironmentVariable('ANDROID_HOME', $SdkRoot, 'User')
[Environment]::SetEnvironmentVariable('ANDROID_SDK_ROOT', $SdkRoot, 'User')
$env:ANDROID_HOME = $SdkRoot; $env:ANDROID_SDK_ROOT = $SdkRoot
$platformTools = Join-Path $SdkRoot 'platform-tools'
if (($env:Path -split ';') -notcontains $platformTools) { $env:Path = "$platformTools;$env:Path" }
Write-Host "Android SDK 安装完成: $SdkRoot" -ForegroundColor Green
Write-Warn '用户级环境变量已写入；新开的 PowerShell 会自动生效。'
