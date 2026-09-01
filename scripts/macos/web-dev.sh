#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"
usage() { cat <<'EOF'
用法：bash scripts/macos/web-dev.sh [--host VALUE] [--port VALUE]
默认仅监听 localhost；需要手机/模拟器访问时显式传 --host 0.0.0.0。
EOF
}
host=127.0.0.1 port=
while [[ $# -gt 0 ]]; do
  case "$1" in
    --host) [[ $# -ge 2 ]] || die "--host 需要值"; host="$2"; shift 2 ;;
    --port) [[ $# -ge 2 ]] || die "--port 需要值"; port="$2"; shift 2 ;;
    --help) usage; exit 0 ;;
    *) die "未知选项：$1" ;;
  esac
done
args=(run dev -- --host "$host")
[[ -n "$port" ]] && args+=(--port "$port")
run_mobile_npm "${args[@]}"
