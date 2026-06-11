package replacer

import (
	"testing"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"10MB", 10 * 1024 * 1024},
		{"1KB", 1024},
		{"512B", 512},
		{"1GB", 1 << 30},
		{"1TB", 1 << 40},
		{"10mb", 10 * 1024 * 1024}, // 大小写不敏感
		{" 5MB ", 5 * 1024 * 1024}, // 去空格
		{"", 0},
		{"0", 0},
		{"100", 100}, // 无单位时当 bytes
	}

	for _, tt := range tests {
		got := parseSize(tt.input)
		if got != tt.want {
			t.Errorf("parseSize(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestFilepathMatch(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "main.py", false},
		{"*.env", ".env", true},
		{"*.yaml", "config.yaml", true},
		{"*.lock", "go.lock", true},
		{"*.exe", "pvctl.exe", true},
	}

	for _, tt := range tests {
		got, err := filepathMatch(tt.pattern, tt.path)
		if err != nil {
			t.Errorf("filepathMatch(%q, %q) error: %v", tt.pattern, tt.path, err)
			continue
		}
		if got != tt.want {
			t.Errorf("filepathMatch(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
		}
	}
}
