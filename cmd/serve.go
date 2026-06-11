package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/privatebox/pvctl/config"
	"github.com/privatebox/pvctl/replacer"
)

type ServeCmd struct {
	configPath string
	port       int
}

func NewServeCmd() *ServeCmd {
	return &ServeCmd{}
}

func (c *ServeCmd) Name() string {
	return "serve"
}

func (c *ServeCmd) Description() string {
	return "启动 HTTP 服务，作为 AI 编程助手的隐私替换 hook 端点"
}

func (c *ServeCmd) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.configPath, "config", "", "配置文件路径（默认查找 .privaterc.toml）")
	fs.IntVar(&c.port, "port", 9876, "监听端口")
}

func (c *ServeCmd) Run(args []string) error {
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

	mux := http.NewServeMux()
	mux.HandleFunc("/hook", makeHookHandler(engine))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	addr := fmt.Sprintf("127.0.0.1:%d", c.port)
	fmt.Fprintf(os.Stderr, "pvctl serve 启动，监听 %s\n", addr)
	fmt.Fprintf(os.Stderr, "  POST /hook   — PostToolUse hook 端点\n")
	fmt.Fprintf(os.Stderr, "  GET  /health — 健康检查\n")

	if err := http.ListenAndServe(addr, mux); err != nil {
		return fmt.Errorf("HTTP 服务异常: %w", err)
	}
	return nil
}

func makeHookHandler(engine *replacer.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error":"读取请求体失败"}`, http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var input struct {
			ToolName     string          `json:"tool_name"`
			ToolResponse json.RawMessage `json:"tool_response"`
		}
		if err := json.Unmarshal(body, &input); err != nil {
			http.Error(w, `{"error":"解析 JSON 失败"}`, http.StatusBadRequest)
			return
		}

		// 从 tool_response 中提取文本内容
		original := extractResponseText(input.ToolResponse)
		if original == "" {
			writeJSON(w, map[string]interface{}{
				"decision": "allow",
			})
			return
		}

		sanitized := engine.Replace(original, "", int64(len(original)))
		if sanitized == original {
			writeJSON(w, map[string]interface{}{
				"decision": "allow",
			})
			return
		}

		writeJSON(w, map[string]interface{}{
			"decision": "allow",
			"hookSpecificOutput": map[string]interface{}{
				"hookEventName":     "PostToolUse",
				"additionalContext": fmt.Sprintf("[pvctl 隐私替换结果]\n%s", sanitized),
			},
		})
	}
}

func extractResponseText(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err == nil {
		if text, ok := obj["content"].(string); ok {
			return text
		}
		if output, ok := obj["output"].(string); ok {
			return output
		}
		if result, ok := obj["result"].(string); ok {
			return result
		}
		// 嵌套 content 数组（Claude 格式）
		if contents, ok := obj["content"].([]interface{}); ok {
			var text string
			for _, c := range contents {
				if m, ok := c.(map[string]interface{}); ok {
					if t, ok := m["text"].(string); ok {
						text += t
					}
				}
			}
			if text != "" {
				return text
			}
		}
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return ""
	}
	return string(b)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
