#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
MOBILE_DIR="$REPO_ROOT/apps/mobile"

log_info() { printf '[INFO] %s\n' "$*"; }
log_warn() { printf '[WARN] %s\n' "$*" >&2; }
die() { printf '[ERROR] %s\n' "$*" >&2; exit 1; }
need_cmd() { command -v "$1" >/dev/null 2>&1 || die "缺少命令 '$1'。请先安装对应工具后重试。"; }

version_at_least() {
  local actual="$1" minimum="$2"
  awk -v actual="$actual" -v minimum="$minimum" 'function ver(v, a, n, i) { n=split(v,a,"."); for(i=1;i<=3;i++) a[i]+=0; return a[1]*1000000+a[2]*1000+a[3] } BEGIN { exit !(ver(actual) >= ver(minimum)) }'
}

ensure_go_module() {
  if [[ ! -f "$REPO_ROOT/go.mod" ]]; then
    [[ -f "$REPO_ROOT/go.mod.base" ]] || die "缺少 go.mod.base，无法初始化 Go module。"
    log_info "未找到 go.mod，复制 go.mod.base。"
    cp "$REPO_ROOT/go.mod.base" "$REPO_ROOT/go.mod"
  fi
}

ensure_mobile_dir() { [[ -d "$MOBILE_DIR" ]] || die "找不到 apps/mobile。请从项目仓库内运行此脚本。"; }

run_mobile_npm() {
  ensure_mobile_dir
  [[ -f "$MOBILE_DIR/package-lock.json" ]] || die "缺少 apps/mobile/package-lock.json，无法安全执行 npm ci。"
  (cd "$MOBILE_DIR" && npm "$@")
}

mobile_command() { ensure_mobile_dir; (cd "$MOBILE_DIR" && "$@"); }

sdk_root() {
  local root="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}"
  [[ -n "$root" ]] || die "未设置 ANDROID_HOME 或 ANDROID_SDK_ROOT。请安装 Android SDK 并设置环境变量。"
  [[ -d "$root" ]] || die "Android SDK 目录不存在：$root"
  printf '%s\n' "$root"
}

android_tool() {
  local root
  root="$(sdk_root)"
  [[ -x "$root/$1" ]] && printf '%s\n' "$root/$1" && return
  command -v "$1" 2>/dev/null || die "找不到 Android 工具 '$1'。请确认 SDK PATH 配置正确。"
}

require_android_tools() {
  android_tool platform-tools/adb >/dev/null
  android_tool emulator/emulator >/dev/null
  sdkmanager_path >/dev/null
  [[ -x "$MOBILE_DIR/android/gradlew" ]] || die "缺少 apps/mobile/android/gradlew。"
}

sdkmanager_path() {
  local root
  root="$(sdk_root)"
  if [[ -x "$root/cmdline-tools/latest/bin/sdkmanager" ]]; then
    printf '%s\n' "$root/cmdline-tools/latest/bin/sdkmanager"
  elif [[ -x "$root/tools/bin/sdkmanager" ]]; then
    printf '%s\n' "$root/tools/bin/sdkmanager"
  else
    die "找不到 sdkmanager。请安装 Android SDK Command-line Tools。"
  fi
}

select_avd() {
  local requested="${1:-${AVD:-pixel7}}" emulator_bin
  emulator_bin="$(android_tool emulator/emulator)"
  local avds
  avds="$("$emulator_bin" -list-avds)"
  printf '%s\n' "$avds" | awk -v requested="$requested" '$0 == requested { found=1 } END { exit !found }' || die "找不到 AVD '$requested'。可用 AVD：$(printf '%s' "$avds" | tr '\n' ' ' )。请使用 --avd NAME 或设置 AVD。"
  printf '%s\n' "$requested"
}

wait_for_android_boot() {
  local adb_bin="$1" avd_name="$2" attempt serial
  for attempt in $(seq 1 60); do
    if serial="$(android_serial_for_avd "$adb_bin" "$avd_name")" && [[ "$($adb_bin -s "$serial" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" == "1" ]]; then return 0; fi
    sleep 2
  done
  die "Android 模拟器启动超时，请检查 emulator 日志和 adb devices。"
}

android_serial_for_avd() {
  local adb_bin="$1" avd_name="$2" serial state running_avd
  while read -r serial state; do
    [[ "$state" == "device" ]] || continue
    running_avd="$($adb_bin -s "$serial" shell getprop ro.boot.qemu.avd_name 2>/dev/null | tr -d '\r')"
    if [[ "$running_avd" == "$avd_name" ]]; then
      printf '%s\n' "$serial"
      return 0
    fi
  done < <("$adb_bin" devices | awk 'NR > 1 {print $1, $2}')
  return 1
}

start_android_avd() {
  local avd_name="$1" emulator_bin="$2" adb_bin="$3" serial
  if serial="$(android_serial_for_avd "$adb_bin" "$avd_name")"; then
    export ANDROID_SERIAL="$serial"
    return
  fi
  log_info "启动 Android AVD：$avd_name"
  "$emulator_bin" -avd "$avd_name" -no-snapshot >/dev/null 2>&1 &
  wait_for_android_boot "$adb_bin" "$avd_name"
  serial="$(android_serial_for_avd "$adb_bin" "$avd_name")" || die "已启动 AVD '$avd_name'，但 adb 未识别到对应设备。"
  export ANDROID_SERIAL="$serial"
}

require_ios_tools() {
  need_cmd xcodebuild; need_cmd xcrun
  xcrun simctl help >/dev/null 2>&1 || die "xcrun simctl 不可用，请安装并选择 Xcode。"
  [[ -d "$MOBILE_DIR/ios" ]] || die "缺少 apps/mobile/ios iOS 工程。"
  if [[ -f "$MOBILE_DIR/ios/App/Podfile" || -f "$MOBILE_DIR/ios/Podfile" ]]; then
    need_cmd pod
  else
    log_info "检测到 Capacitor SPM 工程，不要求 CocoaPods。"
  fi
}
