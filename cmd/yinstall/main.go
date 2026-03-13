package main

import (
	"os"

	"github.com/yinstall/internal/cli"
)

func init() {
	// 将构建时通过 -ldflags 注入的 Version 变量同步到 cli.AppVersion，
	// 使 `yinstall --version` 和日志头显示真实构建版本而非硬编码的 "0.1.0"。
	if Version != "" {
		cli.SetAppVersion(Version)
	}
}

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
