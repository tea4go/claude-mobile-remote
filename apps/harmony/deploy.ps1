# Copyright 2026 Claude Mobile Remote Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# 装机部署脚本：先构建签名 HAP，再通过 hdc 安装到设备。

$ErrorActionPreference = "Stop"

# 加载通用函数
. "$PSScriptRoot\_common.ps1"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Claude Mobile Remote Harmony 装机部署" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 检查 hdc 可用性
$hdc = "hdc"
try {
    & $hdc -v | Out-Null
} catch {
    Write-Host "错误：hdc 命令不可用，请确保 HarmonyOS SDK 的 toolchains 目录在 PATH 中" -ForegroundColor Red
    exit 1
}

# 构建签名 HAP
Write-Host "[1/3] 构建签名 HAP..." -ForegroundColor Yellow
& "$PSScriptRoot\build-signed.ps1"
if ($LASTEXITCODE -ne 0) {
    Write-Host "  ✗ 构建失败" -ForegroundColor Red
    exit $LASTEXITCODE
}
Write-Host ""

# 查找最新产物
$versionInfo = Get-CurrentVersion
$outputDir = "$PSScriptRoot\entry\build\default\outputs\default"
$signedHap = Join-Path $outputDir "ClaudeMobileRemoteDebug-$($versionInfo.VersionName).hap"
$unsignedHap = Join-Path $outputDir "ClaudeMobileRemoteUnsigned-$($versionInfo.VersionName).hap"

$hapToInstall = $null
if (Test-Path $signedHap) {
    $hapToInstall = $signedHap
    Write-Host "[2/3] 准备安装签名 HAP..." -ForegroundColor Yellow
} elseif (Test-Path $unsignedHap) {
    $hapToInstall = $unsignedHap
    Write-Host "[2/3] 准备安装未签名 HAP（可能失败）..." -ForegroundColor Yellow
    Write-Host "  ⚠ 未签名 HAP 需要设备处于开发者模式" -ForegroundColor Yellow
} else {
    Write-Host "  ✗ 未找到可安装的 HAP 文件" -ForegroundColor Red
    exit 1
}

Write-Host "  HAP: $hapToInstall" -ForegroundColor Gray
Write-Host ""

# 枚举已连接设备（过滤空行和 [Empty]）
Write-Host "[3/3] 安装并启动（全部设备）..." -ForegroundColor Yellow
$targets = & $hdc list targets
$devices = @($targets | Where-Object { $_.Trim() -match '\S' -and $_ -notmatch '\[Empty\]' } | ForEach-Object { $_.Trim() })
if ($devices.Count -eq 0) {
    Write-Host "  ✗ 未检测到任何已连接的设备，请确保设备已连接并开启 USB 调试" -ForegroundColor Red
    exit 1
}
Write-Host "  当前已连接设备（$($devices.Count) 台）：" -ForegroundColor Gray
foreach ($dev in $devices) {
    Write-Host "    - $dev" -ForegroundColor Green
}
Write-Host ""

# 逐台安装并启动
$bundleName = "com.tea4go.claudemobileremote1028"
$okCount = 0
foreach ($dev in $devices) {
    Write-Host "  ▶ $dev" -ForegroundColor Cyan

    # 先尝试卸载旧版本（避免签名冲突），忽略失败
    & $hdc -t $dev uninstall $bundleName 2>$null | Out-Null
    Start-Sleep -Milliseconds 500

    # 安装
    # 注意：hdc install 即使安装失败退出码仍为 0，真实结果在输出文本里，
    # 必须按输出关键字判定，不能只看 $LASTEXITCODE。
    $installOut = & $hdc -t $dev install $hapToInstall 2>&1 | Out-String
    if ($installOut -notmatch 'install bundle successfully') {
        Write-Host "    ⚠ 标准安装失败，尝试替换安装..." -ForegroundColor Yellow
        $installOut = & $hdc -t $dev install -r $hapToInstall 2>&1 | Out-String
    }
    if ($installOut -notmatch 'install bundle successfully') {
        $reason = ($installOut -split "`n" | Where-Object { $_ -match 'error|failed' } | Select-Object -First 1).Trim()
        if (!$reason) { $reason = ($installOut.Trim() -split "`n")[0] }
        Write-Host "    ✗ 安装失败，跳过此设备：$reason" -ForegroundColor Red
        continue
    }
    Write-Host "    ✓ 安装完成" -ForegroundColor Green

    # 启动
    # aa start 同样存在退出码不可靠问题，按输出判定：成功输出含 "successfully"
    $startOut = & $hdc -t $dev shell aa start -a EntryAbility -b $bundleName 2>&1 | Out-String
    if ($startOut -match 'successfully') {
        Write-Host "    ✓ 应用已启动" -ForegroundColor Green
    } else {
        $sreason = ($startOut.Trim() -split "`n")[0]
        Write-Host "    ⚠ 启动失败，请在该设备手动拉起应用：$sreason" -ForegroundColor Yellow
    }
    $okCount++
}
Write-Host ""
Write-Host "  部署结果：$okCount/$($devices.Count) 台成功" -ForegroundColor Gray

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "部署完成！" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  应用：Claude Mobile Remote for HarmonyOS" -ForegroundColor Gray
Write-Host "  版本：$($versionInfo.VersionName)" -ForegroundColor Gray
Write-Host ""
Write-Host "提示：首次启动需授予 VPN 权限" -ForegroundColor Yellow
