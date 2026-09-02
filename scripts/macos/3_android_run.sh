#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/_common.sh"
usage() { cat <<'EOF'
用法：bash scripts/macos/3_android_run.sh (--live|--dist) [--avd NAME] [--port PORT]
  --live  使用 http://10.0.2.2:PORT 的宿主机 Web dev 资源（需另行启动 Web）
  --dist  使用本地生产构建资源
EOF
}
mode= avd= port=5173
while [[ $# -gt 0 ]]; do
  case "$1" in
    --live|--dist) [[ -z "$mode" ]] || die "--live 与 --dist 只能选择一个"; mode="$(printf '%s' "$1" | sed 's/^--//')"; shift ;;
    --avd) [[ $# -ge 2 ]] || die "--avd 需要值"; avd="$2"; shift 2 ;;
    --port) [[ $# -ge 2 ]] || die "--port 需要值"; port="$2"; shift 2 ;;
    --help) usage; exit 0 ;;
    *) usage >&2; die "未知选项：$1" ;;
  esac
done
[[ -n "$mode" ]] || { usage >&2; die "必须指定 --live 或 --dist"; }
[[ "$port" =~ ^[0-9]+$ && "$port" -ge 1 && "$port" -le 65535 ]] || die "--port 必须是 1-65535 的端口号"
ensure_mobile_dir
require_android_tools
selected_avd="$(select_avd "$avd")"
adb_bin="$(android_tool platform-tools/adb)"
emulator_bin="$(android_tool emulator/emulator)"
start_android_avd "$selected_avd" "$emulator_bin" "$adb_bin"
if [[ "$mode" == live ]]; then
  (cd "$MOBILE_DIR" && CAP_SERVER_URL="http://10.0.2.2:$port" npm exec -- cap sync android)
else
  "$SCRIPT_DIR/4_web_build.sh"
  (cd "$MOBILE_DIR" && env -u CAP_SERVER_URL npm exec -- cap sync android)
fi
(cd "$MOBILE_DIR/android" && ./gradlew installDebug)
