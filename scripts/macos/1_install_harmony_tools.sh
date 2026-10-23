#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/_harmony.sh"
usage() { cat <<'EOF'
用法：bash scripts/macos/1_install_harmony_tools.sh --archive PATH --sha256 HASH
从华为官网下载 macOS ARM64 Command Line Tools 6.1.1，复制页面 SHA-256。
  https://developer.huawei.com/consumer/cn/download/command-line-tools-for-hmos
默认解压至项目 .local-tools/harmony/6.1.1；可用 HARMONY_TOOLS_HOME 指定空目录。
不安装 IDE 或模拟器，不修改 shell 配置，不覆盖已有安装。
EOF
}
archive=""; checksum=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --archive) [[ $# -ge 2 ]] || die "--archive 缺少值"; archive="$2"; shift 2 ;;
    --sha256) [[ $# -ge 2 ]] || die "--sha256 缺少值"; checksum="$2"; shift 2 ;;
    --help) usage; exit 0 ;;
    *) usage >&2; die "未知选项：$1" ;;
  esac
done
harmony_require_platform
[[ -f "$archive" && -n "$checksum" ]] || { usage >&2; die "需要 ZIP 文件和官方 SHA-256。"; }
[[ ! -e "$HARMONY_TOOLS_HOME" ]] || die "安装目标已存在，请先用 2_harmony_check.sh 检查：$HARMONY_TOOLS_HOME"
python3 "$SCRIPT_DIR/harmony_tools.py" archive "$archive" "$checksum"
mkdir -p "$(dirname "$HARMONY_TOOLS_HOME")"
stage="$(mktemp -d "$(dirname "$HARMONY_TOOLS_HOME")/.extract.XXXXXX")"
trap 'rm -rf "$stage"' EXIT
unzip -q "$archive" -d "$stage"
tool_root="$stage/command-line-tools"
[[ -d "$tool_root" ]] || die "ZIP 中没有 command-line-tools 根目录，拒绝安装。"
python3 "$SCRIPT_DIR/harmony_tools.py" sdk "$tool_root/sdk"
mv "$tool_root" "$HARMONY_TOOLS_HOME"
log_info "工具已解压：$HARMONY_TOOLS_HOME"
du -sh "$HARMONY_TOOLS_HOME"
bash "$SCRIPT_DIR/2_harmony_check.sh"
