#!/usr/bin/env bash
# Sourced by the Harmony CLI scripts; environment changes affect this process only.
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"
HARMONY_DIR="$REPO_ROOT/apps/harmony"
HARMONY_TOOLS_HOME="${HARMONY_TOOLS_HOME:-$REPO_ROOT/.local-tools/harmony/6.1.1}"

harmony_require_platform() {
  [[ "$(uname -s)" == Darwin && "$(uname -m)" == arm64 ]] || die "此入口仅支持 Apple Silicon macOS。"
  need_cmd python3
}

harmony_find_tool() {
  local name="$1" candidate
  for candidate in "$HARMONY_TOOLS_HOME/bin/$name" "$HARMONY_TOOLS_HOME/$name/bin/$name" "$HARMONY_TOOLS_HOME/tools/$name/bin/$name"; do
    if [[ -x "$candidate" ]]; then printf '%s\n' "$candidate"; return; fi
  done
  die "工具链内缺少 $name：$HARMONY_TOOLS_HOME"
}

harmony_environment() {
  harmony_require_platform
  [[ -d "$HARMONY_TOOLS_HOME" ]] || die "未安装工具链。先运行 1_install_harmony_tools.sh --help。"
  HARMONY_HVIGORW="$(harmony_find_tool hvigorw)"
  HARMONY_OHPM="$(harmony_find_tool ohpm)"
  local candidate node_bin=""
  for candidate in "$HARMONY_TOOLS_HOME/tool/node/bin" "$HARMONY_TOOLS_HOME/node/bin" "$HARMONY_TOOLS_HOME/node" "$HARMONY_TOOLS_HOME/tools/node/bin" "$HARMONY_TOOLS_HOME/tools/node"; do
    if [[ -x "$candidate/node" ]]; then node_bin="$candidate"; break; fi
  done
  [[ -n "$node_bin" ]] || die "工具链缺少配套 Node.js，不能使用系统 Node 替代。"
  [[ "$(file "$node_bin/node")" == *arm64* ]] || die "工具链 Node 不是 Mac ARM64 二进制。"
  export PATH="$node_bin:$(dirname "$HARMONY_HVIGORW"):$(dirname "$HARMONY_OHPM"):$PATH"
  export DEVECO_NODE_HOME="$(dirname "$node_bin")"
  export NODE_HOME="$DEVECO_NODE_HOME"
  export DEVECO_SDK_HOME="$HARMONY_TOOLS_HOME/sdk"
  [[ -d "$DEVECO_SDK_HOME" ]] || die "缺少配套 SDK：$DEVECO_SDK_HOME"
  # Confirm the SDK API from its own metadata, not the ZIP filename.
  python3 "$SCRIPT_DIR/harmony_tools.py" sdk "$DEVECO_SDK_HOME" || die "SDK 不匹配项目 6.1.1(24)。"
}
