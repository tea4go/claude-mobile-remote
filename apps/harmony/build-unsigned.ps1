# Copyright 2026 WireGuard Harmony Contributors
# SPDX-License-Identifier: Apache-2.0
#
# 构建未签名 HAP（entry-default-unsigned.hap）。
# 命令行不做签名，产物无法直接装机（hdc install 会报 sign info inconsistent），
# 仅用于验证编译能通过。装机请用 DevEco 或 deploy.ps1 走签名流程。

$ErrorActionPreference = "Stop"

# 加载通用函数
. "$PSScriptRoot\_common.ps1"

# 配置
$ProjectDir  = Resolve-RealPath $PSScriptRoot
$DevEcoTools = "D:\DevDisk\DevTools\DevEco"
$Hvigorw     = "$DevEcoTools\tools\hvigor\bin\hvigorw.bat"
$HapOutput   = "$ProjectDir\entry\build\default\outputs\default\entry-default-unsigned.hap"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "WireGuard Harmony 构建未签名 HAP" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 显示当前版本号（不递增）
$versionInfo = Get-CurrentVersion
Write-Host "  当前版本：$($versionInfo.VersionName) (code: $($versionInfo.VersionCode))" -ForegroundColor Gray
Write-Host "  注意：未签名构建不递增版本号" -ForegroundColor Yellow
Write-Host ""

# 环境变量（三个缺一不可）
#   DEVECO_SDK_HOME   : 不设 → Invalid value of 'DEVECO_SDK_HOME'
#   JAVA_HOME         : hvigor 依赖的 JDK
#   JAVA_TOOL_OPTIONS : 关掉 JDK 21 的 ZIP64 严格校验，否则 SignHap 阶段会误报
$env:DEVECO_SDK_HOME   = "$DevEcoTools\sdk"
$env:JAVA_HOME         = "$DevEcoTools\jbr"
$env:JAVA_TOOL_OPTIONS = "-Djdk.util.zip.disableZip64ExtraFieldValidation=true"

# Rust OHOS 交叉编译环境
Write-Host "[Rust 环境]" -ForegroundColor Yellow
Initialize-RustOhosTarget
Write-Host ""

# 注入版本号（生成 AppVersion.ets，供 UI 显示）
& "$PSScriptRoot\gen-version.ps1"

Write-Host "[1/2] 执行 hvigorw assembleHap..." -ForegroundColor Yellow
Write-Host "  项目: $ProjectDir" -ForegroundColor Gray
Write-Host ""

Push-Location $ProjectDir
try {
    & $Hvigorw --mode module -p product=default -p module=entry@default assembleHap --no-daemon
    $exitCode = $LASTEXITCODE
} finally {
    Pop-Location
}

Write-Host ""
if ($exitCode -ne 0) {
    Write-Host "========================================" -ForegroundColor Red
    Write-Host "构建失败（退出码 $exitCode）" -ForegroundColor Red
    Write-Host "========================================" -ForegroundColor Red
    exit $exitCode
}

# 检查产物
Write-Host "[2/2] 检查产物..." -ForegroundColor Yellow
if (!(Test-Path $HapOutput)) {
    Write-Host "  ✗ 构建报成功但未找到产物：$HapOutput" -ForegroundColor Red
    exit 1
}

# 强制校验最低 API 版本
Assert-HapMinApiVersion -HapPath $HapOutput

# 重命名为标准格式：WireGuardUnsigned-<versionName>.hap
$outputDir = Split-Path $HapOutput -Parent
$targetHap = Join-Path $outputDir "WireGuardUnsigned-$($versionInfo.VersionName).hap"
try {
    Move-Item -LiteralPath $HapOutput -Destination $targetHap -Force
    Write-Host "  ✓ HAP 重命名：WireGuardUnsigned-$($versionInfo.VersionName).hap" -ForegroundColor Green
} catch {
    Write-Host "  ⚠ 重命名失败，保留原名：$_" -ForegroundColor Yellow
    $targetHap = $HapOutput
}

$hap = Get-Item $targetHap
$sizeKB = [math]::Round($hap.Length / 1KB, 1)

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "构建成功！" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  产物: $targetHap" -ForegroundColor Gray
Write-Host "  大小: $sizeKB KB" -ForegroundColor Gray
Write-Host "  时间: $($hap.LastWriteTime.ToString('yyyy-MM-dd HH:mm:ss'))" -ForegroundColor Gray
Write-Host ""
Write-Host "提示：未签名 HAP 无法直接装机，装机请用 DevEco 或 deploy.ps1。" -ForegroundColor Yellow
