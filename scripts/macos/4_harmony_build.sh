#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/_harmony.sh"
if [[ "${1:-}" == --help ]]; then
  printf '%s\n' '用法：bash scripts/macos/4_harmony_build.sh' '安装工程既有 ohpm 依赖并构建 Debug HAP；不配置签名、不安装到设备。' '可设置 HARMONY_TOOLS_HOME/JAVA_HOME。'
  exit 0
fi
[[ $# -eq 0 ]] || die "未知参数：$*"
harmony_environment
cd "$HARMONY_DIR"
"$HARMONY_OHPM" install --all
"$HARMONY_HVIGORW" --mode module -p product=default -p module=entry@default -p buildMode=debug assembleHap --no-daemon
hap="$HARMONY_DIR/entry/build/default/outputs/default/entry-default-unsigned.hap"
[[ -s "$hap" ]] || die "构建结束但未找到未签名 HAP：$hap"
unzip -tq "$hap"
python3 "$SCRIPT_DIR/harmony_tools.py" hap "$hap"
shasum -a 256 "$hap"
log_info "HAP：${hap}（未签名，仅验证编译；设备安装需要签名）"
