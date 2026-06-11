package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/privatebox/pvctl/cmd"
)

var commands = map[string]struct {
	cmd  interface {
		SetFlags(*flag.FlagSet)
		Run([]string) error
		Description() string
	}
	desc string
}{
	"replace":   {cmd: cmd.NewReplaceCmd(), desc: "从 stdin 读取内容，替换隐私数据后输出到 stdout"},
	"serve":     {cmd: cmd.NewServeCmd(), desc: "启动 HTTP 服务，作为 hook 端点"},
	"hook-post": {cmd: cmd.NewHookPostCmd(), desc: "PostToolUse hook 命令（CLI 管道模式）"},
	"hook":      {cmd: cmd.NewHookCmd(), desc: "为 Claude Code / Qwen Code 注册 hook"},
	"hook-test": {cmd: cmd.NewHookTestCmd(), desc: "发送模拟 hook 请求，验证隐私替换效果"},
	"version":   {cmd: cmd.NewVersionCmd(), desc: "显示版本信息"},
}

var commandOrder = []string{"replace", "serve", "hook-post", "hook", "hook-test", "version"}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]

	if subcommand == "help" || subcommand == "-h" || subcommand == "--help" {
		printUsage()
		return
	}

	entry, ok := commands[subcommand]
	if !ok {
		fmt.Fprintf(os.Stderr, "未知命令: %s\n\n", subcommand)
		printUsage()
		os.Exit(1)
	}

	fs := flag.NewFlagSet(subcommand, flag.ExitOnError)
	entry.cmd.SetFlags(fs)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "用法: pvctl %s [选项]\n\n", subcommand)
		fmt.Fprintf(os.Stderr, "%s\n\n", entry.cmd.Description())
		fmt.Fprintf(os.Stderr, "选项:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[2:]); err != nil {
		os.Exit(1)
	}
	if err := entry.cmd.Run(fs.Args()); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	maxLen := 0
	for _, name := range commandOrder {
		if len(name) > maxLen {
			maxLen = len(name)
		}
	}

	fmt.Fprintf(os.Stderr, `Privacy Vault CLI (pvctl) — 隐私数据替换工具

用法:
    pvctl <命令> [选项]

命令:
`)
	for _, name := range commandOrder {
		if entry, ok := commands[name]; ok {
			fmt.Fprintf(os.Stderr, "    %-*s    %s\n", maxLen, name, entry.desc)
		}
	}
	fmt.Fprintf(os.Stderr, `
示例:
    cat config.yaml | pvctl replace
    pvctl serve --port 9876
    pvctl hook install --claude --qwen
    pvctl hook-test --mode http
    pvctl hook-test --mode cli

配置文件:
    默认查找 .privaterc.toml, .pvctl.toml, pvctl.toml
    可通过 --config 指定其他路径
`)
}
