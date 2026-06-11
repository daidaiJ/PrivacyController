package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type HookCmd struct{}

func NewHookCmd() *HookCmd {
	return &HookCmd{}
}

func (c *HookCmd) Name() string {
	return "hook"
}

func (c *HookCmd) Description() string {
	return "为 Claude Code / Qwen Code 注册 pvctl 隐私替换 hook"
}

func (c *HookCmd) SetFlags(fs *flag.FlagSet) {}

func (c *HookCmd) Run(args []string) error {
	if len(args) == 0 {
		printHookUsage()
		return nil
	}

	switch args[0] {
	case "install":
		return hookInstall(args[1:])
	case "uninstall":
		return hookUninstall(args[1:])
	case "show":
		return hookShow(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n\n", args[0])
		printHookUsage()
		return nil
	}
}

func printHookUsage() {
	fmt.Fprintf(os.Stderr, `pvctl hook — AI 编程助手 hook 注册管理

用法:
    pvctl hook install   [--claude] [--qwen] [--mode http|cli] [--port N]
    pvctl hook uninstall [--claude] [--qwen]
    pvctl hook show      [--claude] [--qwen]

选项:
    --claude              操作 Claude Code 配置
    --qwen                操作 Qwen Code 配置
    --mode  http|cli      hook 模式（默认 cli）
    --port  N             HTTP 模式端口（默认 9876）
    --claude-settings P   Claude Code settings.json 路径（默认 ~/.claude/settings.json）
    --qwen-settings   P   Qwen Code settings.json 路径（默认 ~/.qwen/settings.json）

    不指定 --claude/--qwen 则两者都操作。
`)
}

// --- install ---

func hookInstall(args []string) error {
	fs := flag.NewFlagSet("hook install", flag.ExitOnError)
	claude := fs.Bool("claude", false, "注册到 Claude Code")
	qwen := fs.Bool("qwen", false, "注册到 Qwen Code")
	mode := fs.String("mode", "cli", "hook 模式: http 或 cli")
	port := fs.Int("port", 9876, "HTTP 模式端口")
	claudePath := fs.String("claude-settings", "", "Claude Code settings.json 路径")
	qwenPath := fs.String("qwen-settings", "", "Qwen Code settings.json 路径")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if !*claude && !*qwen {
		*claude = true
		*qwen = true
	}

	pvctlPath := findPvctlPath()
	var errs []string

	if *claude {
		p := resolvePath(*claudePath, getClaudeSettingsPath())
		if err := installHooks(p, pvctlPath, *mode, *port, "claude"); err != nil {
			errs = append(errs, fmt.Sprintf("Claude Code (%s): %v", p, err))
		} else {
			fmt.Fprintf(os.Stderr, "✓ Claude Code hook 已注册 → %s\n", p)
		}
	}

	if *qwen {
		p := resolvePath(*qwenPath, getQwenSettingsPath())
		if err := installHooks(p, pvctlPath, *mode, *port, "qwen"); err != nil {
			errs = append(errs, fmt.Sprintf("Qwen Code (%s): %v", p, err))
		} else {
			fmt.Fprintf(os.Stderr, "✓ Qwen Code hook 已注册 → %s\n", p)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("部分注册失败:\n%s", strings.Join(errs, "\n"))
	}
	return nil
}

// --- uninstall ---

func hookUninstall(args []string) error {
	fs := flag.NewFlagSet("hook uninstall", flag.ExitOnError)
	claude := fs.Bool("claude", false, "从 Claude Code 移除")
	qwen := fs.Bool("qwen", false, "从 Qwen Code 移除")
	claudePath := fs.String("claude-settings", "", "Claude Code settings.json 路径")
	qwenPath := fs.String("qwen-settings", "", "Qwen Code settings.json 路径")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if !*claude && !*qwen {
		*claude = true
		*qwen = true
	}

	var errs []string

	if *claude {
		p := resolvePath(*claudePath, getClaudeSettingsPath())
		if err := removePvctlHooks(p); err != nil {
			errs = append(errs, fmt.Sprintf("Claude Code (%s): %v", p, err))
		} else {
			fmt.Fprintf(os.Stderr, "✓ Claude Code hook 已移除 → %s\n", p)
		}
	}

	if *qwen {
		p := resolvePath(*qwenPath, getQwenSettingsPath())
		if err := removePvctlHooks(p); err != nil {
			errs = append(errs, fmt.Sprintf("Qwen Code (%s): %v", p, err))
		} else {
			fmt.Fprintf(os.Stderr, "✓ Qwen Code hook 已移除 → %s\n", p)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("部分移除失败:\n%s", strings.Join(errs, "\n"))
	}
	return nil
}

// --- show ---

func hookShow(args []string) error {
	fs := flag.NewFlagSet("hook show", flag.ExitOnError)
	claude := fs.Bool("claude", false, "显示 Claude Code 配置")
	qwen := fs.Bool("qwen", false, "显示 Qwen Code 配置")
	claudePath := fs.String("claude-settings", "", "Claude Code settings.json 路径")
	qwenPath := fs.String("qwen-settings", "", "Qwen Code settings.json 路径")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if !*claude && !*qwen {
		*claude = true
		*qwen = true
	}

	if *claude {
		p := resolvePath(*claudePath, getClaudeSettingsPath())
		fmt.Fprintf(os.Stderr, "=== Claude Code (%s) ===\n", p)
		showSettings(p)
	}
	if *qwen {
		p := resolvePath(*qwenPath, getQwenSettingsPath())
		fmt.Fprintf(os.Stderr, "\n=== Qwen Code (%s) ===\n", p)
		showSettings(p)
	}
	return nil
}

// --- 核心逻辑 ---

const pvctlHookTag = "pvctl-privacy-replace"

func installHooks(settingsPath, pvctlPath, mode string, port int, target string) error {
	hooks := buildHooks(pvctlPath, mode, port, target)
	return mergeHooksIntoSettings(settingsPath, hooks)
}

func buildHooks(pvctlPath, mode string, port int, target string) map[string]interface{} {
	var hookEntry map[string]interface{}

	if mode == "http" {
		hookEntry = map[string]interface{}{
			"type":          "http",
			"url":           fmt.Sprintf("http://127.0.0.1:%d/hook", port),
			"name":          pvctlHookTag,
			"timeout":       10,
			"statusMessage": "隐私数据替换中...",
		}
	} else {
		command := fmt.Sprintf(`%s hook-post`, quotePath(pvctlPath))
		hookEntry = map[string]interface{}{
			"type":          "command",
			"command":       command,
			"name":          pvctlHookTag,
			"timeout":       30,
			"statusMessage": "隐私数据替换中...",
		}
	}

	matcher := "Bash|Read|ReadFile|Grep|Glob|Write|Edit"
	if target == "qwen" {
		matcher = "Bash|ReadFile|Grep|Glob|WriteFile|Edit"
	}

	return map[string]interface{}{
		"PostToolUse": []interface{}{
			map[string]interface{}{
				"matcher": matcher,
				"hooks":   []interface{}{hookEntry},
			},
		},
	}
}

// --- Settings 文件操作 ---

func mergeHooksIntoSettings(settingsPath string, newHooks map[string]interface{}) error {
	dir := filepath.Dir(settingsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录 %s 失败: %w", dir, err)
	}

	settings := make(map[string]interface{})
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		json.Unmarshal(data, &settings)
	}

	existingHooks, _ := settings["hooks"].(map[string]interface{})
	if existingHooks == nil {
		existingHooks = make(map[string]interface{})
	}

	for event, newEntries := range newHooks {
		newList, ok := newEntries.([]interface{})
		if !ok {
			continue
		}

		existingList, _ := existingHooks[event].([]interface{})
		if existingList == nil {
			existingList = []interface{}{}
		}

		// 移除已有的 pvctl hook（幂等）
		filtered := removeByTag(existingList, pvctlHookTag)
		existingHooks[event] = append(filtered, newList...)
	}

	settings["hooks"] = existingHooks
	return writeJSONFile(settingsPath, settings)
}

func removePvctlHooks(settingsPath string) error {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("解析 %s 失败: %w", settingsPath, err)
	}

	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		return nil
	}

	for event, entries := range hooks {
		list, ok := entries.([]interface{})
		if !ok {
			continue
		}
		filtered := removeByTag(list, pvctlHookTag)
		if len(filtered) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = filtered
		}
	}

	if len(hooks) == 0 {
		delete(settings, "hooks")
	}

	return writeJSONFile(settingsPath, settings)
}

// removeByTag 从 hook 列表中移除包含指定 name 标签的条目
func removeByTag(list []interface{}, tag string) []interface{} {
	filtered := make([]interface{}, 0, len(list))
	for _, entry := range list {
		m, ok := entry.(map[string]interface{})
		if !ok {
			filtered = append(filtered, entry)
			continue
		}
		if isTagged(m, tag) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func isTagged(entry map[string]interface{}, tag string) bool {
	hooks, _ := entry["hooks"].([]interface{})
	for _, h := range hooks {
		hm, ok := h.(map[string]interface{})
		if ok {
			if name, _ := hm["name"].(string); name == tag {
				return true
			}
		}
	}
	return false
}

func showSettings(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "  配置文件不存在: %s\n", path)
		} else {
			fmt.Fprintf(os.Stderr, "  读取失败: %v\n", err)
		}
		return
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		fmt.Fprintf(os.Stderr, "  解析失败: %v\n", err)
		return
	}

	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "  未配置任何 hook")
		return
	}

	pretty, _ := json.MarshalIndent(hooks, "  ", "  ")
	fmt.Fprintf(os.Stderr, "  %s\n", string(pretty))
}

// --- 路径工具 ---

func getClaudeSettingsPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("USERPROFILE"), ".claude", "settings.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

func getQwenSettingsPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("USERPROFILE"), ".qwen", "settings.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".qwen", "settings.json")
}

func resolvePath(explicit, fallback string) string {
	if explicit != "" {
		return explicit
	}
	return fallback
}

func findPvctlPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "pvctl"
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return "pvctl"
	}
	return abs
}

func quotePath(p string) string {
	if runtime.GOOS == "windows" {
		return `"` + strings.ReplaceAll(p, `"`, `\"`) + `"`
	}
	return "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
}

func writeJSONFile(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}
