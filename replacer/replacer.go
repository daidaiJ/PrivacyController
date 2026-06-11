package replacer

import (
	"strings"
	"sync"

	"github.com/go-ego/gse"
	"github.com/privatebox/pvctl/config"
)

// Engine 替换引擎，持有编译后的规则
type Engine struct {
	seg      gse.Segmenter
	rules    []compiledRule
	defaults config.Defaults
	mu       sync.Mutex
}

type compiledRule struct {
	raw     config.Rule
	matcher func(text string, seg *gse.Segmenter) string
}

// New 从配置创建替换引擎
func New(cfg *config.Config) *Engine {
	var seg gse.Segmenter
	seg.SkipLog = true
	seg.LoadDict() // 加载内置词典

	e := &Engine{seg: seg, defaults: cfg.Defaults}
	for _, r := range cfg.Rules {
		rule := r // capture
		e.rules = append(e.rules, compiledRule{
			raw:     rule,
			matcher: compileMatcher(rule),
		})
	}
	return e
}

// Close 释放资源（gse 无需特殊清理）
func (e *Engine) Close() {}

// Replace 对输入文本应用所有规则
func (e *Engine) Replace(input string, filePath string, fileSize int64) string {
	if e == nil {
		return input
	}
	output := input
	for _, cr := range e.rules {
		if !matchPath(cr.raw, filePath, e.defaults) {
			continue
		}
		if !matchSize(cr.raw, fileSize, e.defaults) {
			continue
		}
		output = cr.matcher(output, &e.seg)
	}
	return output
}

// matchPath 检查文件路径是否匹配规则的路径过滤条件
func matchPath(r config.Rule, filePath string, defaults config.Defaults) bool {
	if filePath == "" {
		return true
	}

	includes := r.GetPathInclude(defaults.PathInclude)
	excludes := r.GetPathExclude(defaults.PathExclude)

	// 如果有 include 列表，文件必须匹配至少一个
	if len(includes) > 0 {
		matched := false
		for _, pattern := range includes {
			if ok, _ := filepathMatch(pattern, filePath); ok {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 如果有 exclude 列表，文件不能匹配任何一个
	for _, pattern := range excludes {
		if ok, _ := filepathMatch(pattern, filePath); ok {
			return false
		}
	}

	return true
}

// matchSize 检查文件大小是否在规则允许的范围内
func matchSize(r config.Rule, fileSize int64, defaults config.Defaults) bool {
	if fileSize <= 0 {
		return true
	}

	maxSizeStr := r.MaxFileSize
	if maxSizeStr == "" {
		maxSizeStr = defaults.MaxFileSize
	}
	if maxSizeStr == "" {
		return true
	}

	maxSize := parseSize(maxSizeStr)
	return fileSize <= maxSize
}

func compileMatcher(r config.Rule) func(string, *gse.Segmenter) string {
	switch r.Mode {
	case "exact":
		return compileExactMatcher(r.Match, r.Placeholder)
	case "token":
		return compileTokenMatcher(r.Match, r.Placeholder)
	default: // substring
		return compileSubstringMatcher(r.Match, r.Placeholder)
	}
}

// --- substring 模式 ---

func compileSubstringMatcher(match, placeholder string) func(string, *gse.Segmenter) string {
	return func(text string, _ *gse.Segmenter) string {
		return strings.ReplaceAll(text, match, placeholder)
	}
}

// --- exact 模式 ---

func compileExactMatcher(match, placeholder string) func(string, *gse.Segmenter) string {
	return func(text string, _ *gse.Segmenter) string {
		return exactWordReplace(text, match, placeholder)
	}
}

// exactWordReplace 全词匹配替换（支持中英文混合）
// 英文: 前后必须是 \b 边界（非字母数字）
// 中文: 前后字符不能是 match 的子集（防止"张三"匹配到"张三丰"中的"张三"）
func exactWordReplace(text, match, placeholder string) string {
	var result strings.Builder
	result.Grow(len(text) + len(placeholder)*2)

	runes := []rune(text)
	matchRunes := []rune(match)
	matchLen := len(matchRunes)
	textLen := len(runes)

	i := 0
	for i <= textLen-matchLen {
		// 检查当前位置是否匹配
		matched := true
		for j := 0; j < matchLen; j++ {
			if runes[i+j] != matchRunes[j] {
				matched = false
				break
			}
		}

		if matched {
			// 检查前边界
			prevOk := i == 0 || !isAlphanumeric(runes[i-1])
			// 检查后边界
			nextOk := i+matchLen >= textLen || !isAlphanumeric(runes[i+matchLen])

			if prevOk && nextOk {
				result.WriteString(placeholder)
				i += matchLen
				continue
			}
		}
		result.WriteRune(runes[i])
		i++
	}
	// 追加剩余字符
	for ; i < textLen; i++ {
		result.WriteRune(runes[i])
	}

	return result.String()
}

// isAlphanumeric 判断字符是否是字母数字（包括全角）
func isAlphanumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		(r >= '０' && r <= '９') ||
		(r >= 'ａ' && r <= 'ｚ') ||
		(r >= 'Ａ' && r <= 'Ｚ') ||
		r == '_'
}

// --- token 模式（使用 gse 分词） ---

func compileTokenMatcher(match, placeholder string) func(string, *gse.Segmenter) string {
	return func(text string, seg *gse.Segmenter) string {
		if seg == nil {
			// 没有分词器时 fallback 到 exact 模式
			return exactWordReplace(text, match, placeholder)
		}
		return tokenReplace(text, match, placeholder, seg)
	}
}

// tokenPos 带位置信息的分词结果
type tokenPos struct {
	word  string
	start int // 在原始字符串中的 byte 位置
	end   int
}

// tokenReplace 使用 gse 分词后精确匹配替换
// 对文本和 match 分别使用 Cut 分词，在文本 token 序列中滑动窗口匹配连续的 token
func tokenReplace(text, match, placeholder string, seg *gse.Segmenter) string {
	// 统一使用 Cut 模式分词（保证 match 和 text 使用相同算法）
	textTokens := cutWithPositions(seg, text)

	// 对 match 分词（使用相同的 Cut 模式）
	matchWords := seg.Cut(match, true)
	matchTokens := make([]string, 0, len(matchWords))
	for _, w := range matchWords {
		w = strings.TrimSpace(w)
		if w != "" {
			matchTokens = append(matchTokens, w)
		}
	}
	matchLen := len(matchTokens)
	if matchLen == 0 {
		return text
	}

	// 在文本 token 序列中滑动窗口匹配
	replaceAt := make([]bool, len(textTokens))
	for i := 0; i <= len(textTokens)-matchLen; i++ {
		matched := true
		for j := 0; j < matchLen; j++ {
			if textTokens[i+j].word != matchTokens[j] {
				matched = false
				break
			}
		}
		if matched {
			for j := 0; j < matchLen; j++ {
				replaceAt[i+j] = true
			}
			i += matchLen - 1
		}
	}

	// 从右到左构建结果（避免偏移量变化）
	result := []byte(text)
	for i := len(textTokens) - 1; i >= 0; i-- {
		if replaceAt[i] {
			start := i
			for start > 0 && replaceAt[start-1] {
				start--
			}
			blockStart := textTokens[start].start
			blockEnd := textTokens[i].end

			before := string(result[:blockStart])
			after := string(result[blockEnd:])
			result = []byte(before + placeholder + after)
			i = start
		}
	}

	return string(result)
}

// cutWithPositions 使用 Cut 分词并计算每个 token 的位置
// 通过顺序扫描原文来定位每个 token 的 byte 偏移
func cutWithPositions(seg *gse.Segmenter, text string) []tokenPos {
	words := seg.Cut(text, true)

	var tokens []tokenPos
	pos := 0 // 当前扫描位置（byte 偏移）

	for _, word := range words {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}

		// 在 text 中从 pos 开始查找 word
		idx := strings.Index(text[pos:], word)
		if idx < 0 {
			// 找不到时跳过（不应发生，除非分词结果异常）
			continue
		}

		start := pos + idx
		end := start + len(word)

		tokens = append(tokens, tokenPos{
			word:  word,
			start: start,
			end:   end,
		})

		pos = end // 移动扫描位置
	}

	return tokens
}
