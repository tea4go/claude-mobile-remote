# Copyright 2026 WireGuard Harmony Contributors
# SPDX-License-Identifier: Apache-2.0
#
# 构建签名 HAP（带版本号递增）。
# 每次执行会递增 patch 版本号（如 1.0.0 → 1.0.1），写回 app.json5，
# 然后编译并签名 HAP（需 DevEco 签名配置）。

$ErrorActionPreference = "Stop"

# 加载通用函数
. "$PSScriptRoot\_common.ps1"

# 配置
$ProjectDir  = Resolve-RealPath $PSScriptRoot
$DevEcoTools = "D:\DevDisk\DevTools\DevEco"
$Hvigorw     = "$DevEcoTools\tools\hvigor\bin\hvigorw.bat"
$HapOutput   = "$ProjectDir\entry\build\default\outputs\default\entry-default-signed.hap"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "WireGuard Harmony 构建签名 HAP" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 递增版本号
Write-Host "[版本递增]" -ForegroundColor Yellow
$versionInfo = Update-BuildVersion
Write-Host ""

# 环境变量
$env:DEVECO_SDK_HOME   = "$DevEcoTools\sdk"
$env:JAVA_HOME         = "$DevEcoTools\jbr"
$env:JAVA_TOOL_OPTIONS = "-Djdk.util.zip.disableZip64ExtraFieldValidation=true"

# Rust OHOS 交叉编译环境
Write-Host "[Rust 环境]" -ForegroundColor Yellow
Initialize-RustOhosTarget
Write-Host ""

# 注入版本号
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

# 检查产物（签名版本文件名不同）
Write-Host "[2/2] 检查产物..." -ForegroundColor Yellow
if (!(Test-Path $HapOutput)) {
    Write-Host "  ⚠ 未找到签名 HAP：$HapOutput" -ForegroundColor Yellow
    Write-Host "  提示：需要在 build-profile.json5 中配置 signingConfigs" -ForegroundColor Yellow
    Write-Host "  回退检查未签名产物..." -ForegroundColor Gray
    $HapOutput = "$ProjectDir\entry\build\default\outputs\default\entry-default-unsigned.hap"
    if (!(Test-Path $HapOutput)) {
        Write-Host "  ✗ 未签名产物也不存在：$HapOutput" -ForegroundColor Red
        exit 1
    }
}

# 强制校验最低 API 版本
Assert-HapMinApiVersion -HapPath $HapOutput

# 重命名（根据签名和构建模式命名）
$isSigned = $HapOutput -like "*-signed.hap"
$outputDir = Split-Path $HapOutput -Parent

if ($isSigned) {
    # 签名包：WireGuardDebug-x.y.z.hap（默认 debug 模式）
    $prefix = "WireGuardDebug"
} else {
    # 未签名包：WireGuardUnsigned-x.y.z.hap
    $prefix = "WireGuardUnsigned"
}

$targetHap = Join-Path $outputDir "$prefix-$($versionInfo.VersionName).hap"
try {
    Move-Item -LiteralPath $HapOutput -Destination $targetHap -Force
    Write-Host "  ✓ HAP 重命名：$prefix-$($versionInfo.VersionName).hap" -ForegroundColor Green
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
Write-Host "  版本: $($versionInfo.VersionName) (code: $($versionInfo.VersionCode))" -ForegroundColor Gray
if (!$isSigned) {
    Write-Host ""
    Write-Host "⚠ 注意：产出为未签名 HAP（未配置 signingConfigs）" -ForegroundColor Yellow
}
