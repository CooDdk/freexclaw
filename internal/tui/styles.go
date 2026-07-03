package tui

import "github.com/charmbracelet/lipgloss"

var (
	PrimaryColor   = lipgloss.Color("#8B5CF6")
	SecondaryColor = lipgloss.Color("#6366F1")
	AccentColor    = lipgloss.Color("#EC4899")
	TextColor      = lipgloss.Color("#F3F4F6")
	MutedColor     = lipgloss.Color("#9CA3AF")
	BorderColor    = lipgloss.Color("#374151")
	UserColor      = lipgloss.Color("#A78BFA")
	AssistantColor = lipgloss.Color("#22D3EE")
	ErrorColor     = lipgloss.Color("#EF4444")
)

var (
	AppStyle = lipgloss.NewStyle()

	ChatViewStyle = lipgloss.NewStyle().
			Padding(1, 2)

	InputAreaStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(BorderColor).
			Padding(1, 2)

	UserPrefixStyle = lipgloss.NewStyle().
			Foreground(UserColor).
			Bold(true)

	AIPrefixStyle = lipgloss.NewStyle().
			Foreground(AssistantColor)

	MessageContentStyle = lipgloss.NewStyle().
				Foreground(TextColor)

	SystemMessageStyle = lipgloss.NewStyle().
				Foreground(MutedColor).
				Italic(true).
				PaddingLeft(2)

	ToolResultStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#10B981")).
				PaddingLeft(2)

	ThinkingStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Italic(true)

	PlaceholderStyle = lipgloss.NewStyle().
				Foreground(MutedColor).
				Italic(true)

	WelcomeStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	StatusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1F2937")).
			Foreground(MutedColor).
			Padding(0, 1)

	HelpStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Faint(true).
			Padding(0, 1)

	CommandHintBorder = lipgloss.Color("#4B5563")

	CommandHintStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(CommandHintBorder).
				Padding(0, 1)

	CommandHintSelectedItemStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FBBF24")).
					Bold(true)

	CommandHintItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E5E7EB"))

	CommandHintDescStyle = lipgloss.NewStyle().
				Foreground(MutedColor).
				Faint(true)
)
