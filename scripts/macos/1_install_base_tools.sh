#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

usage() { cat <<'EOF'
用法：bash scripts/macos/1_install_base_tools.sh
  安装/检查 Go、Node.js/npm，并准备项目 Go 与前端依赖。
EOF
}
[[ "${1:-}" == "--help" ]] && { usage; exit 0; }
[[ $# -eq 0 ]] || { usage >&2; die "未知选项：$1"; }

if ! command -v brew >/dev/null 2>&1; then
  die "缺少 Homebrew。请先从 https://brew.sh 安装 Homebrew。"
fi
eval "$(brew shellenv)"
command -v go >/dev/null 2>&1 || brew install go
command -v node >/dev/null 2>&1 || brew install node
command -v npm >/dev/null 2>&1 || die "Node.js 安装后仍找不到 npm，请重新打开终端。"
ensure_go_module
(cd "$REPO_ROOT" && go mod tidy)
run_mobile_npm ci
log_info "基础工具和项目依赖安装完成。"
