package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var workDir string

func SetWorkDir(dir string) {
	workDir = dir
}

func GetWorkDir() string {
	return workDir
}

type FileContent struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type ToolResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func ReadFile(path string) (FileContent, error) {
	path = expandPath(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return FileContent{}, fmt.Errorf("读取文件失败: %w", err)
	}
	return FileContent{
		Path:    path,
		Content: string(data),
	}, nil
}

func WriteFile(path string, content string, append bool) error {
	path = expandPath(path)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	var err error
	if append {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("打开文件失败: %w", err)
		}
		defer f.Close()
		_, err = f.WriteString(content)
	} else {
		err = os.WriteFile(path, []byte(content), 0644)
	}
	if err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	return nil
}

func ListDir(path string) ([]string, error) {
	path = expandPath(path)
	if path == "" {
		path = "."
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	var result []string
	for _, f := range files {
		name := f.Name()
		if f.IsDir() {
			name = "📁 " + name + "/"
		} else {
			name = "📄 " + name
		}
		result = append(result, name)
	}
	return result, nil
}

func FormatFileContent(fc FileContent) string {
	return fmt.Sprintf("📄 文件: %s\n```\n%s\n```", fc.Path, fc.Content)
}

func FormatToolResult(success bool, message string, data string) string {
	if success {
		return fmt.Sprintf("✅ %s", message)
	}
	return fmt.Sprintf("❌ %s", message)
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = strings.Replace(path, "~", home, 1)
		}
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(workDir, path)
	}

	return filepath.Clean(path)
}

func ParseWriteArgs(args string) (path string, content string, append bool, err error) {
	args = strings.TrimSpace(args)
	
	if strings.HasPrefix(args, "-a ") {
		append = true
		args = strings.TrimPrefix(args, "-a ")
	}
	
	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		return "", "", false, fmt.Errorf("用法: /write <文件路径> <内容>")
	}
	
	path = parts[0]
	content = parts[1]
	return path, content, append, nil
}

func SerializeFileContent(fc FileContent) string {
	data, _ := json.Marshal(fc)
	return string(data)
}

func DeserializeFileContent(data string) (*FileContent, error) {
	var fc FileContent
	if err := json.Unmarshal([]byte(data), &fc); err != nil {
		return nil, err
	}
	return &fc, nil
}