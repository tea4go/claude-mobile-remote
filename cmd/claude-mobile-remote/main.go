// claude-mobile-remote 在 PC 上托管本地 Claude 会话，并通过 WebSocket
// 向连接 VPN 的手机提供会话接力（A1 仅提供骨架入口）。
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/tea4go/claude-mobile-remote/internal/version"
)

func main() {
	showVersion := flag.Bool("version", false, "打印版本并退出")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Version)
		return
	}

	fmt.Printf("claude-mobile-remote %s — A1 skeleton\n", version.Version)
	fmt.Println("会话托管与手机接力将在后续子阶段实现（见 docs/plans/）。")
	os.Exit(0)
}
