package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "test.toml")

	content := `
[defaults]
max_file_size = "5MB"

[[rules]]
match = "张三"
mode = "token"
placeholder = "<NAME>"

[[rules]]
match = "13800138000"
mode = "exact"
placeholder = "<PHONE>"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Defaults.MaxFileSize != "5MB" {
		t.Errorf("MaxFileSize = %q, want %q", cfg.Defaults.MaxFileSize, "5MB")
	}
	if len(cfg.Rules) != 2 {
		t.Fatalf("Rules count = %d, want 2", len(cfg.Rules))
	}
	if cfg.Rules[0].Match != "张三" {
		t.Errorf("Rule[0].Match = %q, want %q", cfg.Rules[0].Match, "张三")
	}
	if cfg.Rules[0].Mode != "token" {
		t.Errorf("Rule[0].Mode = %q, want %q", cfg.Rules[0].Mode, "token")
	}
	if cfg.Rules[1].Placeholder != "<PHONE>" {
		t.Errorf("Rule[1].Placeholder = %q, want %q", cfg.Rules[1].Placeholder, "<PHONE>")
	}
}

func TestLoadValidation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "empty match",
			content: `
[[rules]]
match = ""
mode = "exact"
placeholder = "<X>"
`,
			wantErr: "match 不能为空",
		},
		{
			name: "empty placeholder",
			content: `
[[rules]]
match = "test"
mode = "exact"
placeholder = ""
`,
			wantErr: "placeholder 不能为空",
		},
		{
			name: "invalid mode",
			content: `
[[rules]]
match = "test"
mode = "invalid"
placeholder = "<X>"
`,
			wantErr: "mode 必须是 substring、exact 或 token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "test.toml")
			if err := os.WriteFile(cfgPath, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			_, err := Load(cfgPath)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got := err.Error(); !contains(got, tt.wantErr) {
				t.Errorf("error = %q, want contains %q", got, tt.wantErr)
			}
		})
	}
}

func TestLoadDefault(t *testing.T) {
	dir := t.TempDir()

	// 在临时目录中创建 .privaterc.toml
	cfgPath := filepath.Join(dir, ".privaterc.toml")
	content := `
[[rules]]
match = "test"
mode = "exact"
placeholder = "<T>"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// 切换到临时目录
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	cfg, path, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault failed: %v", err)
	}
	if cfg.Rules[0].Match != "test" {
		t.Errorf("Match = %q, want %q", cfg.Rules[0].Match, "test")
	}
	if filepath.Base(path) != ".privaterc.toml" {
		t.Errorf("path = %q, want .privaterc.toml", path)
	}
}

func TestRulePathMethods(t *testing.T) {
	r := Rule{
		PathInclude: []string{"*.env"},
		PathExclude: []string{"*.lock"},
	}

	defaults := Defaults{
		PathInclude: []string{"*.go"},
		PathExclude: []string{"*.exe"},
	}

	// 规则有自己的值，优先使用
	if got := r.GetPathInclude(defaults.PathInclude); len(got) != 1 || got[0] != "*.env" {
		t.Errorf("GetPathInclude = %v, want [*.env]", got)
	}

	if got := r.GetPathExclude(defaults.PathExclude); len(got) != 1 || got[0] != "*.lock" {
		t.Errorf("GetPathExclude = %v, want [*.lock]", got)
	}

	// 规则没有值时，使用默认值
	r2 := Rule{}
	if got := r2.GetPathInclude(defaults.PathInclude); len(got) != 1 || got[0] != "*.go" {
		t.Errorf("GetPathInclude default = %v, want [*.go]", got)
	}

	if got := r2.GetPathExclude(defaults.PathExclude); len(got) != 1 || got[0] != "*.exe" {
		t.Errorf("GetPathExclude default = %v, want [*.exe]", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
