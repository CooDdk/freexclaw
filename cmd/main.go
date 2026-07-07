package main

import (
	"flag"
	"fmt"
	"os"
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

// resumeCommandName returns the invocation name used in the resume hint.
// Uses os.Args[0] basename so it reflects however the user installed the
// binary (e.g. renamed to `fx`). The .exe suffix is stripped for readability.
func resumeCommandName() string {
	name := filepath.Base(os.Args[0])
	if ext := filepath.Ext(name); strings.EqualFold(ext, ".exe") {
		name = strings.TrimSuffix(name, ext)
	}
	if name == "" || name == "." {
		return "freexclaw"
	}
	return name
}
