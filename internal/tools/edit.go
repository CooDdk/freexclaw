package tools

import (
	"fmt"
	"os"
	"strings"
)

// EditFile 在文件中做一次精确字符串替换。
// old 必须在文件中恰好出现一次；零次或多次都返回 error 且不改动文件。
func EditFile(path, old, new string) error {
	if old == "" {
		return fmt.Errorf("old 参数不能为空")
	}

	path = expandPath(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	content := string(data)
	count := strings.Count(content, old)
	switch {
	case count == 0:
		return fmt.Errorf("未找到匹配的原文")
	case count > 1:
		return fmt.Errorf("原文在文件中出现多次 (%d 次)，请扩大上下文以唯一定位", count)
	}

	updated := strings.Replace(content, old, new, 1)
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	return nil
}
