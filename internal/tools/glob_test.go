package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupGlobTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := []string{
		"a.go",
		"b.go",
		"nested/c.go",
		"nested/deep/d.go",
		"nested/e.txt",
		"skip.md",
	}
	for _, rel := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return dir
}

func TestGlob_DoublestarMatchesAllDepths(t *testing.T) {
	dir := setupGlobTree(t)
	res, err := Glob(GlobOptions{Pattern: "**/*.go", Path: dir})
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(res) < 4 {
		t.Fatalf("expected at least 4 .go files, got %d: %#v", len(res), res)
	}
	for _, m := range res {
		if !strings.HasSuffix(m, ".go") {
			t.Fatalf("glob returned non-.go entry %q", m)
		}
	}
}

func TestGlob_FlatPatternOneLevel(t *testing.T) {
	dir := setupGlobTree(t)
	res, err := Glob(GlobOptions{Pattern: "*.go", Path: dir})
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected exactly 2 top-level .go files, got %d: %#v", len(res), res)
	}
}

func TestGlob_SortedByMtimeDesc(t *testing.T) {
	dir := setupGlobTree(t)
	newer := filepath.Join(dir, "b.go")
	future := time.Now().Add(1 * time.Hour)
	if err := os.Chtimes(newer, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	res, err := Glob(GlobOptions{Pattern: "*.go", Path: dir})
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(res) == 0 || filepath.Base(res[0]) != "b.go" {
		t.Fatalf("expected b.go first (newest), got %#v", res)
	}
}

func TestGlob_EmptyPatternError(t *testing.T) {
	if _, err := Glob(GlobOptions{Pattern: "", Path: "."}); err == nil {
		t.Fatalf("expected error for empty pattern")
	}
}

func TestGlob_NoMatchesEmpty(t *testing.T) {
	dir := setupGlobTree(t)
	res, err := Glob(GlobOptions{Pattern: "*.rs", Path: dir})
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected empty, got %#v", res)
	}
}

func TestFormatGlobResults_Truncates(t *testing.T) {
	files := make([]string, 300)
	for i := range files {
		files[i] = filepath.Join(".", "x.go")
	}
	out := FormatGlobResults(files, "**/*.go", 200)
	if !strings.Contains(out, "(截断") {
		t.Fatalf("expected truncation notice, got %q", out)
	}
}
