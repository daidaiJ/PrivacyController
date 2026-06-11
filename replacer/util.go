package replacer

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// filepathMatch 用 glob 模式匹配路径
func filepathMatch(pattern, path string) (bool, error) {
	return filepath.Match(pattern, path)
}

// parseSize 解析人类可读的大小字符串为 bytes
// 支持: B, KB, MB, GB, TB（大小写不敏感）
// 如 "10MB" → 10485760
func parseSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	s = strings.ToUpper(s)
	var multiplier int64 = 1
	var numStr string

	switch {
	case strings.HasSuffix(s, "TB"):
		multiplier = 1 << 40
		numStr = strings.TrimSuffix(s, "TB")
	case strings.HasSuffix(s, "GB"):
		multiplier = 1 << 30
		numStr = strings.TrimSuffix(s, "GB")
	case strings.HasSuffix(s, "MB"):
		multiplier = 1 << 20
		numStr = strings.TrimSuffix(s, "MB")
	case strings.HasSuffix(s, "KB"):
		multiplier = 1 << 10
		numStr = strings.TrimSuffix(s, "KB")
	case strings.HasSuffix(s, "B"):
		multiplier = 1
		numStr = strings.TrimSuffix(s, "B")
	default:
		numStr = s
	}

	numStr = strings.TrimSpace(numStr)
	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0
	}
	return num * multiplier
}

// compileGlobRegex 将 glob 模式转换为正则表达式（用于更灵活的路径匹配）
// 注意：当前使用 filepath.Match，此函数备用于后续扩展
func compileGlobRegex(pattern string) (*regexp.Regexp, error) {
	// 转义正则特殊字符，除了 * 和 ?
		var parts []string
		for i := 0; i < len(pattern); i++ {
			c := pattern[i]
			switch c {
			case '*':
				parts = append(parts, ".*")
			case '?':
				parts = append(parts, ".")
			case '.', '+', '^', '$', '(', ')', '[', ']', '{', '}', '|', '\\':
				parts = append(parts, `\`+string(c))
			default:
				parts = append(parts, string(c))
			}
		}
		re, err := regexp.Compile("^" + strings.Join(parts, "") + "$")
		if err != nil {
			return nil, fmt.Errorf("编译 glob 正则失败 %q: %w", pattern, err)
		}
		return re, nil
}
