# Copyright 2026 WireGuard Harmony Contributors
# SPDX-License-Identifier: Apache-2.0
#
# 构建脚本通用函数

$ErrorActionPreference = "Stop"

<#
.SYNOPSIS
  从 AppScope/app.json5 读取 versionName
.OUTPUTS
  [string] versionName 值；不存在返回空字符串
#>
function Read-AppVersion {
    $appJsonPath = "$PSScriptRoot\AppScope\app.json5"
    if (-not (Test-Path -LiteralPath $appJsonPath)) {
        Write-Host "错误：未找到 app.json5：$appJsonPath" -ForegroundColor Red
        return ''
    }

    try {
        $json = Get-Content $appJsonPath -Raw | ConvertFrom-Json
        return $json.app.versionName
    } catch {
        Write-Host "错误：读取 app.json5 失败：$_" -ForegroundColor Red
        return ''
    }
}

<#
.SYNOPSIS
  从 AppScope/app.json5 读取 versionCode
.OUTPUTS
  [int] versionCode 值；不存在返回 0
#>
function Read-AppVersionCode {
    $appJsonPath = "$PSScriptRoot\AppScope\app.json5"
    if (-not (Test-Path -LiteralPath $appJsonPath)) {
        Write-Host "错误：未找到 app.json5：$appJsonPath" -ForegroundColor Red
        return 0
    }

    try {
        $json = Get-Content $appJsonPath -Raw | ConvertFrom-Json
        return $json.app.versionCode
    } catch {
        Write-Host "错误：读取 app.json5 失败：$_" -ForegroundColor Red
        return 0
    }
}

<#
.SYNOPSIS
  写入 AppScope/app.json5 的 versionName 和 versionCode
.PARAMETER VersionName
  版本号字符串（如 "1.0.1"）
.PARAMETER VersionCode
  版本代码整数（如 1000001）
.OUTPUTS
  [bool] 写入成功返回 $true；失败返回 $false
.NOTES
  UTF-8 无 BOM 格式写回，保持 JSON 缩进格式
#>
function Set-AppVersion([string]$VersionName, [int]$VersionCode) {
    $appJsonPath = "$PSScriptRoot\AppScope\app.json5"
    if (-not (Test-Path -LiteralPath $appJsonPath)) {
        Write-Host "错误：未找到 app.json5：$appJsonPath" -ForegroundColor Red
        return $false
    }

    try {
        $json = Get-Content $appJsonPath -Raw | ConvertFrom-Json
        $json.app.versionName = $VersionName
        $json.app.versionCode = $VersionCode

        # 保持原有格式（2 空格缩进）
        $output = $json | ConvertTo-Json -Depth 10

        $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
        [System.IO.File]::WriteAllText($appJsonPath, $output, $utf8NoBom)

        Write-Host "✓ app.json5 已设置 versionName = $VersionName, versionCode = $VersionCode" -ForegroundColor Green
        return $true
    } catch {
        Write-Host "错误：写入 app.json5 失败：$_" -ForegroundColor Red
        return $false
    }
}

<#
.SYNOPSIS
  构建时处理版本号：递增版本号并同步到 app.json5
.DESCRIPTION
  - 从 app.json5 读取当前 versionName（如 "1.0.0"）
  - patch 版本递增（如 "1.0.0" → "1.0.1"，"1.0.9" → "1.0.10"）
  - 将新 versionName 和对应的 versionCode 写回 app.json5
  - versionCode = 主*1000000 + 次*1000 + 修订，严格单调递增
.OUTPUTS
  [hashtable] 包含 VersionName 和 VersionCode
#>
function Update-BuildVersion {
    $currentVersion = Read-AppVersion

    if ([string]::IsNullOrWhiteSpace($currentVersion)) {
        Write-Host "错误：无法从 app.json5 读取 versionName" -ForegroundColor Red
        exit 1
    }

    # 解析版本号（格式：x.y.z）
    $parts = $currentVersion.Split('.')
    if ($parts.Length -lt 3) {
        Write-Host "错误：版本号格式不正确，应为 x.y.z（如 1.0.0）" -ForegroundColor Red
        exit 1
    }

    # patch 版本递增（限制 0-9，超过则进位）
    $major = [int]$parts[0]
    $minor = [int]$parts[1]
    $patch = [int]$parts[2] + 1

    # 如果 patch 超过 9，进位到 minor
    if ($patch -gt 9) {
        $patch = 0
        $minor += 1
        # 如果 minor 超过 9，进位到 major
        if ($minor -gt 9) {
            $minor = 0
            $major += 1
        }
    }

    $newVersion = "$major.$minor.$patch"
    $newVersionCode = $major * 1000000 + $minor * 1000 + $patch

    # 显示版本号变更
    Write-Host "  当前版本：$currentVersion" -ForegroundColor Gray
    Write-Host "  构建版本：$newVersion (code: $newVersionCode)" -ForegroundColor Green

    # 写回 app.json5（同时更新 versionName 和 versionCode）
    if (-not (Set-AppVersion $newVersion $newVersionCode)) {
        exit 1
    }

    return @{
        VersionName = $newVersion
        VersionCode = $newVersionCode
    }
}

<#
.SYNOPSIS
  仅读取当前版本号（不修改）
.OUTPUTS
  [hashtable] 包含 VersionName 和 VersionCode
#>
function Get-CurrentVersion {
    $versionName = Read-AppVersion
    $versionCode = Read-AppVersionCode

    if ([string]::IsNullOrWhiteSpace($versionName)) {
        Write-Host "错误：无法从 app.json5 读取 versionName" -ForegroundColor Red
        exit 1
    }

    return @{
        VersionName = $versionName
        VersionCode = $versionCode
    }
}

<#
.SYNOPSIS
  检查 Rust 工具链是否真的包含目标标准库
.DESCRIPTION
  不依赖 rustup target list，直接检查 sysroot 中的 libcore。
#>
function Test-RustTargetAvailable {
    param(
        [Parameter(Mandatory = $true)][string]$Toolchain,
        [Parameter(Mandatory = $true)][string]$Target
    )

    $sysrootOutput = @(& rustc +$Toolchain --print sysroot 2>&1)
    if ($LASTEXITCODE -ne 0 -or $sysrootOutput.Count -eq 0) {
        Write-Host "  错误：无法取得 Rust 工具链 $Toolchain 的 sysroot：$($sysrootOutput -join ' ')" -ForegroundColor Red
        return $false
    }

    $sysroot = [string]$sysrootOutput[$sysrootOutput.Count - 1]
    $corePattern = Join-Path $sysroot "lib\rustlib\$Target\lib\libcore-*.rlib"
    $coreFiles = @(Get-ChildItem -Path $corePattern -File -ErrorAction SilentlyContinue)
    if ($coreFiles.Count -eq 0) {
        Write-Host "  错误：sysroot 中缺少 $Target 的 libcore：$corePattern" -ForegroundColor Yellow
        return $false
    }

    return $true
}

<#
.SYNOPSIS
  从 Rust 官方归档安装目标标准库
#>
function Install-RustTargetFromOfficialArchive {
    param(
        [Parameter(Mandatory = $true)][string]$Toolchain,
        [Parameter(Mandatory = $true)][string]$Target
    )

    $versionOutput = @(& rustc +$Toolchain --version 2>&1)
    if ($LASTEXITCODE -ne 0 -or $versionOutput.Count -eq 0) {
        throw "无法读取 Rust 工具链 $Toolchain 的版本：$($versionOutput -join ' ')"
    }
    $versionText = $versionOutput -join ' '
    if ($versionText -notmatch '\brustc\s+(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)\b') {
        throw "无法从 rustc --version 提取 semver：$versionText"
    }
    $version = $matches[1]

    $sysrootOutput = @(& rustc +$Toolchain --print sysroot 2>&1)
    if ($LASTEXITCODE -ne 0 -or $sysrootOutput.Count -eq 0) {
        throw "无法取得 Rust 工具链 $Toolchain 的 sysroot：$($sysrootOutput -join ' ')"
    }
    $sysroot = [string]$sysrootOutput[$sysrootOutput.Count - 1]
    $tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
    $archivePath = Join-Path $tempDir "rust-std-$version-$Target.tar.xz"
    $url = "https://static.rust-lang.org/dist/rust-std-$version-$Target.tar.xz"

    try {
        New-Item -ItemType Directory -Path $tempDir -Force -ErrorAction Stop | Out-Null
        Write-Host "  正在下载 Rust 官方归档：$url" -ForegroundColor Yellow
        try {
            Invoke-WebRequest -Uri $url -OutFile $archivePath -UseBasicParsing -ErrorAction Stop
        } catch {
            throw "下载 Rust 官方归档失败：$url；$($_.Exception.Message)"
        }
        if (-not (Test-Path -LiteralPath $archivePath)) {
            throw "Rust 官方归档下载后不存在：$archivePath"
        }

        $tar = Get-Command tar -ErrorAction SilentlyContinue
        if (-not $tar) {
            throw "未找到 tar，无法解压 Rust 官方归档"
        }
        & $tar.Source -xf $archivePath -C $tempDir
        if ($LASTEXITCODE -ne 0) {
            throw "解压 Rust 官方归档失败（退出码 $LASTEXITCODE）：$archivePath"
        }
        $installScript = Join-Path (Join-Path $tempDir "rust-std-$version-$Target") "install.ps1"
        if (-not (Test-Path -LiteralPath $installScript)) {
            throw "归档中未找到 install.ps1：$installScript"
        }

        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $installScript --prefix $sysroot --disable-ldconfig
        if ($LASTEXITCODE -ne 0) {
            throw "调用 Rust 官方归档 install.ps1 失败（退出码 $LASTEXITCODE）"
        }
        if (-not (Test-RustTargetAvailable -Toolchain $Toolchain -Target $Target)) {
            throw "归档安装完成后仍未找到 $Target 的 libcore"
        }
    } finally {
        Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

<#
.SYNOPSIS
  准备 Rust OHOS 交叉编译目标
.DESCRIPTION
  固定使用 MSVC Rust 工具链；先按真实 libcore 检查，rustup 失败后侧载官方归档。
#>
function Initialize-RustOhosTarget {
    param(
        [string]$Toolchain = "stable-x86_64-pc-windows-msvc",
        [string]$Target = "aarch64-unknown-linux-ohos"
    )

    if (-not (Get-Command rustup -ErrorAction SilentlyContinue)) {
        Write-Host "错误：未找到 rustup，无法准备 Rust OHOS 交叉编译环境" -ForegroundColor Red
        exit 1
    }
    if (-not (Get-Command cargo -ErrorAction SilentlyContinue)) {
        Write-Host "错误：未找到 cargo，无法编译 Rust 原生库" -ForegroundColor Red
        exit 1
    }

    $env:RUSTUP_TOOLCHAIN = $Toolchain
    if (-not (Test-RustTargetAvailable -Toolchain $Toolchain -Target $Target)) {
        Write-Host "  未找到 Rust 目标 $Target 的真实标准库，尝试 rustup 安装..." -ForegroundColor Yellow
        $previousDistServer = $env:RUSTUP_DIST_SERVER
        $installExitCode = 1
        try {
            $env:RUSTUP_DIST_SERVER = "https://static.rust-lang.org"
            & rustup target add $Target --toolchain $Toolchain
            $installExitCode = $LASTEXITCODE
        } finally {
            $env:RUSTUP_DIST_SERVER = $previousDistServer
        }

        if (-not (Test-RustTargetAvailable -Toolchain $Toolchain -Target $Target)) {
            Write-Host "  rustup 未能提供可用的 $Target（退出码 $installExitCode），切换官方归档侧载..." -ForegroundColor Yellow
            try {
                Install-RustTargetFromOfficialArchive -Toolchain $Toolchain -Target $Target
            } catch {
                Write-Host "错误：Rust 目标 $Target 的 rustup 和官方归档安装均失败：$($_.Exception.Message)" -ForegroundColor Red
                exit 1
            }
        }
    }

    if (-not (Test-RustTargetAvailable -Toolchain $Toolchain -Target $Target)) {
        Write-Host "错误：Rust 目标 $Target 最终仍缺少 libcore" -ForegroundColor Red
        exit 1
    }

    Write-Host "  ✓ Rust 工具链：$Toolchain" -ForegroundColor Green
    Write-Host "  ✓ Rust 目标：$Target（已核实 libcore）" -ForegroundColor Green
}

<#
.SYNOPSIS
  将路径解析为真实物理路径（展开目录联接 junction 与符号链接）
.DESCRIPTION
  hvigor 会校验模块路径的真实路径是否等于传入路径，从目录联接（如 C:\MyWork -> D:\MyWork）
  运行构建会因此报 "Path not found"。本函数逐级展开 reparse point，返回真实路径。
#>
function Resolve-RealPath {
    param([Parameter(Mandatory = $true)][string]$Path)

    $full = [System.IO.Path]::GetFullPath($Path)
    $root = [System.IO.Path]::GetPathRoot($full)
    $parts = $full.Substring($root.Length).Split(
        [char[]]@([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar),
        [System.StringSplitOptions]::RemoveEmptyEntries
    )

    $current = $root
    foreach ($part in $parts) {
        $candidate = [System.IO.Path]::Combine($current, $part)
        $item = Get-Item -LiteralPath $candidate -Force -ErrorAction SilentlyContinue
        if ($item -and ($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
            $target = $item.Target
            if ($target) {
                $resolved = [string]$target[0]
                if (-not [System.IO.Path]::IsPathRooted($resolved)) {
                    $resolved = [System.IO.Path]::GetFullPath(
                        [System.IO.Path]::Combine((Split-Path -LiteralPath $candidate), $resolved)
                    )
                }
                $current = $resolved
            } else {
                $current = $candidate
            }
        } else {
            $current = $candidate
        }
    }

    return $current
}

<#
.SYNOPSIS
  强制校验 HAP 产物的最低 API 版本（minAPIVersion）
.DESCRIPTION
  HAP 本质是 zip，根目录 module.json 里的 minAPIVersion 由 build-profile.json5
  的 compatibleSdkVersion 决定。校验时断言 minAPIVersion 必须等于期望值，不符即中止。
.PARAMETER HapPath
  HAP 文件路径
.PARAMETER Expected
  期望的 minAPIVersion 数字编码，默认 50000012（API 12 / HarmonyOS 5.0.0）
.NOTES
  校验失败直接 exit 1，阻断后续构建/装机流程。
#>
function Assert-HapMinApiVersion {
    param(
        [Parameter(Mandatory = $true)][string]$HapPath,
        [int]$Expected = 50000012
    )

    if (-not (Test-Path -LiteralPath $HapPath)) {
        Write-Host "  ✗ minAPIVersion 校验失败：HAP 不存在 $HapPath" -ForegroundColor Red
        exit 1
    }

    Add-Type -AssemblyName System.IO.Compression.FileSystem -ErrorAction SilentlyContinue
    $text = $null
    $zip = [System.IO.Compression.ZipFile]::OpenRead($HapPath)
    try {
        $entry = $zip.Entries | Where-Object { $_.FullName -eq 'module.json' } | Select-Object -First 1
        if (-not $entry) {
            Write-Host "  ✗ HAP 内未找到 module.json，无法校验 minAPIVersion" -ForegroundColor Red
            exit 1
        }
        $reader = New-Object System.IO.StreamReader($entry.Open())
        try { $text = $reader.ReadToEnd() } finally { $reader.Dispose() }
    } finally {
        $zip.Dispose()
    }

    if ($text -notmatch '"minAPIVersion"\s*:\s*(\d+)') {
        Write-Host "  ✗ module.json 中未找到 minAPIVersion 字段" -ForegroundColor Red
        exit 1
    }

    $actual = [int]$matches[1]
    if ($actual -ne $Expected) {
        Write-Host "  ✗ minAPIVersion 校验失败：期望 $Expected（API 12 / HarmonyOS 5.0.0），实际 $actual" -ForegroundColor Red
        Write-Host "    根因通常是 build-profile.json5 的 compatibleSdkVersion 未设为 `"5.0.0(12)`"。" -ForegroundColor Yellow
        exit 1
    }

    Write-Host "  ✓ minAPIVersion 校验通过：$actual (API 12 / HarmonyOS 5.0.0)" -ForegroundColor Green
}

<#
.SYNOPSIS
  强制校验 APP 包内所有 HAP 的最低 API 版本
.DESCRIPTION
  APP 包（.app）是 HarmonyOS 上架格式，本质是 zip，内含多个 HAP。
  校验时解压到临时目录，遍历所有 .hap 文件，逐个调 Assert-HapMinApiVersion。
  任一 HAP 不符即中止。
.PARAMETER AppPath
  APP 包文件路径
.PARAMETER Expected
  期望的 minAPIVersion 数字编码，默认 50000012（API 12 / HarmonyOS 5.0.0）
.NOTES
  临时目录用后自动清理。校验失败直接 exit 1，阻断上架流程。
#>
function Assert-AppMinApiVersion {
    param(
        [Parameter(Mandatory = $true)][string]$AppPath,
        [int]$Expected = 50000012
    )

    if (-not (Test-Path -LiteralPath $AppPath)) {
        Write-Host "  ✗ APP minAPIVersion 校验失败：APP 不存在 $AppPath" -ForegroundColor Red
        exit 1
    }

    $tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

    try {
        Add-Type -AssemblyName System.IO.Compression.FileSystem -ErrorAction SilentlyContinue
        [System.IO.Compression.ZipFile]::ExtractToDirectory($AppPath, $tempDir)

        $haps = Get-ChildItem -Path $tempDir -Recurse -Filter "*.hap"
        if ($haps.Count -eq 0) {
            Write-Host "  ✗ APP 内未找到 HAP 文件，无法校验" -ForegroundColor Red
            exit 1
        }

        Write-Host "  检测到 $($haps.Count) 个 HAP，逐个校验 minAPIVersion..." -ForegroundColor Gray
        foreach ($hap in $haps) {
            Write-Host "    校验: $($hap.Name)" -ForegroundColor Gray
            Assert-HapMinApiVersion -HapPath $hap.FullName -Expected $Expected
        }
        Write-Host "  ✓ APP 包所有 HAP 的 minAPIVersion 校验通过" -ForegroundColor Green
    } finally {
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
