// Package version 提供构建版本信息，供 CLI 标志、日志和后续协议握手使用。
package version

// Version 是当前开发版本。发布流程确定后改为 ldflags 注入，当前先固定。
const Version = "0.1.0-dev"
