#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"
usage() { cat <<'EOF'
用法：bash scripts/macos/web-build.sh [--skip-test] [--skip-lint]
EOF
}
skip_test=0 skip_lint=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-test) skip_test=1; shift ;;
    --skip-lint) skip_lint=1; shift ;;
    --help) usage; exit 0 ;;
    *) usage >&2; die "未知选项：$1" ;;
  esac
done
[[ "$skip_test" -eq 1 ]] || run_mobile_npm run test
[[ "$skip_lint" -eq 1 ]] || run_mobile_npm run lint
run_mobile_npm run build
