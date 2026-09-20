#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"
if [[ "${1:-}" == "--help" ]]; then printf '%s\n' '用法：bash scripts/macos/3_cli_run.sh [CLI 参数...]' 'CLI 参数会原样传递给 claude-mobile-remote。'; exit 0; fi
ensure_go_module
(cd "$REPO_ROOT" && go run ./cmd/claude-mobile-remote "$@")
