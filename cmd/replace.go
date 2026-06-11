package cmd

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/privatebox/pvctl/config"
	"github.com/privatebox/pvctl/replacer"
)

type ReplaceCmd struct {
	configPath string
	verbose    bool
}

func NewReplaceCmd() *ReplaceCmd {
	return &ReplaceCmd{}
}

func (c *ReplaceCmd) Name() string {
	return "replace"
}

func (c *ReplaceCmd) Description() string {
	return "从 stdin 读取内容，替换隐私数据后输出到 stdout"
}

func (c *ReplaceCmd) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.configPath, "config", "", "配置文件路径（默认查找 .privaterc.toml）")
	fs.BoolVar(&c.verbose, "verbose", false, "输出诊断信息到 stderr")
}

func (c *ReplaceCmd) Run(args []string) error {
	// 1. 加载配置
	var cfg *config.Config
	var err error

	if c.configPath != "" {
		cfg, err = config.Load(c.configPath)
	} else {
		cfg, _, err = config.LoadDefault()
	}
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 2. 创建替换引擎
	engine := replacer.New(cfg)

	// 3. 从 stdin 读取全部内容
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("读取 stdin 失败: %w", err)
	}

	// 4. 替换
	input := string(data)
	output := engine.Replace(input, "", int64(len(data)))

	// 5. 输出到 stdout
	fmt.Print(output)

	return nil
}
