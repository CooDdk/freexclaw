package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupGrepTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"a.go":         "package a\nfunc Hello() {}\n",
		"b.go":         "package b\nfunc HELLO() {}\n",
		"nested/c.go":  "package c\nvar Hello = 1\n",
		"nested/d.txt": "hello txt\n",
		"skip.bin":     "\x00\x01hello\x02", // binary-ish, still matches text
	}
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("setup mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("setup write: %v", err)
		}
	}
	return dir
}

func TestGrep_LiteralMatchCaseSensitive(t *testing.T) {
	dir := setupGrepTree(t)
	res, err := Grep(GrepOptions{Pattern: "Hello", Path: dir})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(res) < 2 {
		t.Fatalf("expected at least 2 matches, got %d: %#v", len(res), res)
	}
	for _, m := range res {
		if !strings.Contains(m.Line, "Hello") {
			t.Fatalf("match line should contain Hello, got %q", m.Line)
		}
	}
}

func TestGrep_CaseInsensitive(t *testing.T) {
	dir := setupGrepTree(t)
	res, err := Grep(GrepOptions{Pattern: "hello", Path: dir, IgnoreCase: true})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	seen := map[string]bool{}
	for _, m := range res {
		seen[filepath.Base(m.File)] = true
	}
	if !seen["a.go"] || !seen["b.go"] {
		t.Fatalf("case-insensitive should match Hello and HELLO, got %#v", seen)
	}
}

func TestGrep_GlobFilter(t *testing.T) {
	dir := setupGrepTree(t)
	res, err := Grep(GrepOptions{Pattern: "hello", Path: dir, Glob: "*.go", IgnoreCase: true})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	for _, m := range res {
		if filepath.Ext(m.File) != ".go" {
			t.Fatalf("glob *.go should exclude %s", m.File)
		}
	}
}

func TestGrep_NoMatchReturnsEmpty(t *testing.T) {
	dir := setupGrepTree(t)
	res, err := Grep(GrepOptions{Pattern: "definitely-not-present", Path: dir})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(res))
	}
}

func TestGrep_InvalidRegexReturnsError(t *testing.T) {
	if _, err := Grep(GrepOptions{Pattern: "([bad", Path: "."}); err == nil {
		t.Fatalf("expected error for invalid regex")
	}
}

func TestFormatGrepResults_Truncates(t *testing.T) {
	var res []GrepMatch
	for i := 0; i < 250; i++ {
		res = append(res, GrepMatch{File: "x.go", Line: "hit", LineNumber: i + 1})
	}
	out := FormatGrepResults(res, "hit", 200)
	if !strings.Contains(out, "(截断") {
		t.Fatalf("expected truncation notice, got %q", out)
	}
}
