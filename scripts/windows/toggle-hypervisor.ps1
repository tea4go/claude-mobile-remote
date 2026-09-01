param(
  [Parameter(Mandatory = $true)]
  [ValidateSet('Emulator', 'Whpx', 'Wsl2', 'Status')]
  [string]$Mode
)

# 在三种 Hyper-V 相关配置之间切换，用于在 WSL2 与 Android 模拟器之间切换，或诊断虚拟化状态。
# 背景：Windows 上 WSL2 与模拟器共用 Hyper-V 虚拟机监控程序。模拟器有两个加速后端：
#   - AEHD（Android Emulator Hypervisor Driver）：要求 Hyper-V 关。
#   - WHPX（Windows Hypervisor Platform）：要求 Hyper-V 开；官方推荐路径。WHPX 会被 VBS（虚拟化安全）破坏，
#     所以用 WHPX 时必须同时关掉 VBS（EnableVirtualizationBasedSecurity = 0）。
# 注意：早前模拟器「卡死」的真正根因不是 hypervisor，而是默认 virtio-wifi 拉起宿主 netsimd.exe
#   （网络模拟守护进程）在 ntdll.dll access-violation 崩溃、阻塞 QEMU 主循环，导致 WHPX/AEHD/TCG 一律卡死；
#   该问题已由 _common.ps1 的 Start-AndroidAvd 用 `-feature -Wifi -feature -VirtioWifi` 禁用 wifi 解决，WHPX 下已验证。
#   因此日常用默认 WHPX 即可，本脚本主要用于 WSL2/模拟器切换与 Status 诊断。
# 三种模式（切换后均需重启电脑，需管理员权限）：
#   Emulator = AEHD：Hyper-V 关 + VBS 关
#   Whpx     = WHPX：Hyper-V 开 + VBS 关（推荐）
#   Wsl2     = Hyper-V 开 + VBS 开（WSL2 可用）

function Write-Step([string]$Message) { Write-Host "[>] $Message" -ForegroundColor Cyan }

$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
  Write-Host '需要管理员权限。请在开始菜单右键 PowerShell，选择「以管理员身份运行」后重试。' -ForegroundColor Red
  exit 1
}

if ($Mode -eq 'Status') {
  Write-Step '虚拟机监控程序状态'
  bcdedit /enum '{current}' | Select-String 'hypervisorlaunchtype' | ForEach-Object { Write-Host ("  " + $_.Line.Trim()) }
  $hp = (Get-CimInstance Win32_ComputerSystem).HypervisorPresent
  Write-Host ("  HypervisorPresent = {0}" -f $hp)
  if ($hp) { Write-Host '    -> Hyper-V 激活：模拟器走 WHPX，WSL2 可用。' -ForegroundColor Yellow }
  else { Write-Host '    -> Hyper-V 关闭：模拟器走 AEHD，WSL2 停用。' -ForegroundColor Green }
  $vbs = (Get-ItemProperty "HKLM:\SYSTEM\CurrentControlSet\Control\DeviceGuard" -ErrorAction SilentlyContinue).EnableVirtualizationBasedSecurity
  $vbsText = if ($null -eq $vbs) { '未设置' } else { "$vbs" }
  Write-Host ("  EnableVirtualizationBasedSecurity = {0}  （1=开着，会破坏 WHPX）" -f $vbsText)
  $aehd = Get-Service aehd -ErrorAction SilentlyContinue
  if ($aehd) { Write-Host ("  AEHD 服务：{0} / 启动类型 {1}" -f $aehd.Status, $aehd.StartType) } else { Write-Host '  AEHD 服务未安装。' }
  exit 0
}

$hypervisorTarget = switch ($Mode) {
  'Emulator' { 'off' }
  'Whpx' { 'auto' }
  'Wsl2' { 'auto' }
}
$vbsValue = switch ($Mode) {
  'Emulator' { 0 }
  'Whpx' { 0 }
  'Wsl2' { 1 }
}
$desc = switch ($Mode) {
  'Emulator' { '模拟器模式（AEHD：Hyper-V 关 + VBS 关）' }
  'Whpx' { '模拟器模式（WHPX：Hyper-V 开 + VBS 关，推荐）' }
  'Wsl2' { 'WSL2 模式（Hyper-V 开 + VBS 开）' }
}

Write-Step "bcdedit /set hypervisorlaunchtype $hypervisorTarget"
bcdedit /set hypervisorlaunchtype $hypervisorTarget
if ($LASTEXITCODE -ne 0) { Write-Host 'bcdedit 执行失败。' -ForegroundColor Red; exit 1 }
Write-Step "EnableVirtualizationBasedSecurity = $vbsValue"
reg add "HKLM\SYSTEM\CurrentControlSet\Control\DeviceGuard" /v EnableVirtualizationBasedSecurity /t REG_DWORD /d $vbsValue /f | Out-Null
if ($LASTEXITCODE -ne 0) { Write-Host '设置 VBS 失败。' -ForegroundColor Red; exit 1 }

Write-Host ''
Write-Host ("已切换到「{0}」。请重启电脑后生效。" -f $desc) -ForegroundColor Green
if ($Mode -eq 'Emulator') {
  Write-Host '重启后验证与运行：' -ForegroundColor Yellow
  Write-Host '  .\toggle-hypervisor.ps1 Status'
  Write-Host '  .\android-run.ps1 -Live -Avd Medium_Phone'
} elseif ($Mode -eq 'Whpx') {
  Write-Host '重启后验证与运行：' -ForegroundColor Yellow
  Write-Host '  .\toggle-hypervisor.ps1 Status          # HypervisorPresent=True 且 EnableVirtualizationBasedSecurity=0'
  Write-Host '  emulator -avd Medium_Phone -accel-check   # 应显示 WHPX 可用'
  Write-Host '  .\android-run.ps1 -Live -Avd Medium_Phone'
} else {
  Write-Host '重启后 WSL2（Ubuntu）即可恢复。' -ForegroundColor Yellow
}
