#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

usage() { cat <<'EOF'
用法：bash scripts/macos/2_check.sh [选项]
  --android  检查 Android SDK、adb、emulator、sdkmanager 和 Gradle wrapper
  --ios      检查 Xcode Simulator、CocoaPods 和 iOS 工程
  --help     显示帮助
EOF
}

check_cmd_version() {
  local command_name="$1" minimum="$2" version_command="$3" advice="$4" output version
  if ! command -v "$command_name" >/dev/null 2>&1; then log_warn "缺少 $command_name。安装建议：$advice"; return 1; fi
  output="$(eval "$version_command" 2>&1)"
  output="$(printf '%s\n' "$output" | awk 'NR == 1 { print; exit }')"
  version="$(printf '%s\n' "$output" | awk 'match($0, /[0-9]+(\.[0-9]+)*/) { print substr($0, RSTART, RLENGTH); exit }')"
  if [[ -z "$version" ]] || ! version_at_least "$version" "$minimum"; then log_warn "$command_name 版本过低或无法识别（需要 >= $minimum）：$output"; return 1; fi
  log_info "$command_name $version"
}

android_checks() {
  local root="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}" java_output java_version
  local failed=0
  if [[ -z "$root" || ! -d "$root" ]]; then log_warn "缺少 Android SDK。请安装 Android Studio/Command-line Tools，并设置 ANDROID_HOME 或 ANDROID_SDK_ROOT。"; return 1; fi
  [[ -x "$root/platform-tools/adb" ]] || { log_warn "缺少 adb；请安装 platform-tools。"; failed=1; }
  [[ -x "$root/emulator/emulator" ]] || { log_warn "缺少 emulator；请安装 Android Emulator。"; failed=1; }
  [[ -x "$root/cmdline-tools/latest/bin/sdkmanager" || -x "$root/tools/bin/sdkmanager" ]] || { log_warn "缺少 sdkmanager；请安装 Android SDK Command-line Tools。"; failed=1; }
  if ! command -v java >/dev/null 2>&1; then
    log_warn "缺少 Java/JDK；请安装 JDK 21 或 Android Studio 自带 JBR。"
    failed=1
  else
    java_output="$(java -version 2>&1)"
    java_output="$(printf '%s\n' "$java_output" | awk 'NR == 1 { print; exit }')"
    java_version="$(printf '%s\n' "$java_output" | awk 'match($0, /[0-9]+(\.[0-9]+)*/) { print substr($0, RSTART, RLENGTH); exit }')"
    if [[ -z "$java_version" ]] || ! version_at_least "$java_version" 17; then
      log_warn "Java 版本过低或无法识别（建议 >= 17）：$java_output"
      failed=1
    else
      log_info "Java $java_version"
    fi
  fi
  [[ -d "$root/platforms/android-36" ]] || { log_warn "缺少 Android platform android-36；请执行 sdkmanager 'platforms;android-36'。"; failed=1; }
  [[ -d "$root/build-tools" ]] && find "$root/build-tools" -mindepth 1 -maxdepth 1 -type d -print -quit | awk 'NF { found=1 } END { exit !found }' || { log_warn "缺少 Android build-tools；请通过 Android Studio 或 sdkmanager 安装。"; failed=1; }
  [[ -x "$MOBILE_DIR/android/gradlew" ]] || { log_warn "缺少 apps/mobile/android/gradlew。"; failed=1; }
  [[ "$failed" -eq 0 ]] && log_info "Android SDK 检查通过：$root"
  return "$failed"
}

ios_checks() {
  local failed=0
  command -v xcodebuild >/dev/null 2>&1 || { log_warn "缺少 xcodebuild。请安装 Xcode。"; failed=1; }
  command -v xcrun >/dev/null 2>&1 && xcrun simctl help >/dev/null 2>&1 || { log_warn "xcrun simctl 不可用，请安装并选择 Xcode。"; failed=1; }
  [[ -d "$MOBILE_DIR/ios" ]] || { log_warn "缺少 apps/mobile/ios iOS 工程。"; failed=1; }
  if [[ -f "$MOBILE_DIR/ios/App/Podfile" || -f "$MOBILE_DIR/ios/Podfile" ]]; then
    command -v pod >/dev/null 2>&1 || { log_warn "缺少 CocoaPods。请执行 brew install cocoapods。"; failed=1; }
  else
    log_info "检测到 Capacitor SPM 工程，不要求 CocoaPods。"
  fi
  [[ "$failed" -eq 0 ]] && log_info "iOS Simulator 检查通过。"
  return "$failed"
}

android=0 ios=0 failed=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --android) android=1; shift ;;
    --ios) ios=1; shift ;;
    --help) usage; exit 0 ;;
    *) usage >&2; die "未知选项：$1" ;;
  esac
done
check_cmd_version go 1.25 'go version' 'brew install go' || failed=1
check_cmd_version node 20 'node --version' 'brew install node' || failed=1
command -v npm >/dev/null 2>&1 || { log_warn "缺少 npm。请安装 Node.js（brew install node）。"; failed=1; }
command -v npm >/dev/null 2>&1 && log_info "npm $(npm --version)"
if [[ "$android" -eq 1 ]]; then android_checks || failed=1; fi
if [[ "$ios" -eq 1 ]]; then ios_checks || failed=1; fi
[[ "$failed" -eq 0 ]] || exit 1
log_info "基础环境检查通过。"
