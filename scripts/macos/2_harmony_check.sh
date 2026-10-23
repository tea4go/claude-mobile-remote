#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/_harmony.sh"
if [[ "${1:-}" == --help ]]; then
  printf '%s\n' '用法：bash scripts/macos/2_harmony_check.sh' '检查本机 ARM64 工具链、API 24 SDK及配套 Node。可设置 HARMONY_TOOLS_HOME/JAVA_HOME。'
  exit 0
fi
[[ $# -eq 0 ]] || die "未知参数：$*"
harmony_environment
log_info "工具链：$HARMONY_TOOLS_HOME"
node --version
"$HARMONY_OHPM" --version
"$HARMONY_HVIGORW" --version
log_info "环境检查通过；尚未执行工程编译。"
