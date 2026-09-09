# Copyright 2026 Claude Mobile Remote Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# 签名配置脚本：从 .env 文件读取签名信息，自动配置 build-profile.json5。

$ErrorActionPreference = "Stop"

$ProjectDir = $PSScriptRoot
$EnvFile = "$ProjectDir\.env"
$BuildProfile = "$ProjectDir\build-profile.json5"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "HarmonyOS 签名配置工具" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 检查 .env 文件
if (!(Test-Path $EnvFile)) {
    Write-Host "错误：未找到 .env 文件" -ForegroundColor Red
    Write-Host ""
    Write-Host "请执行以下步骤：" -ForegroundColor Yellow
    Write-Host "  1. 复制 .env.example 为 .env" -ForegroundColor Yellow
    Write-Host "  2. 编辑 .env 填写签名信息" -ForegroundColor Yellow
    Write-Host "  3. 重新运行此脚本" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "命令：" -ForegroundColor Gray
    Write-Host "  Copy-Item .env.example .env" -ForegroundColor Gray
    Write-Host "  notepad .env" -ForegroundColor Gray
    exit 1
}

# 解析 .env 文件
Write-Host "[1/3] 读取 .env 配置..." -ForegroundColor Yellow
$envVars = @{}
Get-Content $EnvFile | ForEach-Object {
    $line = $_.Trim()
    # 跳过注释和空行
    if ($line -and !$line.StartsWith("#")) {
        if ($line -match "^([^=]+)=(.*)$") {
            $key = $matches[1].Trim()
            $value = $matches[2].Trim()
            $envVars[$key] = $value
        }
    }
}

# 验证必需字段
$required = @(
    "SIGNING_STORE_FILE",
    "SIGNING_CERT_PATH",
    "SIGNING_KEY_ALIAS",
    "SIGNING_STORE_PASSWORD",
    "SIGNING_KEY_PASSWORD"
)

$missing = @()
foreach ($key in $required) {
    if (!$envVars.ContainsKey($key) -or [string]::IsNullOrWhiteSpace($envVars[$key])) {
        $missing += $key
    }
}

if ($missing.Count -gt 0) {
    Write-Host "  ✗ 配置不完整，缺少以下字段：" -ForegroundColor Red
    $missing | ForEach-Object { Write-Host "    - $_" -ForegroundColor Red }
    Write-Host ""
    Write-Host "请编辑 .env 文件补充缺失信息" -ForegroundColor Yellow
    exit 1
}

# 检查文件存在性
Write-Host "  ✓ 配置完整" -ForegroundColor Green
Write-Host ""
Write-Host "[2/3] 验证文件..." -ForegroundColor Yellow

$storeFile = $envVars["SIGNING_STORE_FILE"]
$certPath = $envVars["SIGNING_CERT_PATH"]

if (!(Test-Path $storeFile)) {
    Write-Host "  ✗ 未找到 .p12 文件：$storeFile" -ForegroundColor Red
    exit 1
}
Write-Host "  ✓ 找到 .p12 文件：$storeFile" -ForegroundColor Green

if (!(Test-Path $certPath)) {
    Write-Host "  ✗ 未找到 .cer 文件：$certPath" -ForegroundColor Red
    exit 1
}
Write-Host "  ✓ 找到 .cer 文件：$certPath" -ForegroundColor Green

# 转换为相对路径或保持绝对路径
$storeFileEscaped = $storeFile -replace '\\', '/'
$certPathEscaped = $certPath -replace '\\', '/'

Write-Host ""
Write-Host "[3/3] 写入 build-profile.json5..." -ForegroundColor Yellow

# 构建签名配置 JSON
$signingConfig = @"
{
  "app": {
    "signingConfigs": [
      {
        "name": "default",
        "type": "HarmonyOS",
        "material": {
          "certpath": "$certPathEscaped",
          "storeFile": "$storeFileEscaped",
          "keyAlias": "$($envVars['SIGNING_KEY_ALIAS'])",
          "storePassword": "$($envVars['SIGNING_STORE_PASSWORD'])",
          "keyPassword": "$($envVars['SIGNING_KEY_PASSWORD'])",
          "signAlg": "SHA256withECDSA",
          "profile": "$certPathEscaped"
        }
      }
    ],
    "products": [
      {
        "name": "default",
        "signingConfig": "default",
        "compatibleSdkVersion": "6.1.1(24)",
        "targetSdkVersion": "6.1.1(24)",
        "runtimeOS": "HarmonyOS",
        "buildOption": {
          "strictMode": {
            "caseSensitiveCheck": true,
            "useNormalizedOHMUrl": true
          }
        }
      }
    ],
    "buildModeSet": [
      {
        "name": "debug"
      },
      {
        "name": "release"
      }
    ]
  },
  "modules": [
    {
      "name": "entry",
      "srcPath": "./entry",
      "targets": [
        {
          "name": "default",
          "applyToProducts": [
            "default"
          ]
        }
      ]
    }
  ]
}
"@

# 写入文件
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($BuildProfile, $signingConfig, $utf8NoBom)

Write-Host "  ✓ build-profile.json5 已更新" -ForegroundColor Green
Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "签名配置完成！" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  密钥库：$storeFile" -ForegroundColor Gray
Write-Host "  证书：$certPath" -ForegroundColor Gray
Write-Host "  别名：$($envVars['SIGNING_KEY_ALIAS'])" -ForegroundColor Gray
Write-Host ""
Write-Host "现在可以运行：" -ForegroundColor Yellow
Write-Host "  .\build-signed.ps1   # 构建签名 HAP" -ForegroundColor Gray
Write-Host "  .\deploy.ps1         # 构建并安装到设备" -ForegroundColor Gray
Write-Host ""
Write-Host "⚠ 注意：build-profile.json5 包含密码，已在 .gitignore 中排除" -ForegroundColor Yellow
