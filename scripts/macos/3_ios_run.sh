#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/_common.sh"
usage() { cat <<'EOF'
用法：bash scripts/macos/3_ios_run.sh [--simulator NAME]
仅运行到 Xcode iPhone Simulator，不连接或签名真机。
EOF
}
simulator=
while [[ $# -gt 0 ]]; do
  case "$1" in
    --simulator) [[ $# -ge 2 ]] || die "--simulator 需要值"; simulator="$2"; shift 2 ;;
    --help) usage; exit 0 ;;
    *) usage >&2; die "未知选项：$1" ;;
  esac
done
require_ios_tools
ensure_jpush_sdk
if [[ -z "$simulator" ]]; then
  devices="$(xcrun simctl list devices available)"
  simulator="$(printf '%s\n' "$devices" | sed -nE 's/^[[:space:]]+(iPhone[^()]*) \(([0-9A-Fa-f-]+)\) \((Booted|Shutdown)\)[[:space:]]*$/\2/p' | awk 'NR == 1 { print }')"
  [[ -n "$simulator" ]] || die "没有可用的 iPhone Simulator，请在 Xcode 中安装运行时。"
fi
"$SCRIPT_DIR/4_web_build.sh"
mobile_command npm exec -- cap sync ios
mobile_command npm exec -- cap run ios --target "$simulator"
