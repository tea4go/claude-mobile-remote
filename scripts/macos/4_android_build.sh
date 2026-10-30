#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/_common.sh"
usage() { cat <<'EOF'
用法：bash scripts/macos/4_android_build.sh [--release]
  默认构建 debug；--release 构建并签名正式 APK。
  Release 读取 CMR_ANDROID_SIGNING_CONFIG 或仓库外 ~/.config/claude-mobile-remote/android-signing/signing.json。
  本脚本不自动生成或轮换密钥；首次准备见 docs/android-release.md。
EOF
}
release=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --release) release=1; shift ;;
    --help) usage; exit 0 ;;
    *) usage >&2; die "未知选项：$1" ;;
  esac
done
require_android_tools
"$SCRIPT_DIR/4_web_build.sh"
release_mode=0
[[ "$release" -eq 0 ]] || release_mode=1
mobile_command env -u CAP_SERVER_URL CMR_NATIVE_TARGET=android CMR_ANDROID_RELEASE="$release_mode" npm exec -- cap sync android
if [[ "$release" -eq 1 ]]; then
  (cd "$MOBILE_DIR/android" && ./gradlew -PcmrPushEnabled=false assembleRelease)
  apk_path="$MOBILE_DIR/android/app/build/outputs/apk/release/app-release-unsigned.apk"
  [[ -f "$apk_path" ]] || die "未找到未签名 APK：$apk_path"
  python3 "$SCRIPT_DIR/android_release.py" --config "${CMR_ANDROID_SIGNING_CONFIG:-$HOME/.config/claude-mobile-remote/android-signing/signing.json}" --apk "$apk_path"
  apk_path="$REPO_ROOT/.local-tools/android-release/WhaleRemoteControl-1.0.0-android-signed.apk"
else
  (cd "$MOBILE_DIR/android" && ./gradlew assembleDebug)
  apk_path="$MOBILE_DIR/android/app/build/outputs/apk/debug/ClaudeMobileRemote.apk"
fi
[[ -f "$apk_path" ]] || die "构建结束但未找到 APK：$apk_path"
log_info "APK：$apk_path"
