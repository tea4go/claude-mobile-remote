#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "$0")" && pwd)/_common.sh"
[[ $# -eq 1 ]] || die "用法：bash scripts/macos/ios_import_jpush.sh <解压后的 jpush-ios-6.2.2-release 目录>"
source_dir="$1/Lib/JPushSDKLib-noidfa"
target_dir="$MOBILE_DIR/ios/App/JPushSDK/Binaries"
for item in jpush-ios-6.2.2.xcframework jcore-noidfa-ios-5.5.1.xcframework; do
  [[ -d "$source_dir/$item" ]] || die "缺少 $source_dir/$item"
done
mkdir -p "$target_dir"
for item in jpush-ios-6.2.2.xcframework jcore-noidfa-ios-5.5.1.xcframework; do
  ditto "$source_dir/$item" "$target_dir/$item"
done
for sdk in JPush JCore; do
  if [[ "$sdk" == JPush ]]; then item=jpush-ios-6.2.2.xcframework; else item=jcore-noidfa-ios-5.5.1.xcframework; fi
  mkdir -p "$target_dir/${sdk}Privacy.bundle"
  cp "$target_dir/$item/ios-arm64/PrivacyInfo.xcprivacy" "$target_dir/${sdk}Privacy.bundle/PrivacyInfo.xcprivacy"
done
log_info "已导入 JPush 6.2.2 / JCore noidfa 5.5.1（本机依赖，不纳入 Git）。"
