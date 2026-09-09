# Copyright 2026 WireGuard Harmony Contributors
# SPDX-License-Identifier: Apache-2.0
#
# 构建上架华为应用市场用的 APP 包（harmony-default-signed.app）。
# 用 `hvigorw assembleApp --mode project` 打包整个工程。
#
# 签名说明：
#   - 上架必须用「发布证书」签名（AGC 申请的 release .cer + 发布 Profile .p7b）。
#   - 若 build-profile.json5 的 signingConfigs 已配置发布签名，产出 *-signed.app，可上架。
#   - 若 signingConfigs 为空，仍会产出 *-unsigned.app，但仅供验证编译，无法上架。
#
# 版本号：
#   不自增版本号，直接读取 app.json5 当前版本。
#   版本号统一由 build-signed.ps1 在签名 HAP 时递增，避免多个脚本重复递增导致版本跳号。

$ErrorActionPreference = "Stop"

# 加载通用函数
. "$PSScriptRoot\_common.ps1"

# 配置
$ProjectDir   = $PSScriptRoot
$DevEcoTools  = "D:\DevDisk\DevTools\DevEco"
$Hvigorw      = "$DevEcoTools\tools\hvigor\bin\hvigorw.bat"
$BuildProfile = "$ProjectDir\build-profile.json5"
$AppOutputDir = "$ProjectDir\build\outputs\default"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "WireGuard 构建上架 APP 包" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 1. 检查工程级 build-profile.json5
Write-Host "[1/5] 检查构建配置..." -ForegroundColor Yellow
if (!(Test-Path $BuildProfile)) {
    Write-Host "  X 未找到 build-profile.json5：$BuildProfile" -ForegroundColor Red
    Write-Host "    该文件是工程级构建配置（gitignore），需在 DevEco 打开工程生成，" -ForegroundColor Gray
    Write-Host "    或参照旧配置手工补齐 products/modules 段。" -ForegroundColor Gray
    exit 1
}
$profileText = Get-Content $BuildProfile -Raw
$isSigned = -not ($profileText -match '"signingConfigs"\s*:\s*\[\s*\]')
if ($isSigned) {
    Write-Host "  √ signingConfigs 已配置（产出签名包）" -ForegroundColor Green
} else {
    Write-Host "  ! signingConfigs 为空：将产出未签名包，无法上架" -ForegroundColor Yellow
    Write-Host "    上架前需在 build-profile.json5 配置发布证书签名。" -ForegroundColor Gray
}
Write-Host ""

# 2. 版本号处理（只读，不递增）
Write-Host "[2/5] 读取版本号..." -ForegroundColor Yellow
$versionInfo = Get-CurrentVersion
Write-Host "  当前版本：$($versionInfo.VersionName) (code: $($versionInfo.VersionCode))" -ForegroundColor Gray
Write-Host "  注意：APP 包构建不递增版本号，版本号统一由 build-signed.ps1 管理" -ForegroundColor Yellow
Write-Host ""

# 环境变量（三个缺一不可）
#   DEVECO_SDK_HOME   : 不设 -> Invalid value of 'DEVECO_SDK_HOME'
#   JAVA_HOME         : hvigor 依赖的 JDK
#   JAVA_TOOL_OPTIONS : 关掉 JDK 21 的 ZIP64 严格校验，否则 SignHap 阶段会误报
$env:DEVECO_SDK_HOME   = "$DevEcoTools\sdk"
$env:JAVA_HOME         = "$DevEcoTools\jbr"
$env:JAVA_TOOL_OPTIONS = "-Djdk.util.zip.disableZip64ExtraFieldValidation=true"

# 注入版本号（生成 AppVersion.ets，供面板标题栏显示）
& "$PSScriptRoot\gen-version.ps1"

# 3. 执行构建
Write-Host "[3/5] 执行 hvigorw assembleApp..." -ForegroundColor Yellow
Write-Host "  项目: $ProjectDir" -ForegroundColor Gray
Write-Host ""

Push-Location $ProjectDir
try {
    & $Hvigorw --mode project -p product=default assembleApp --no-daemon
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

# 4. 定位产物（按签名状态精确选择，避免误取中间产物）
Write-Host "[4/5] 检查产物..." -ForegroundColor Yellow
if ($isSigned) {
    # 签名模式：只认 *-signed.app，明确排除 unsigned 中间产物
    $appItem = Get-ChildItem -Path $AppOutputDir -Filter "*signed.app" -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -notlike "*unsigned*" } |
        Sort-Object LastWriteTime -Descending | Select-Object -First 1
    if (-not $appItem) {
        Write-Host "  X 已配签名但未找到 *-signed.app：$AppOutputDir" -ForegroundColor Red
        Write-Host "    请检查 build-profile.json5 的 products 是否引用了 signingConfig。" -ForegroundColor Gray
        exit 1
    }
} else {
    $appItem = Get-ChildItem -Path $AppOutputDir -Filter "*unsigned.app" -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending | Select-Object -First 1
    if (-not $appItem) {
        Write-Host "  X 构建报成功但未找到 .app 产物：$AppOutputDir" -ForegroundColor Red
        exit 1
    }
}

# 强制校验 APP 包内所有 HAP 的最低 API 版本
Assert-AppMinApiVersion -AppPath $appItem.FullName

# 5. 重命名为标准格式：WireGuardRelease-<versionName>.app
Write-Host "[5/5] 重命名产物..." -ForegroundColor Yellow
$outName = "WireGuardRelease-$($versionInfo.VersionName).app"
$finalOutput = Join-Path $AppOutputDir $outName
try {
    Move-Item -LiteralPath $appItem.FullName -Destination $finalOutput -Force
    Write-Host "  √ 产物已重命名：$outName" -ForegroundColor Green
} catch {
    Write-Host "  ! 重命名失败，保留原名：$_" -ForegroundColor Yellow
    $finalOutput = $appItem.FullName
    $outName = $appItem.Name
}

$app = Get-Item $finalOutput
$sizeKB = [math]::Round($app.Length / 1KB, 1)

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "构建成功！" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  产物: $finalOutput" -ForegroundColor Gray
Write-Host "  文件名: $outName" -ForegroundColor Gray
Write-Host "  大小: $sizeKB KB" -ForegroundColor Gray
Write-Host ""
if ($isSigned) {
    Write-Host "已签名 APP 包，可上传 AppGallery Connect 提交审核。" -ForegroundColor Green
} else {
    Write-Host "警告：此为未签名包，不能上架！" -ForegroundColor Red
    Write-Host "上架需先在 build-profile.json5 配置发布证书签名后重新构建。" -ForegroundColor Yellow
}
