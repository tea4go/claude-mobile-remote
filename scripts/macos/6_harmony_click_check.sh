#!/usr/bin/env bash
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1
source "$(cd "$(dirname "$0")" && pwd)/_harmony.sh"
exec python3 "$SCRIPT_DIR/harmony_click_check.py" --hdc "$HARMONY_TOOLS_HOME/sdk/default/openharmony/toolchains/hdc" "$@"
