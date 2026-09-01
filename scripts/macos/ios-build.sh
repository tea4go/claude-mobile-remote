#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/_common.sh"
usage() { cat <<'EOF'
用法：bash scripts/macos/ios-build.sh
构建 iOS Simulator Debug 产物；不触碰签名配置。
EOF
}
if [[ $# -gt 0 && "$1" == "--help" ]]; then usage; exit 0; fi
[[ $# -eq 0 ]] || { usage >&2; die "未知选项：$1"; }
require_ios_tools
"$SCRIPT_DIR/web-build.sh"
mobile_command npm exec -- cap sync ios
workspace="$MOBILE_DIR/ios/App/App.xcworkspace"
if [[ ! -d "$workspace" ]]; then
  workspace="$MOBILE_DIR/ios/App/App.xcodeproj/project.xcworkspace"
fi
[[ -d "$workspace" ]] || die "缺少 iOS workspace：$workspace。请先完成 Capacitor 原生依赖准备。"
xcodebuild -workspace "$workspace" -scheme App -configuration Debug -sdk iphonesimulator -destination 'generic/platform=iOS Simulator' build
