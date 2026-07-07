package tools

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

type GlobOptions struct {
	Pattern    string
	Path       string
	MaxResults int
}

// Glob 按 pattern 递归匹配（支持 **），按 mtime 降序返回，最多 MaxResults 条。
func Glob(opts GlobOptions) ([]string, error) {
	if opts.Pattern == "" {
		return nil, fmt.Errorf("pattern 不能为空")
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

	fsys := os.DirFS(root)
	rawMatches, err := doublestar.Glob(fsys, opts.Pattern, doublestar.WithFilesOnly())
	if err != nil {
		return nil, fmt.Errorf("glob 失败: %w", err)
	}

	type entry struct {
		path  string
		mtime int64
	}
	entries := make([]entry, 0, len(rawMatches))
	for _, rel := range rawMatches {
		full := rel
		if !isAbs(rel) {
			full = joinPath(root, rel)
		}
		info, statErr := os.Stat(full)
		if statErr != nil {
			continue
		}
		entries = append(entries, entry{path: full, mtime: info.ModTime().UnixNano()})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].mtime > entries[j].mtime
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.path
	}
	return out, nil
}

func isAbs(p string) bool {
	return len(p) > 0 && (p[0] == '/' || p[0] == '\\' || (len(p) > 1 && p[1] == ':'))
}

func joinPath(root, rel string) string {
	if strings.HasSuffix(root, "/") || strings.HasSuffix(root, "\\") {
		return root + rel
	}
	return root + string(os.PathSeparator) + rel
}

// FormatGlobResults 按每行一条输出，超过 limit 显示截断提示。
func FormatGlobResults(files []string, pattern string, limit int) string {
	if limit <= 0 {
		limit = 200
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("模式: %s，命中 %d 个文件\n\n", pattern, len(files)))
	shown := files
	truncated := false
	if len(shown) > limit {
		shown = shown[:limit]
		truncated = true
	}
	for _, f := range shown {
		sb.WriteString(f)
		sb.WriteByte('\n')
	}
	if truncated {
		sb.WriteString(fmt.Sprintf("\n(截断，仅显示前 %d 条)\n", limit))
	}
	return sb.String()
}
