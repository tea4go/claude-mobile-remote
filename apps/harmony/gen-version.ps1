# Copyright 2026 Claude Mobile Remote Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# 生成版本常量文件 AppVersion.ets，供 UI 显示。
# 版本格式：v<versionName>-B<yyMMdd>-<HHmm>，例如 v1.0.1-B260815-1200
#   versionName   从 app.json5 读取（构建时已递增）
#   B<yyMMdd>     构建日期
#   <HHmm>       构建时刻
# 由 build-signed.ps1 / build-unsigned.ps1 在编译前调用，确保每次构建刷新时间戳。

$ErrorActionPreference = "Stop"

# 加载通用函数
. "$PSScriptRoot\_common.ps1"

$OutFile = "$PSScriptRoot\entry\src\main\ets\common\AppVersion.ets"

# 确保目标目录存在（首次构建时 common 目录可能尚未创建）
$outDir = Split-Path -LiteralPath $OutFile -Parent
if (-not (Test-Path -LiteralPath $outDir)) {
    New-Item -ItemType Directory -Path $outDir -Force | Out-Null
}

# 从 app.json5 读取当前版本号
$versionInfo = Get-CurrentVersion
$versionName = $versionInfo.VersionName

$now = Get-Date
$stamp = $now.ToString("yyMMdd")
$time = $now.ToString("HHmm")
$fullVersion = "v$versionName-B$stamp-$time"

$content = @"
// Copyright 2026 Claude Mobile Remote Contributors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// 本文件由 gen-version.ps1 在每次构建时生成，请勿手动修改。
// 版本格式：v<versionName>-B<yyMMdd>-<HHmm>
export const APP_VERSION: string = '$fullVersion';
"@

$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($OutFile, $content, $utf8NoBom)
Write-Host "  ✓ 版本号已注入: $fullVersion" -ForegroundColor Green
