#!/usr/bin/env bash
set -euo pipefail
repo_dir="$(cd "$(dirname "$0")/../.." && pwd)"
output_dir="${1:?Usage: bash scripts/macos/7_notify_server_build.sh NEW_OUTPUT_DIRECTORY}"
if [[ -e "$output_dir" ]]; then echo 'Output directory already exists' >&2; exit 1; fi
mkdir -p "$output_dir"
output_dir="$(cd "$output_dir" && pwd)"
cd "$repo_dir"
for architecture in amd64 arm64; do
  package_dir="$output_dir/linux-$architecture"
  mkdir -p "$package_dir"
  CGO_ENABLED=0 GOOS=linux GOARCH="$architecture" GOFLAGS=-buildvcs=false go build -trimpath -o "$package_dir/notify-server" ./cmd/notify-server
  cp deploy/notify/README.md deploy/notify/cmr-notify.service deploy/notify/notify.env.example deploy/notify/notifications.json.example deploy/notify/nginx-location.conf "$package_dir/"
  (cd "$package_dir" && shasum -a 256 notify-server README.md cmr-notify.service notify.env.example notifications.json.example nginx-location.conf > SHA256SUMS)
  COPYFILE_DISABLE=1 tar -czf "$output_dir/cmr-notify-linux-$architecture.tar.gz" -C "$package_dir" .
done
(cd "$output_dir" && shasum -a 256 cmr-notify-linux-*.tar.gz > SHA256SUMS)
echo "Notification packages generated: $output_dir"
