package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/CooDdk/freexclaw/internal/config"
	"github.com/CooDdk/freexclaw/internal/tools"
	"github.com/CooDdk/freexclaw/internal/tui"
)

func main() {
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

	model, err := tui.NewModel(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FreeX Claw] 初始化界面失败: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[FreeX Claw] 运行失败: %v\n", err)
		os.Exit(1)
	}
}
