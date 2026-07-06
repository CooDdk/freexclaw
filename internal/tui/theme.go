package tui

import "github.com/charmbracelet/lipgloss"

// BrandSpinnerFrames 是 FreeX Claw 品牌 spinner 的四帧循环。
var BrandSpinnerFrames = []string{"✦", "✧", "✩", "✪"}

// 品牌色（颜色复用自 styles.go 调色板）
var (
	MarkerUserStyle      = lipgloss.NewStyle().Foreground(UserColor).Bold(true)
	MarkerAssistantStyle = lipgloss.NewStyle().Foreground(AssistantColor).Bold(true)
	MarkerToolStyle      = lipgloss.NewStyle().Foreground(ToolColor).Bold(true)
	MarkerWarnStyle      = lipgloss.NewStyle().Foreground(WarnColor).Bold(true)
	MarkerOKStyle        = lipgloss.NewStyle().Foreground(OKColor).Bold(true)
	MarkerFailStyle      = lipgloss.NewStyle().Foreground(ErrorColor).Bold(true)
)

func MarkerUser() string      { return MarkerUserStyle.Render("❯") }
func MarkerAssistant() string { return MarkerAssistantStyle.Render("✻") }
func MarkerToolStart() string { return MarkerToolStyle.Render("▸") }
func MarkerToolOK() string    { return MarkerOKStyle.Render("✓") }
func MarkerToolFail() string  { return MarkerFailStyle.Render("✗") }
func MarkerWarn() string      { return MarkerWarnStyle.Render("⚠") }
