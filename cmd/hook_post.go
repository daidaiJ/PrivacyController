package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/privatebox/pvctl/config"
	"github.com/privatebox/pvctl/replacer"
)

type HookPostCmd struct {
	configPath string
}

func NewHookPostCmd() *HookPostCmd {
	return &HookPostCmd{}
}

func (c *HookPostCmd) Name() string {
	return "hook-post"
}

func (c *HookPostCmd) Description() string {
	return "PostToolUse hook 命令，从 stdin 读取 hook JSON，替换 tool_response 中的隐私数据"
}

func (c *HookPostCmd) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.configPath, "config", "", "配置文件路径（默认查找 .privaterc.toml）")
}

func (c *HookPostCmd) Run(args []string) error {
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

	engine := replacer.New(cfg)
	defer engine.Close()

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("读取 stdin 失败: %w", err)
	}

	var input struct {
		ToolName     string          `json:"tool_name"`
		ToolResponse json.RawMessage `json:"tool_response"`
	}
	if err := json.Unmarshal(data, &input); err != nil {
		// 非 JSON 输入，直接透传
		fmt.Print(string(data))
		return nil
	}

	original := extractResponseText(input.ToolResponse)
	if original == "" {
		fmt.Print(`{"decision":"allow"}`)
		return nil
	}

	sanitized := engine.Replace(original, "", int64(len(original)))
	if sanitized == original {
		fmt.Print(`{"decision":"allow"}`)
		return nil
	}

	resp := map[string]interface{}{
		"decision": "allow",
		"hookSpecificOutput": map[string]interface{}{
			"hookEventName":     "PostToolUse",
			"additionalContext": fmt.Sprintf("[pvctl 隐私替换结果]\n%s", sanitized),
		},
	}
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(resp)
}
