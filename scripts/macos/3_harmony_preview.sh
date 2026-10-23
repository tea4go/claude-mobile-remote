#!/usr/bin/env bash
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1
source "$(cd "$(dirname "$0")" && pwd)/_harmony.sh"
if [[ "${1:-}" == --help ]]; then
  printf '%s\n' '用法：bash scripts/macos/3_harmony_preview.sh [--build-only]' '使用隔离工程编译 ArkUI 模拟数据界面并打开本机预览；不需要签名或模拟器。' '修改后重新运行；Ctrl+C停止。预览不能替代真机原生能力验收。'
  exit 0
fi
[[ $# -eq 0 || ( $# -eq 1 && "$1" == --build-only ) ]] || die "未知参数：$*"
harmony_environment
export HARMONY_HVIGORW
project="$(python3 "$SCRIPT_DIR/harmony_preview.py" prepare "$REPO_ROOT")"
cd "$project"
"$HARMONY_OHPM" install --all
"$HARMONY_HVIGORW" --mode module -p product=default -p module=entry@default -p buildMode=debug assembleHap --no-daemon
python3 "$SCRIPT_DIR/harmony_preview.py" configure "$project"
python3 "$SCRIPT_DIR/harmony_preview.py" compile "$project"
[[ -s "$project/entry/.preview/default/intermediates/assets/default/ets/modules.abc" ]] || die '未生成预览字节码'
[[ "${1:-}" == --build-only ]] && exit 0
exec python3 "$SCRIPT_DIR/harmony_preview.py" serve "$project" --tools "$HARMONY_TOOLS_HOME"
