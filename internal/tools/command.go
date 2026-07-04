package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const commandTimeout = 2 * time.Minute

type CommandResult struct {
	Command  string
	Cwd      string
	ExitCode int
	Stdout   string
	Stderr   string
	TimedOut bool
}

func RunCommand(command string, cwd string) (CommandResult, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return CommandResult{}, fmt.Errorf("命令不能为空")
	}

	if err := validateCommand(command); err != nil {
		return CommandResult{}, err
	}

	resolvedCwd, err := resolveCommandCwd(cwd)
	if err != nil {
		return CommandResult{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	cmd := shellCommand(ctx, command)
	cmd.Dir = resolvedCwd

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	result := CommandResult{
		Command: strings.TrimSpace(command),
		Cwd:     resolvedCwd,
		Stdout:  strings.TrimSpace(stdout.String()),
		Stderr:  strings.TrimSpace(stderr.String()),
	}

	if runErr == nil {
		return result, nil
	}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.TimedOut = true
		result.ExitCode = -1
		return result, fmt.Errorf("命令执行超时（>%s）", commandTimeout)
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		result.ExitCode = extractExitCode(exitErr)
		return result, fmt.Errorf("命令执行失败，退出码 %d", result.ExitCode)
	}

	return result, fmt.Errorf("命令执行失败: %w", runErr)
}

func FormatCommandResult(result CommandResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("命令: %s\n", result.Command))
	sb.WriteString(fmt.Sprintf("工作目录: %s\n", result.Cwd))
	if result.TimedOut {
		sb.WriteString("状态: 超时\n")
	} else {
		sb.WriteString(fmt.Sprintf("退出码: %d\n", result.ExitCode))
	}

	if result.Stdout != "" {
		sb.WriteString("\n标准输出:\n")
		sb.WriteString(result.Stdout)
		sb.WriteString("\n")
	}

	if result.Stderr != "" {
		sb.WriteString("\n标准错误:\n")
		sb.WriteString(result.Stderr)
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String())
}

func resolveCommandCwd(cwd string) (string, error) {
	if strings.TrimSpace(cwd) == "" {
		return GetWorkDir(), nil
	}

	resolved := expandPath(strings.TrimSpace(cwd))
	root := filepath.Clean(GetWorkDir())
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return "", fmt.Errorf("解析工作目录失败: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("命令工作目录超出当前工作区: %s", cwd)
	}
	return resolved, nil
}

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", command)
	}
	return exec.CommandContext(ctx, "/bin/sh", "-lc", command)
}

func validateCommand(command string) error {
	lower := strings.ToLower(strings.TrimSpace(command))
	blocked := []string{
		"rm -rf /",
		"rm -rf *",
		"shutdown",
		"reboot",
		"mkfs",
		"format ",
		"del /f /s /q",
		"rd /s /q",
		"remove-item -recurse -force",
		"git reset --hard",
		"git clean -fd",
	}
	for _, token := range blocked {
		if strings.Contains(lower, token) {
			return fmt.Errorf("拒绝执行高风险命令: %s", command)
		}
	}
	return nil
}

func extractExitCode(err *exec.ExitError) int {
	if status, ok := err.Sys().(syscall.WaitStatus); ok {
		return status.ExitStatus()
	}
	return 1
}
