#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/_common.sh"
usage() { cat <<'EOF'
用法：bash scripts/macos/android-build.sh [--release]
  默认构建 debug；--release 构建未签名 release，不读取或生成 keystore。
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
"$SCRIPT_DIR/web-build.sh"
mobile_command npm exec -- cap sync android
if [[ "$release" -eq 1 ]]; then
  log_warn "正在构建 release；签名需由外部发布流程处理。"
  (cd "$MOBILE_DIR/android" && ./gradlew assembleRelease)
else
  (cd "$MOBILE_DIR/android" && ./gradlew assembleDebug)
fi
