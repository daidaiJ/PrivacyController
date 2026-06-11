package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Rule 定义一条替换规则
type Rule struct {
	Match       string   `toml:"match"`
	Mode        string   `toml:"mode"`        // substring | exact
	Placeholder string   `toml:"placeholder"`
	PathInclude []string `toml:"path_include,omitempty"`
	PathExclude []string `toml:"path_exclude,omitempty"`
	MaxFileSize string   `toml:"max_file_size,omitempty"` // 如 "10MB", "1KB"
}

// Defaults 全局默认值
type Defaults struct {
	MaxFileSize string   `toml:"max_file_size,omitempty"`
	PathInclude []string `toml:"path_include,omitempty"`
	PathExclude []string `toml:"path_exclude,omitempty"`
}

// Config 顶层配置
type Config struct {
	Defaults Defaults `toml:"defaults"`
	Rules    []Rule   `toml:"rules"`
}

// Load 加载并解析 TOML 配置文件
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件 %s 失败: %w", path, err)
	}

	// 校验规则
	for i, r := range cfg.Rules {
		if r.Match == "" {
			return nil, fmt.Errorf("规则 #%d: match 不能为空", i+1)
		}
		if r.Placeholder == "" {
			return nil, fmt.Errorf("规则 #%d: placeholder 不能为空", i+1)
		}
		if r.Mode != "substring" && r.Mode != "exact" && r.Mode != "token" {
			return nil, fmt.Errorf("规则 #%d: mode 必须是 substring、exact 或 token，当前: %s", i+1, r.Mode)
		}
	}

	return &cfg, nil
}

// LoadDefault 尝试从常见位置加载配置
func LoadDefault() (*Config, string, error) {
	candidates := []string{
		".privaterc.toml",
		".pvctl.toml",
		"pvctl.toml",
	}
	for _, name := range candidates {
		abs, _ := filepath.Abs(name)
		if _, err := os.Stat(abs); err == nil {
			cfg, err := Load(abs)
			return cfg, abs, err
		}
	}
	return nil, "", fmt.Errorf("未找到配置文件（查找: %v）", candidates)
}

// GetPathInclude 获取规则生效的路径包含模式（规则优先，否则用全局默认值）
func (r *Rule) GetPathInclude(defaults []string) []string {
	if len(r.PathInclude) > 0 {
		return r.PathInclude
	}
	return defaults
}

// GetPathExclude 获取规则排除的路径模式（规则优先，否则用全局默认值）
func (r *Rule) GetPathExclude(defaults []string) []string {
	if len(r.PathExclude) > 0 {
		return r.PathExclude
	}
	return defaults
}
