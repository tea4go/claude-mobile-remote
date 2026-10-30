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

function Set-ProcessEnvironment {
  param([hashtable]$Overrides = @{})
  $snapshot = @{}
  foreach ($name in $Overrides.Keys) {
    $snapshot[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
    $value = $Overrides[$name]
    if ($null -eq $value) { Remove-Item -Path ("Env:{0}" -f $name) -ErrorAction SilentlyContinue }
    else { [Environment]::SetEnvironmentVariable($name, [string]$value, 'Process') }
  }
  return $snapshot
}

function Restore-ProcessEnvironment {
  param([hashtable]$Snapshot = @{})
  foreach ($name in $Snapshot.Keys) {
    $value = $Snapshot[$name]
    if ($null -eq $value) { Remove-Item -Path ("Env:{0}" -f $name) -ErrorAction SilentlyContinue }
    else { [Environment]::SetEnvironmentVariable($name, [string]$value, 'Process') }
  }
}

function Get-ProcessEnvironmentValue([string]$Name, [string]$Fallback = '<默认>') {
  $value = [Environment]::GetEnvironmentVariable($Name, 'Process')
  if ([string]::IsNullOrWhiteSpace($value)) { return $Fallback }
  return $value
}

function Get-GoEnvironmentProfiles {
  $profiles = New-Object System.Collections.Generic.List[object]
  $seen = New-Object System.Collections.Generic.HashSet[string]

  foreach ($profile in @(
    @{
      Name = '继承当前 Go 环境'
      Environment = @{}
    },
    @{
      Name = '官方 Go 代理'
      Environment = @{
        GOPROXY = 'https://proxy.golang.org,direct'
        GOSUMDB = 'sum.golang.org'
      }
    },
    @{
      Name = '源站直连'
      Environment = @{
        GOPROXY = 'direct'
        GOSUMDB = 'sum.golang.org'
      }
    }
  )) {
    $signature = ($profile.Environment.GetEnumerator() | Sort-Object Name | ForEach-Object { '{0}={1}' -f $_.Name, $_.Value }) -join ';'
    if ($seen.Add($signature)) { [void]$profiles.Add($profile) }
  }

  return $profiles.ToArray()
}

function Get-GoEnvironmentSummary {
  param([hashtable]$Environment = @{})
  if (-not $Environment -or $Environment.Count -eq 0) {
    return ('GOPROXY={0}; GOSUMDB={1}' -f `
      (Get-ProcessEnvironmentValue -Name 'GOPROXY'), `
      (Get-ProcessEnvironmentValue -Name 'GOSUMDB'))
  }
  return (($Environment.GetEnumerator() | Sort-Object Name | ForEach-Object { '{0}={1}' -f $_.Name, $_.Value }) -join '; ')
}

function Invoke-GoWithFallback {
  param([Parameter(Mandatory = $true)][object[]]$Arguments)

  $attemptErrors = New-Object System.Collections.Generic.List[string]
  foreach ($profile in Get-GoEnvironmentProfiles) {
    $summary = Get-GoEnvironmentSummary -Environment $profile.Environment
    Write-Info ("Go 依赖下载策略：{0}（{1}）" -f $profile.Name, $summary)
    $snapshot = Set-ProcessEnvironment -Overrides $profile.Environment
    try {
      Invoke-External -FilePath 'go' -Arguments $Arguments
      return
    } catch {
      $message = $_.Exception.Message
      [void]$attemptErrors.Add(('{0}: {1}' -f $profile.Name, $message))
      Write-Warn ("{0} 失败：{1}" -f $profile.Name, $message)
    } finally {
      Restore-ProcessEnvironment -Snapshot $snapshot
    }
  }

  Fail ("Go 命令在所有下载策略下均失败：{0}{1}" -f [Environment]::NewLine, ($attemptErrors -join [Environment]::NewLine))
}

# PowerShell 5.1 在 $ErrorActionPreference='Stop' 下，会把原生命令经 2> 重定向的 stderr 包装成终止错误；
# 这里临时放宽到 Continue，以便丢弃 stderr、只捕获 stdout（裸 & 调用不受此影响，但会透传 stderr 噪音）。
function Invoke-Capture {
  param(
    [Parameter(Mandatory = $true)][string]$FilePath,
    [object[]]$Arguments = @()
  )
  $prev = $ErrorActionPreference
  try {
    $ErrorActionPreference = 'Continue'
    & $FilePath @Arguments 2>$null
  } finally {
    $ErrorActionPreference = $prev
  }
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
  # $Command 是 gradle wrapper（apps/mobile/android/gradlew.bat）的完整路径，Gradle 工程根目录是 wrapper 所在目录，
  # 而非 apps/mobile。必须从该目录执行，否则 Gradle 会报 "does not contain a Gradle build"。
  $dir = Split-Path -Parent $Command
  if (-not (Test-Path -LiteralPath $dir)) { Fail "找不到 Android 工程目录: $dir" }
  Push-Location $dir
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
  $sdkHome = Get-AndroidHome
  if ($sdkHome) {
    $relative = switch ($Name) {
      'adb' { 'platform-tools\adb.exe' }
      'emulator' { 'emulator\emulator.exe' }
      'sdkmanager' { 'cmdline-tools\latest\bin\sdkmanager.bat' }
      default { $null }
    }
    if ($relative) {
      $candidate = Join-Path $sdkHome $relative
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

# 读取 JDK home 下 release 文件的主版本号（JAVA_VERSION="21.0.10" → 21），无法识别返回 0。
function Get-JavaMajor([string]$JavaHome) {
  if (-not $JavaHome) { return 0 }
  $release = Join-Path $JavaHome 'release'
  if (-not (Test-Path -LiteralPath $release)) { return 0 }
  $content = Get-Content -LiteralPath $release -Raw
  $match = [regex]::Match($content, 'JAVA_VERSION="(\d+)')
  if ($match.Success) { return [int]$match.Groups[1].Value }
  return 0
}

# Gradle 实际会使用的 JDK home：优先 JAVA_HOME，其次 PATH 上的 java。
function Get-EffectiveJavaHome {
  if ($env:JAVA_HOME) {
    $bin = Join-Path $env:JAVA_HOME 'bin\java.exe'
    if (Test-Path -LiteralPath $bin) { return $env:JAVA_HOME }
  }
  $command = Get-Command 'java' -ErrorAction SilentlyContinue
  if ($command) { return (Split-Path -Parent (Split-Path -Parent $command.Source)) }
  return $null
}

# 解析一个兼容的 JDK home。Capacitor 8 生成的 capacitor.build.gradle 写死 source/target=Java 21，
# 而 Gradle 8.14.3 只能运行在 JDK ≤ 24（JDK 26 会触发 Groovy "Unsupported class file major version 70"），
# 因此要求 21–24。当前 JDK 不兼容时回退到 Android Studio 内置 JBR（JDK 21）；找不到返回 $null。
function Resolve-AndroidJdk {
  $effective = Get-EffectiveJavaHome
  $major = Get-JavaMajor $effective
  if ($major -ge 21 -and $major -le 24) { return $effective }
  $jbrCandidates = @(
    (Join-Path $env:LOCALAPPDATA 'Programs\Android Studio\jbr'),
    (Join-Path $env:ProgramFiles 'Android\Android Studio\jbr'),
    'C:\Program Files\Android\Android Studio\jbr'
  )
  foreach ($jbr in $jbrCandidates) {
    $jbrMajor = Get-JavaMajor $jbr
    if ($jbrMajor -ge 21 -and $jbrMajor -le 24) { return $jbr }
  }
  return $null
}

function Test-AndroidEnvironment {
  $errors = New-Object System.Collections.Generic.List[string]
  $sdkHome = Get-AndroidHome
  if (-not $sdkHome) { [void]$errors.Add('未找到 ANDROID_HOME 或 ANDROID_SDK_ROOT 指向的 Android SDK。') }
  foreach ($tool in @('adb', 'emulator')) {
    if (-not (Get-AndroidTool $tool)) { [void]$errors.Add("未找到 $tool，请通过 Android Studio 或 command-line tools 安装。") }
  }
  if (-not (Get-AndroidTool 'sdkmanager')) { [void]$errors.Add('未找到 sdkmanager，请安装 Android SDK Command-line Tools。') }
  $variables = Join-Path $script:MobileRoot 'android\variables.gradle'
  if ($sdkHome -and (Test-Path -LiteralPath $variables)) {
    $content = Get-Content -LiteralPath $variables -Raw
    $match = [regex]::Match($content, 'compileSdkVersion\s*=\s*(\d+)')
    if ($match.Success) {
      $platform = Join-Path $sdkHome ("platforms\android-{0}" -f $match.Groups[1].Value)
      if (-not (Test-Path -LiteralPath $platform)) { [void]$errors.Add("缺少 SDK platform android-$($match.Groups[1].Value)。请用 sdkmanager 安装。") }
    }
  }
  if ($sdkHome) {
    $buildTools = Join-Path $sdkHome 'build-tools'
    $requiredBuildTools = $null
    if (Test-Path -LiteralPath $variables) {
      $match = [regex]::Match($content, 'buildToolsVersion\s*=\s*[''"]([^''"]+)[''"]')
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
  if ($errors) { Fail (($errors -join [Environment]::NewLine) + [Environment]::NewLine + '安装建议：Android Studio SDK Manager 或官方 command-line tools。') }
  $wrapper = Get-GradleWrapper
  if (-not $wrapper) { Fail '找不到 apps/mobile/android/gradlew.bat（或 gradlew）。请确认 Capacitor Android 工程已生成。' }
  $jdk = Resolve-AndroidJdk
  if (-not $jdk) {
    $effective = Get-EffectiveJavaHome
    $major = Get-JavaMajor $effective
    $current = if ($effective) { "$effective (JDK $major)" } else { '未检测到' }
    Fail "未找到兼容的 JDK（需 21–24）。当前 java：$current。Capacitor 8 要求 Java 21，Gradle 8.14.3 最高支持 JDK 24；请安装 JDK 21（Android Studio 内置 JBR 即可，或运行 jvms install 21 后切换）。"
  }
  if ((Get-JavaMajor (Get-EffectiveJavaHome)) -notin 21..24) {
    Write-Info "当前 JAVA_HOME 指向的 JDK 不兼容（需 21–24），已改用兼容 JDK：$jdk"
  }
  $env:JAVA_HOME = $jdk
  return $wrapper
}

function Get-AvdNames {
  $emulator = Get-AndroidTool 'emulator'
  if (-not $emulator) { return @() }
  return @(Invoke-Capture $emulator '-list-avds' | Where-Object { $_ -and $_.Trim() })
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
  # 禁用 Wifi/virtio-wifi 以避免 emulator 的 netsimd.exe（网络模拟守护进程）崩溃挂死 guest CPU——本机 netsimd 在 ntdll.dll 访问违规，会阻塞 QEMU 主循环导致内核永不启动
  # -no-snapshot：跳过 quickboot 快照加载，强制冷启动。本机残留快照（在禁用 Wifi 前/崩溃态保存）加载后 guest 会卡死在 offline，adb 永远等不到 boot_completed。
  $proc = Start-Process -FilePath $emulator -ArgumentList @('-avd', $AvdName, '-feature', '-Wifi', '-feature', '-VirtioWifi', '-no-snapshot') -PassThru
  $ready = $false
  $start = Get-Date
  $deadline = $start.AddMinutes(5)
  $lastState = ''
  $lastTick = $start
  while ((Get-Date) -lt $deadline) {
    Start-Sleep -Seconds 3
    if ($proc.HasExited) { Fail "模拟器进程已意外退出（exit code $($proc.ExitCode)）。可运行 emulator -avd $AvdName -verbose 单独启动查看报错。" }
    $serial = Get-AvdSerial $adb $AvdName
    if ($serial) {
      $boot = (Invoke-Capture $adb @('-s', $serial, 'shell', 'getprop', 'sys.boot_completed') | Out-String).Trim()
      if ($boot -match '1') { $ready = $true; $env:ANDROID_SERIAL = $serial; break }
      $state = '设备已连接，系统启动中'
    } else {
      if ((Invoke-Capture $adb 'devices' | Out-String) -match 'offline') { $state = 'offline（模拟器进程已启动，系统未就绪）' } else { $state = '等待 adb 识别设备' }
    }
    $now = Get-Date
    if ($state -ne $lastState) { Write-Info "等待模拟器启动：$state"; $lastState = $state; $lastTick = $now }
    elseif (($now - $lastTick).TotalSeconds -ge 30) { Write-Info "仍在等待模拟器启动（已 $([int]($now - $start).TotalSeconds) 秒）..."; $lastTick = $now }
  }
  if (-not $ready) { Fail "Android AVD '$AvdName' 在 300 秒内未启动完成。可运行 emulator -avd $AvdName -verbose 单独启动查看报错。" }
}

function Get-AvdSerial([string]$Adb, [string]$AvdName) {
  $rows = Invoke-Capture $Adb 'devices'
  foreach ($row in $rows) {
    $parts = ($row -split '\s+')
    if ($parts.Count -ge 2 -and $parts[1] -eq 'device') {
      $running = (Invoke-Capture $Adb @('-s', $parts[0], 'shell', 'getprop', 'ro.boot.qemu.avd_name') | Out-String).Trim()
      if ($running -eq $AvdName) { return $parts[0] }
    }
  }
  return $null
}

# 从 apps/mobile/android/app/build.gradle 读取 applicationId（App 包名）。
function Get-AndroidAppId {
  $buildGradle = Join-Path $script:MobileRoot 'android\app\build.gradle'
  if (-not (Test-Path -LiteralPath $buildGradle)) { return $null }
  $content = Get-Content -LiteralPath $buildGradle -Raw
  $match = [regex]::Match($content, 'applicationId\s+[''"]([^''"]+)[''"]')
  if ($match.Success) { return $match.Groups[1].Value }
  return $null
}

# 通过 am start 显式拉起入口 Activity；namespace 与发布 applicationId 独立。
# installDebug 只安装、不启动，这里补上启动步骤。
function Start-AndroidApp {
  param([Parameter(Mandatory = $true)][string]$AppId)
  $adb = Get-AndroidTool 'adb'
  if (-not $adb) { Fail '缺少 adb，无法启动 App。' }
  Write-Info "启动 App: $AppId"
  $null = Invoke-Capture $adb @('shell', 'am', 'start', '-n', "$AppId/com.tea4go.claudemobileremote.MainActivity")
}

# 探测本机某个 TCP 端口是否可连接，用于判断 -Live 模式依赖的 Web dev server 是否已在运行。
function Test-TcpPort([int]$Port, [string]$Address = '127.0.0.1') {
  $client = New-Object System.Net.Sockets.TcpClient
  try {
    $client.Connect($Address, $Port)
    return $true
  } catch {
    return $false
  } finally {
    $client.Dispose()
  }
}
