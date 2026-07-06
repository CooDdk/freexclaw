package tui

import "github.com/charmbracelet/lipgloss"

// BrandSpinnerFrames 是 FreeX Claw 品牌 spinner 的四帧循环。
var BrandSpinnerFrames = []string{"✦", "✧", "✩", "✪"}

// 品牌色（与 styles.go 中的 UserColor / AssistantColor 保持一致）
var (
	MarkerUserStyle   = lipgloss.NewStyle().Foreground(UserColor).Bold(true)
	MarkerAssistStyle = lipgloss.NewStyle().Foreground(AssistantColor).Bold(true)
	MarkerToolStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F97316")).Bold(true)
	MarkerWarnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Bold(true)
	MarkerOKStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
	MarkerFailStyle   = lipgloss.NewStyle().Foreground(ErrorColor).Bold(true)
)

func MarkerUser() string      { return MarkerUserStyle.Render("❯") }
func MarkerAssistant() string { return MarkerAssistStyle.Render("✻") }
func MarkerToolStart() string { return MarkerToolStyle.Render("▸") }
func MarkerToolOK() string    { return MarkerOKStyle.Render("✓") }
func MarkerToolFail() string  { return MarkerFailStyle.Render("✗") }
func MarkerWarn() string      { return MarkerWarnStyle.Render("⚠") }
