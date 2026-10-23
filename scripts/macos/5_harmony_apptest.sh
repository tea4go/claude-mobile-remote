#!/usr/bin/env bash
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1
source "$(cd "$(dirname "$0")" && pwd)/_harmony.sh"
if [[ "${1:-}" == --help ]]; then
  printf '%s\n' '用法：bash scripts/macos/5_harmony_apptest.sh --unsigned' '      bash scripts/macos/5_harmony_apptest.sh --config /仓库外/signing.json' '在隔离工程构建 Release APP；--unsigned 仅验证编译，不能上传分发。' '      bash scripts/macos/5_harmony_apptest.sh --init-signing /仓库外/新目录' '配置格式和证书申请见 docs/harmony-preview-apptest.md。需要 JAVA_HOME。'
  exit 0
fi
[[ ( $# -eq 1 && "$1" == --unsigned ) || ( $# -eq 2 && ( "$1" == --config || "$1" == --init-signing ) ) ]] || die '请使用 --unsigned、--config 或 --init-signing，详见 --help'
harmony_environment
export HARMONY_OHPM HARMONY_HVIGORW HARMONY_TOOLS_HOME
if [[ -z "${JAVA_HOME:-}" && -x "$HOME/jdk/bin/java" ]]; then export JAVA_HOME="$HOME/jdk"; fi
[[ -x "${JAVA_HOME:-}/bin/java" ]] || die '请设置 JAVA_HOME 指向现有 JDK'
exec python3 "$SCRIPT_DIR/harmony_apptest.py" "$REPO_ROOT" "$@"
