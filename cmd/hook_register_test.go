package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- mergeHooksIntoSettings / removePvctlHooks 单元测试 ---

func TestMergeHooksIntoSettings_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	hooks := map[string]interface{}{
		"PostToolUse": []interface{}{
			map[string]interface{}{
				"matcher": "Bash|Read",
				"hooks": []interface{}{
					map[string]interface{}{"type": "command", "name": pvctlHookTag},
				},
			},
		},
	}

	if err := mergeHooksIntoSettings(path, hooks); err != nil {
		t.Fatalf("mergeHooksIntoSettings: %v", err)
	}

	settings := readSettings(t, path)
	postHooks := settings["hooks"].(map[string]interface{})
	list := postHooks["PostToolUse"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("PostToolUse count = %d, want 1", len(list))
	}
}

func TestMergeHooksIntoSettings_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	writeSettings(t, path, map[string]interface{}{
		"permissions": map[string]interface{}{"allow": []string{"Bash"}},
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Write",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "name": "other-hook"}},
				},
			},
			"SessionStart": []interface{}{
				map[string]interface{}{"hooks": []interface{}{map[string]interface{}{"type": "command"}}},
			},
		},
	})

	hooks := map[string]interface{}{
		"PostToolUse": []interface{}{
			map[string]interface{}{
				"matcher": "Bash",
				"hooks":   []interface{}{map[string]interface{}{"type": "command", "name": pvctlHookTag}},
			},
		},
	}

	mergeHooksIntoSettings(path, hooks)

	settings := readSettings(t, path)
	if settings["permissions"] == nil {
		t.Error("permissions lost after merge")
	}

	postList := settings["hooks"].(map[string]interface{})["PostToolUse"].([]interface{})
	if len(postList) != 2 {
		t.Errorf("PostToolUse count = %d, want 2", len(postList))
	}

	if settings["hooks"].(map[string]interface{})["SessionStart"] == nil {
		t.Error("SessionStart lost after merge")
	}
}

func TestMergeHooksIntoSettings_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	hooks := map[string]interface{}{
		"PostToolUse": []interface{}{
			map[string]interface{}{
				"matcher": "Bash",
				"hooks":   []interface{}{map[string]interface{}{"type": "command", "name": pvctlHookTag}},
			},
		},
	}

	mergeHooksIntoSettings(path, hooks)
	mergeHooksIntoSettings(path, hooks)

	settings := readSettings(t, path)
	postList := settings["hooks"].(map[string]interface{})["PostToolUse"].([]interface{})
	if len(postList) != 1 {
		t.Errorf("idempotent: PostToolUse count = %d, want 1", len(postList))
	}
}

func TestRemovePvctlHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	writeSettings(t, path, map[string]interface{}{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "name": pvctlHookTag, "command": "pvctl hook-post"}},
				},
				map[string]interface{}{
					"matcher": "Write",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "name": "other-hook"}},
				},
			},
			"SessionStart": []interface{}{
				map[string]interface{}{"hooks": []interface{}{map[string]interface{}{"type": "command"}}},
			},
		},
	})

	removePvctlHooks(path)

	settings := readSettings(t, path)
	hooks := settings["hooks"].(map[string]interface{})

	postList := hooks["PostToolUse"].([]interface{})
	if len(postList) != 1 {
		t.Errorf("PostToolUse count = %d, want 1", len(postList))
	}
	first := postList[0].(map[string]interface{})
	name := first["hooks"].([]interface{})[0].(map[string]interface{})["name"]
	if name != "other-hook" {
		t.Errorf("remaining hook = %q, want %q", name, "other-hook")
	}

	if hooks["SessionStart"] == nil {
		t.Error("SessionStart lost after remove")
	}
}

func TestRemovePvctlHooks_AllPvctl(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	writeSettings(t, path, map[string]interface{}{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "name": pvctlHookTag}},
				},
			},
		},
	})

	removePvctlHooks(path)

	settings := readSettings(t, path)
	if settings["hooks"] != nil {
		t.Error("hooks key should be removed when all entries are pvctl")
	}
}

func TestRemovePvctlHooks_NonExistentFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	if err := removePvctlHooks(path); err != nil {
		t.Fatalf("removePvctlHooks on nonexistent file: %v", err)
	}
}

// --- buildHooks 测试 ---

func TestBuildHooks_CLIMode(t *testing.T) {
	hooks := buildHooks("/usr/bin/pvctl", "cli", 9876, "claude")

	postList := hooks["PostToolUse"].([]interface{})
	entry := postList[0].(map[string]interface{})
	hook := entry["hooks"].([]interface{})[0].(map[string]interface{})

	if hook["type"] != "command" {
		t.Errorf("type = %q, want command", hook["type"])
	}
	if hook["name"] != pvctlHookTag {
		t.Errorf("name = %q, want %q", hook["name"], pvctlHookTag)
	}
}

func TestBuildHooks_HTTPMode(t *testing.T) {
	hooks := buildHooks("/usr/bin/pvctl", "http", 8080, "qwen")

	postList := hooks["PostToolUse"].([]interface{})
	hook := postList[0].(map[string]interface{})["hooks"].([]interface{})[0].(map[string]interface{})

	if hook["type"] != "http" {
		t.Errorf("type = %q, want http", hook["type"])
	}
	if hook["url"] != "http://127.0.0.1:8080/hook" {
		t.Errorf("url = %q, want http://127.0.0.1:8080/hook", hook["url"])
	}
}

// --- installHooks 端到端测试（使用临时路径） ---

func TestInstallHooks_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// 安装 cli 模式 hook
	if err := installHooks(settingsPath, "/test/pvctl", "cli", 9876, "claude"); err != nil {
		t.Fatalf("installHooks: %v", err)
	}

	settings := readSettings(t, settingsPath)
	postList := settings["hooks"].(map[string]interface{})["PostToolUse"].([]interface{})
	if len(postList) != 1 {
		t.Fatalf("PostToolUse count = %d, want 1", len(postList))
	}

	hook := postList[0].(map[string]interface{})["hooks"].([]interface{})[0].(map[string]interface{})
	if hook["type"] != "command" {
		t.Errorf("type = %q, want command", hook["type"])
	}
	if hook["name"] != pvctlHookTag {
		t.Errorf("name = %q, want %q", hook["name"], pvctlHookTag)
	}
}

func TestInstallAndUninstall_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// 先安装
	installHooks(settingsPath, "/test/pvctl", "cli", 9876, "qwen")

	// 再添加其他 hook
	settings := readSettings(t, settingsPath)
	settings["hooks"].(map[string]interface{})["SessionStart"] = []interface{}{
		map[string]interface{}{"hooks": []interface{}{map[string]interface{}{"type": "command", "command": "echo start"}}},
	}
	writeSettings(t, settingsPath, settings)

	// 卸载 pvctl hook
	removePvctlHooks(settingsPath)

	settings = readSettings(t, settingsPath)
	hooks := settings["hooks"].(map[string]interface{})

	if hooks["PostToolUse"] != nil {
		t.Error("PostToolUse should be removed")
	}
	if hooks["SessionStart"] == nil {
		t.Error("SessionStart should be preserved")
	}
}

func TestInstallWithCustomPath(t *testing.T) {
	dir := t.TempDir()
	customPath := filepath.Join(dir, "custom", "deep", "settings.json")

	installHooks(customPath, "/test/pvctl", "http", 8080, "claude")

	settings := readSettings(t, customPath)
	postList := settings["hooks"].(map[string]interface{})["PostToolUse"].([]interface{})
	hook := postList[0].(map[string]interface{})["hooks"].([]interface{})[0].(map[string]interface{})

	if hook["type"] != "http" {
		t.Errorf("type = %q, want http", hook["type"])
	}
	if hook["url"] != "http://127.0.0.1:8080/hook" {
		t.Errorf("url = %q, want http://127.0.0.1:8080/hook", hook["url"])
	}
}

// --- resolvePath 测试 ---

func TestResolvePath(t *testing.T) {
	if got := resolvePath("explicit", "fallback"); got != "explicit" {
		t.Errorf("resolvePath(explicit, fallback) = %q, want explicit", got)
	}
	if got := resolvePath("", "fallback"); got != "fallback" {
		t.Errorf("resolvePath('', fallback) = %q, want fallback", got)
	}
}

// --- 工具函数 ---

func readSettings(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readSettings(%s): %v", path, err)
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("unmarshal(%s): %v", path, err)
	}
	return settings
}

func writeSettings(t *testing.T, path string, v interface{}) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatalf("writeSettings(%s): %v", path, err)
	}
}
