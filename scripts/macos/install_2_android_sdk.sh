#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

SDK_ROOT="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-$HOME/Library/Android/sdk}}"
CHECK_ONLY=0
usage() { cat <<'EOF'
用法：bash scripts/macos/install_2_android_sdk.sh [选项]
  --sdk-root PATH  指定 Android SDK 根目录
  --check-only     只检查 sdkmanager 和 adb，不安装组件
  --help           显示帮助
EOF
}
while [[ $# -gt 0 ]]; do
  case "$1" in
    --sdk-root) [[ $# -ge 2 ]] || die "--sdk-root 需要值"; SDK_ROOT="$2"; shift 2 ;;
    --check-only) CHECK_ONLY=1; shift ;;
    --help) usage; exit 0 ;;
    *) usage >&2; die "未知选项：$1" ;;
  esac
done

mkdir -p "$SDK_ROOT"
export ANDROID_HOME="$SDK_ROOT" ANDROID_SDK_ROOT="$SDK_ROOT"
export PATH="$SDK_ROOT/cmdline-tools/latest/bin:$SDK_ROOT/platform-tools:$PATH"
if ! command -v sdkmanager >/dev/null 2>&1; then
  [[ "$CHECK_ONLY" -eq 0 ]] || die "找不到 sdkmanager。"
  need_cmd brew
  brew install --cask android-commandlinetools
  export PATH="$SDK_ROOT/cmdline-tools/latest/bin:$PATH"
fi
sdkmanager_bin="$(command -v sdkmanager)"
command -v adb >/dev/null 2>&1 || [[ -x "$SDK_ROOT/platform-tools/adb" ]] || { [[ "$CHECK_ONLY" -eq 1 ]] && die "找不到 adb。" || true; }
[[ "$CHECK_ONLY" -eq 1 ]] && { log_info "Android SDK 工具检查完成：$SDK_ROOT"; exit 0; }

set +o pipefail
yes | "$sdkmanager_bin" --sdk_root="$SDK_ROOT" --licenses >/dev/null
set -o pipefail
"$sdkmanager_bin" --sdk_root="$SDK_ROOT" \
  "platform-tools" "platforms;android-36" "build-tools;36.0.0"
[[ -x "$SDK_ROOT/platform-tools/adb" ]] || die "SDK 组件安装后仍找不到 adb：$SDK_ROOT/platform-tools/adb"
log_info "Android SDK 安装完成：$SDK_ROOT"
log_info "当前终端已设置 ANDROID_HOME/ANDROID_SDK_ROOT。新终端请加入 ~/.zshrc："
printf 'export ANDROID_HOME="%s"\n' "$SDK_ROOT"
printf 'export ANDROID_SDK_ROOT="$ANDROID_HOME"\n'
printf 'export PATH="$ANDROID_HOME/cmdline-tools/latest/bin:$ANDROID_HOME/platform-tools:$PATH"\n'
