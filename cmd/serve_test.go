package cmd

import (
	"encoding/json"
	"testing"
)

func TestExtractResponseText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain string",
			input: `"hello world"`,
			want:  "hello world",
		},
		{
			name:  "object with content string",
			input: `{"content":"file content here"}`,
			want:  "file content here",
		},
		{
			name:  "object with output field",
			input: `{"output":"command output"}`,
			want:  "command output",
		},
		{
			name:  "object with result field",
			input: `{"result":"search results"}`,
			want:  "search results",
		},
		{
			name: "object with content array (Claude format)",
			input: `{"content":[{"type":"text","text":"line 1"},{"type":"text","text":"line 2"}]}`,
			want:  "line 1line 2",
		},
		{
			name:  "empty string",
			input: `""`,
			want:  "",
		},
		{
			name:  "null",
			input: `null`,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw json.RawMessage
			if err := json.Unmarshal([]byte(tt.input), &raw); err != nil {
				t.Fatalf("unmarshal input: %v", err)
			}
			got := extractResponseText(raw)
			if got != tt.want {
				t.Errorf("extractResponseText = %q, want %q", got, tt.want)
			}
		})
	}
}
