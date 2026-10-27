#!/usr/bin/env bash
set -euo pipefail
repo_dir="$(cd "$(dirname "$0")/../.." && pwd)"
output_dir="${1:?Usage: bash scripts/macos/6_review_server_build.sh NEW_OUTPUT_DIRECTORY}"
if [[ -e "$output_dir" ]]; then
  echo 'Output directory already exists' >&2
  exit 1
fi
mkdir -p "$output_dir"
output_dir="$(cd "$output_dir" && pwd)"
cd "$repo_dir"
for architecture in amd64 arm64; do
  package_dir="$output_dir/linux-$architecture"
  mkdir -p "$package_dir"
  CGO_ENABLED=0 GOOS=linux GOARCH="$architecture" GOFLAGS=-buildvcs=false go build -trimpath -o "$package_dir/review-server" ./cmd/review-server
  cp deploy/review/README.md deploy/review/review-notes.md deploy/review/nginx-location.conf deploy/review/cmr-review.service "$package_dir/"
  (cd "$package_dir" && shasum -a 256 review-server README.md review-notes.md nginx-location.conf cmr-review.service > SHA256SUMS)
  tar -czf "$output_dir/cmr-review-linux-$architecture.tar.gz" -C "$package_dir" .
done
(cd "$output_dir" && shasum -a 256 cmr-review-linux-*.tar.gz > SHA256SUMS)
echo "Review packages generated: $output_dir"
