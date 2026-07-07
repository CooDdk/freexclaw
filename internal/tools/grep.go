package tools

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// GrepOptions 控制 Grep 的行为。Pattern 是必填 regex。
type GrepOptions struct {
	Pattern    string
	Path       string
	Glob       string // 例如 *.go；空表示不过滤
	IgnoreCase bool
	MaxResults int // 0 表示 200 默认上限
}

type GrepMatch struct {
	File       string
	LineNumber int
	Line       string
}

// Grep 在 Path 下递归扫描，按 GrepOptions 规则匹配。
func Grep(opts GrepOptions) ([]GrepMatch, error) {
	if opts.Pattern == "" {
		return nil, fmt.Errorf("pattern 不能为空")
	}
	pattern := opts.Pattern
	if opts.IgnoreCase && !strings.HasPrefix(pattern, "(?i)") {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("非法正则: %w", err)
	}

	root := opts.Path
	if root == "" {
		root = "."
	}
	root = expandPath(root)

	limit := opts.MaxResults
	if limit <= 0 {
		limit = 200
	}

	var matches []GrepMatch
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // 权限或其他 IO 错误跳过
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if opts.Glob != "" {
			ok, gerr := filepath.Match(opts.Glob, d.Name())
			if gerr != nil || !ok {
				return nil
			}
		}
		if len(matches) >= limit {
			return filepath.SkipAll
		}
		matches = append(matches, scanFile(path, re, limit-len(matches))...)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].File != matches[j].File {
			return matches[i].File < matches[j].File
		}
		return matches[i].LineNumber < matches[j].LineNumber
	})
	return matches, nil
}

func scanFile(path string, re *regexp.Regexp, remaining int) []GrepMatch {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var out []GrepMatch
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			out = append(out, GrepMatch{
				File:       path,
				LineNumber: lineNum,
				Line:       line,
			})
			if len(out) >= remaining {
				break
			}
		}
	}
	return out
}

// FormatGrepResults 生成 file:line:content 输出，超过 limit 显示截断提示。
func FormatGrepResults(matches []GrepMatch, pattern string, limit int) string {
	if limit <= 0 {
		limit = 200
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("模式: %s，命中 %d 行\n\n", pattern, len(matches)))
	shown := matches
	truncated := false
	if len(shown) > limit {
		shown = shown[:limit]
		truncated = true
	}
	for _, m := range shown {
		sb.WriteString(fmt.Sprintf("%s:%d:%s\n", m.File, m.LineNumber, m.Line))
	}
	if truncated {
		sb.WriteString(fmt.Sprintf("\n(截断，仅显示前 %d 行)\n", limit))
	}
	return sb.String()
}
