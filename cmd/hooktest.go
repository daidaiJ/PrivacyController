package cmd

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/privatebox/pvctl/config"
	"github.com/privatebox/pvctl/replacer"
)

type HookTestCmd struct {
	configPath string
	mode       string
	port       int
	text       string
	toolName   string
}

func NewHookTestCmd() *HookTestCmd {
	return &HookTestCmd{}
}

func (c *HookTestCmd) Name() string {
	return "hook-test"
}

func (c *HookTestCmd) Description() string {
	return "发送模拟 PostToolUse hook 请求，验证隐私替换效果"
}

func (c *HookTestCmd) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.configPath, "config", "", "配置文件路径")
	fs.StringVar(&c.mode, "mode", "cli", "测试模式: http / cli / direct")
	fs.IntVar(&c.port, "port", 9876, "HTTP 模式端口")
	fs.StringVar(&c.text, "text", "张三的手机号是13800138000，邮箱zhangsan@example.com", "测试文本（含敏感数据）")
	fs.StringVar(&c.toolName, "tool", "Bash", "模拟的工具名")
}

func (c *HookTestCmd) Run(args []string) error {
	// 构造模拟的 PostToolUse hook 输入
	hookInput := map[string]interface{}{
		"session_id":       "test-session-001",
		"hook_event_name":  "PostToolUse",
		"tool_name":        c.toolName,
		"tool_input":       map[string]interface{}{"command": "echo test"},
		"tool_response":    c.text,
		"permission_mode":  "yolo",
	}

	inputJSON, err := json.Marshal(hookInput)
	if err != nil {
		return fmt.Errorf("序列化 hook 输入失败: %w", err)
	}

	fmt.Fprintf(os.Stderr, "=== pvctl hook-test ===\n")
	fmt.Fprintf(os.Stderr, "模式: %s\n", c.mode)
	fmt.Fprintf(os.Stderr, "测试文本: %s\n", c.text)
	fmt.Fprintf(os.Stderr, "--- 输入 JSON ---\n")
	fmt.Fprintf(os.Stderr, "%s\n", prettyJSON(inputJSON))
	fmt.Fprintf(os.Stderr, "--- 处理结果 ---\n")

	var output []byte

	switch c.mode {
	case "http":
		output, err = c.testHTTP(inputJSON)
	case "cli":
		output, err = c.testCLI(inputJSON)
	case "direct":
		output, err = c.testDirect(inputJSON)
	default:
		return fmt.Errorf("未知模式: %s（支持 http / cli / direct）", c.mode)
	}

	if err != nil {
		return fmt.Errorf("测试失败: %w", err)
	}

	fmt.Fprintf(os.Stderr, "%s\n", prettyJSON(output))

	// 解析并展示关键信息
	var result map[string]interface{}
	if json.Unmarshal(output, &result) == nil {
		if hso, ok := result["hookSpecificOutput"].(map[string]interface{}); ok {
			if ctx, ok := hso["additionalContext"].(string); ok {
				fmt.Fprintf(os.Stderr, "\n=== 替换后的文本 ===\n%s\n", ctx)
			}
		}
	}

	return nil
}

// testHTTP 发送到 HTTP 服务
func (c *HookTestCmd) testHTTP(input []byte) ([]byte, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/hook", c.port)
	resp, err := http.Post(url, "application/json", bytes.NewReader(input))
	if err != nil {
		return nil, fmt.Errorf("请求 %s 失败: %w\n提示: 请先运行 pvctl serve --port %d", url, err, c.port)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// testCLI 通过 pvctl hook-post 命令
func (c *HookTestCmd) testCLI(input []byte) ([]byte, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("获取可执行路径失败: %w", err)
	}

	args := []string{"hook-post"}
	if c.configPath != "" {
		args = append(args, "--config", c.configPath)
	}

	cmd := exec.Command(exePath, args...)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行 pvctl hook-post 失败: %w", err)
	}
	return output, nil
}

// testDirect 直接调用替换引擎（不走子进程/HTTP）
func (c *HookTestCmd) testDirect(input []byte) ([]byte, error) {
	var cfg *config.Config
	var err error

	if c.configPath != "" {
		cfg, err = config.Load(c.configPath)
	} else {
		cfg, _, err = config.LoadDefault()
	}
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}

	engine := replacer.New(cfg)
	defer engine.Close()

	sanitized := engine.Replace(c.text, "", int64(len(c.text)))

	result := map[string]interface{}{
		"decision": "allow",
	}
	if sanitized != c.text {
		result["hookSpecificOutput"] = map[string]interface{}{
			"hookEventName":     "PostToolUse",
			"additionalContext": fmt.Sprintf("[pvctl 隐私替换结果]\n%s", sanitized),
		}
	}

	return json.MarshalIndent(result, "", "  ")
}

func prettyJSON(data []byte) string {
	var buf bytes.Buffer
	if json.Indent(&buf, data, "", "  ") != nil {
		return string(data)
	}
	return buf.String()
}

// 工具函数：判断 slice 中是否包含某字符串
func contains(ss []string, s string) bool {
	for _, v := range ss {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}
