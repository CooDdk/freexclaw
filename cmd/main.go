package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/CooDdk/freexclaw/internal/config"
	"github.com/CooDdk/freexclaw/internal/tools"
	"github.com/CooDdk/freexclaw/internal/tui"
)

func main() {
	splashEnabled := flag.Bool("splash", false, "启用启动动画（默认关闭）")
	resumeID := flag.String("resume", "", "恢复指定 ID 的历史会话（默认新建空会话）")
	yolo := flag.Bool("yolo", false, "跳过 run_command 的逐次确认提示（自动化脚本用）")
	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FreeX Claw] 获取当前目录失败: %v\n", err)
		os.Exit(1)
	}
	tools.SetWorkDir(cwd)

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FreeX Claw] 加载配置失败: %v\n", err)
		os.Exit(1)
	}

	if cfg.APIKey == "" {
		configPath, _ := config.GetConfigPath()
		fmt.Fprintln(os.Stderr, "╔══════════════════════════════════════════╗")
		fmt.Fprintln(os.Stderr, "║          FreeX Claw - 配置向导            ║")
		fmt.Fprintln(os.Stderr, "╠══════════════════════════════════════════╣")
		fmt.Fprintln(os.Stderr, "║  欢迎使用 FreeX Claw 终端 AI 助手!        ║")
		fmt.Fprintln(os.Stderr, "║                                          ║")
		fmt.Fprintln(os.Stderr, "║  请先配置 API Key 以继续使用。           ║")
		fmt.Fprintln(os.Stderr, "║                                          ║")
		fmt.Fprintf(os.Stderr, "║  配置文件: %s  \n", configPath)
		fmt.Fprintln(os.Stderr, "║                                          ║")
		fmt.Fprintln(os.Stderr, "║  请编辑上述配置文件，填入您的 API Key     ║")
		fmt.Fprintln(os.Stderr, "║  后重新运行程序。                        ║")
		fmt.Fprintln(os.Stderr, "╚══════════════════════════════════════════╝")
		os.Exit(1)
	}

	// Print brand banner once (into scrollback)
	width := 80
	if w, _, terr := term.GetSize(int(os.Stdout.Fd())); terr == nil && w > 0 {
		width = w
	}
	fmt.Println(tui.RenderBannerPublic(width))
	fmt.Println()

	model, err := tui.NewModel(cfg, tui.ModelOptions{
		Splash:   *splashEnabled,
		ResumeID: strings.TrimSpace(*resumeID),
		Yolo:     *yolo,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FreeX Claw] 初始化界面失败: %v\n", err)
		os.Exit(1)
	}

	// Inline rendering: no alt-screen, no mouse capture.
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[FreeX Claw] 运行失败: %v\n", err)
		os.Exit(1)
	}

	// On graceful exit, hint the user how to resume this exact session.
	if id := model.CurrentSessionID(); id != "" && model.CurrentSessionHasMessages() {
		fmt.Println()
		fmt.Println("Resume this session with:")
		fmt.Printf("  %s --resume %s\n", resumeCommandName(), id)
	}
}

// resumeCommandName returns the invocation string used in the resume hint.
// Strategy: if the binary is resolvable on PATH (matches the running executable
// after symlink resolution), return its basename so the hint is short and works
// in the user's shell. Otherwise fall back to the absolute path so the hint is
// verbose but always runnable.
func resumeCommandName() string {
	exe, err := os.Executable()
	if err != nil || exe == "" {
		exe = os.Args[0]
	}
	if exe == "" {
		return "freexclaw"
	}
	base := filepath.Base(exe)
	if resolved, lerr := exec.LookPath(base); lerr == nil {
		a, e1 := filepath.EvalSymlinks(resolved)
		b, e2 := filepath.EvalSymlinks(exe)
		if e1 == nil && e2 == nil && strings.EqualFold(a, b) {
			return base
		}
	}
	return exe
}
