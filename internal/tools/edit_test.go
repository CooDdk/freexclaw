package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	return path
}

func TestEditFile_UniqueMatchReplacesInPlace(t *testing.T) {
	path := writeTempFile(t, "sample.txt", "alpha\nbeta\ngamma\n")

	if err := EditFile(path, "beta", "BETA"); err != nil {
		t.Fatalf("EditFile: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := "alpha\nBETA\ngamma\n"
	if string(got) != want {
		t.Fatalf("content mismatch:\n want %q\n  got %q", want, string(got))
	}
}

func TestEditFile_ZeroMatchReturnsError(t *testing.T) {
	path := writeTempFile(t, "sample.txt", "alpha\nbeta\n")

	err := EditFile(path, "not-in-file", "X")
	if err == nil {
		t.Fatalf("expected error for zero match, got nil")
	}
	if !strings.Contains(err.Error(), "未找到") {
		t.Fatalf("expected 未找到 in error, got %q", err.Error())
	}
}

func TestEditFile_MultipleMatchReturnsError(t *testing.T) {
	path := writeTempFile(t, "sample.txt", "beta\nbeta\n")

	err := EditFile(path, "beta", "X")
	if err == nil {
		t.Fatalf("expected error for multiple matches, got nil")
	}
	if !strings.Contains(err.Error(), "多次") {
		t.Fatalf("expected 多次 in error, got %q", err.Error())
	}

	got, _ := os.ReadFile(path)
	if string(got) != "beta\nbeta\n" {
		t.Fatalf("file should be unchanged on error, got %q", string(got))
	}
}

func TestEditFile_EmptyOldReturnsError(t *testing.T) {
	path := writeTempFile(t, "sample.txt", "alpha\n")

	if err := EditFile(path, "", "X"); err == nil {
		t.Fatalf("expected error for empty old string, got nil")
	}
}

func TestEditFile_PreservesCRLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crlf.txt")
	if err := os.WriteFile(path, []byte("alpha\r\nbeta\r\ngamma\r\n"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := EditFile(path, "beta", "BETA"); err != nil {
		t.Fatalf("EditFile: %v", err)
	}

	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "\r\n") {
		t.Fatalf("CRLF should be preserved, got %q", string(got))
	}
	if !strings.Contains(string(got), "BETA") {
		t.Fatalf("replacement should be applied, got %q", string(got))
	}
}

func TestEditFile_FileNotFound(t *testing.T) {
	if err := EditFile(filepath.Join(t.TempDir(), "missing.txt"), "a", "b"); err == nil {
		t.Fatalf("expected error for missing file, got nil")
	}
}
