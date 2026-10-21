#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/_common.sh"
usage() { cat <<'EOF'
用法：bash scripts/macos/3_ios_device.sh
安装前端依赖、构建并同步 iOS，打开 Xcode。
在 Xcode 中选择连接的 iPhone、确认 Signing & Capabilities，然后点击 Run 编译安装。
EOF
}
if [[ $# -gt 0 && "$1" == "--help" ]]; then usage; exit 0; fi
[[ $# -eq 0 ]] || { usage >&2; die "未知选项：$1"; }
need_cmd npm
need_cmd xcodebuild
ensure_mobile_dir
ensure_jpush_sdk
[[ -d "$MOBILE_DIR/ios" ]] || die "缺少 apps/mobile/ios iOS 工程。"
if [[ -f "$MOBILE_DIR/ios/App/Podfile" || -f "$MOBILE_DIR/ios/Podfile" ]]; then
  need_cmd pod
fi
# 真机使用打包资源，避免继承 Android 模拟器的开发服务器地址。
unset CAP_SERVER_URL
run_mobile_npm ci
run_mobile_npm run build
mobile_command npm exec -- cap sync ios
log_info "即将打开 Xcode：选择你的 iPhone，确认签名后点击 Run（▶）编译安装。"
mobile_command npm exec -- cap open ios
