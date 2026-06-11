package replacer

import (
	"testing"

	"github.com/privatebox/pvctl/config"
)

func newTestEngine(rules []config.Rule) *Engine {
	cfg := &config.Config{
		Defaults: config.Defaults{},
		Rules:    rules,
	}
	return New(cfg)
}

func TestSubstringReplace(t *testing.T) {
	engine := newTestEngine([]config.Rule{
		{Match: "北京市朝阳区", Mode: "substring", Placeholder: "<ADDR>"},
	})
	defer engine.Close()

	tests := []struct {
		input string
		want  string
	}{
		{"地址：北京市朝阳区建国路88号", "地址：<ADDR>建国路88号"},
		{"没有敏感信息", "没有敏感信息"},
		{"北京市朝阳区", "<ADDR>"},
		{"", ""},
	}

	for _, tt := range tests {
		got := engine.Replace(tt.input, "", int64(len(tt.input)))
		if got != tt.want {
			t.Errorf("SubstringReplace(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExactReplace(t *testing.T) {
	engine := newTestEngine([]config.Rule{
		{Match: "test", Mode: "exact", Placeholder: "<MASKED>"},
	})
	defer engine.Close()

	tests := []struct {
		input string
		want  string
	}{
		{"this is a test", "this is a <MASKED>"},
		{"testing", "testing"},           // "test" 是 "testing" 的子串，exact 不应匹配
		{"test-test", "<MASKED>-<MASKED>"},
		{"test123", "test123"},            // 后面紧跟数字，exact 不匹配
	}

	for _, tt := range tests {
		got := engine.Replace(tt.input, "", int64(len(tt.input)))
		if got != tt.want {
			t.Errorf("ExactReplace(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExactReplaceChinese(t *testing.T) {
	// exact 模式基于 ASCII 字符边界，中文字符不被视为字母数字
	// 因此 "张三" 会匹配 "张三丰" 中的 "张三"
	// 中文场景应使用 token 模式（gse 分词）
	engine := newTestEngine([]config.Rule{
		{Match: "张三", Mode: "exact", Placeholder: "<NAME>"},
	})
	defer engine.Close()

	tests := []struct {
		input string
		want  string
	}{
		{"张三来了", "<NAME>来了"},
		{"张三丰来了", "<NAME>丰来了"}, // exact 会匹配（中文字符非 alphanumeric 边界）
	}

	for _, tt := range tests {
		got := engine.Replace(tt.input, "", int64(len(tt.input)))
		if got != tt.want {
			t.Errorf("ExactReplace(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTokenReplace(t *testing.T) {
	engine := newTestEngine([]config.Rule{
		{Match: "张三", Mode: "token", Placeholder: "<NAME>"},
	})
	defer engine.Close()

	tests := []struct {
		input string
		want  string
	}{
		{"张三来了", "<NAME>来了"},
		{"张三丰来了", "张三丰来了"}, // token 模式不匹配 "张三丰"
	}

	for _, tt := range tests {
		got := engine.Replace(tt.input, "", int64(len(tt.input)))
		if got != tt.want {
			t.Errorf("TokenReplace(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMultipleRules(t *testing.T) {
	engine := newTestEngine([]config.Rule{
		{Match: "张三", Mode: "token", Placeholder: "<NAME>"},
		{Match: "13800138000", Mode: "exact", Placeholder: "<PHONE>"},
		{Match: "北京市朝阳区", Mode: "substring", Placeholder: "<ADDR>"},
	})
	defer engine.Close()

	input := "张三的电话是13800138000，住在北京市朝阳区"
	want := "<NAME>的电话是<PHONE>，住在<ADDR>"
	got := engine.Replace(input, "", int64(len(input)))
	if got != want {
		t.Errorf("MultipleRules = %q, want %q", got, want)
	}
}

func TestPathFiltering(t *testing.T) {
	engine := newTestEngine([]config.Rule{
		{
			Match:       "secret",
			Mode:        "exact",
			Placeholder: "<SECRET>",
			PathInclude: []string{"*.env", "*.yaml"},
		},
	})
	defer engine.Close()

	// 匹配 .env 文件
	got := engine.Replace("secret", "config.env", 6)
	if got != "<SECRET>" {
		t.Errorf("PathFilter .env = %q, want %q", got, "<SECRET>")
	}

	// 不匹配 .go 文件
	got = engine.Replace("secret", "main.go", 6)
	if got != "secret" {
		t.Errorf("PathFilter .go = %q, want %q", got, "secret")
	}

	// filePath 为空时始终匹配
	got = engine.Replace("secret", "", 6)
	if got != "<SECRET>" {
		t.Errorf("PathFilter empty = %q, want %q", got, "<SECRET>")
	}
}

func TestSizeFiltering(t *testing.T) {
	engine := newTestEngine([]config.Rule{
		{
			Match:       "secret",
			Mode:        "exact",
			Placeholder: "<SECRET>",
			MaxFileSize: "10B",
		},
	})
	defer engine.Close()

	// 文件大小在限制内
	got := engine.Replace("secret", "", 5)
	if got != "<SECRET>" {
		t.Errorf("SizeFilter within = %q, want %q", got, "<SECRET>")
	}

	// 文件大小超出限制
	got = engine.Replace("secret", "", 100)
	if got != "secret" {
		t.Errorf("SizeFilter exceeded = %q, want %q", got, "secret")
	}
}

func TestGlobalPathExclude(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{
			PathExclude: []string{"*.exe", "*.png"},
		},
		Rules: []config.Rule{
			{Match: "test", Mode: "substring", Placeholder: "<T>"},
		},
	}
	engine := New(cfg)
	defer engine.Close()

	got := engine.Replace("test", "image.png", 4)
	if got != "test" {
		t.Errorf("GlobalPathExclude .png = %q, want %q", got, "test")
	}

	got = engine.Replace("test", "code.go", 4)
	if got != "<T>" {
		t.Errorf("GlobalPathExclude .go = %q, want %q", got, "<T>")
	}
}

func TestNilEngine(t *testing.T) {
	var engine *Engine
	got := engine.Replace("test", "", 4)
	if got != "test" {
		t.Errorf("NilEngine = %q, want %q", got, "test")
	}
}
