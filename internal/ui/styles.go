package ui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Name string

	AddBg lipgloss.Color
	DelBg lipgloss.Color

	CursorAddBg  lipgloss.Color
	CursorDelBg  lipgloss.Color
	CursorCtxBg  lipgloss.Color
	CursorAddFg  lipgloss.Color
	CursorDelFg  lipgloss.Color
	CursorCtxFg  lipgloss.Color

	GutterAdd lipgloss.Color
	GutterDel lipgloss.Color
	GutterCtx lipgloss.Color

	StatusApproved lipgloss.Color
	StatusChanged  lipgloss.Color
	StatusViewed   lipgloss.Color
	StatusNew      lipgloss.Color

	CommentFg lipgloss.Color
	CommentBg lipgloss.Color

	SearchBg lipgloss.Color
	SearchFg lipgloss.Color

	CanvasBg      lipgloss.Color
	BorderNormal  lipgloss.Color
	BorderFocused lipgloss.Color
	TopBarBg      lipgloss.Color
	TopBarFg      lipgloss.Color
	StatusBarBg   lipgloss.Color
	StatusBarFg   lipgloss.Color

	DimText     lipgloss.Color
	NormalText  lipgloss.Color
	BrightText  lipgloss.Color
	AccentText  lipgloss.Color
	AccentText2 lipgloss.Color

	StatsAdded   lipgloss.Color
	StatsDeleted lipgloss.Color

	LineNumberFg lipgloss.Color

	ChromaTheme string
}

var Themes []Theme

var (
	PaneStyle        lipgloss.Style
	FocusedPaneStyle lipgloss.Style

	TopBarStyle          lipgloss.Style
	TopInfoStyle         lipgloss.Style
	TopStatsAddedStyle   lipgloss.Style
	TopStatsDeletedStyle lipgloss.Style

	LineNumberStyle lipgloss.Style

	DiffCtxGutter lipgloss.Style

	DiffAddLineStyle lipgloss.Style
	DiffDelLineStyle lipgloss.Style

	CursorNormalStyle lipgloss.Style
	CursorAddStyle    lipgloss.Style
	CursorDelStyle    lipgloss.Style

	CommentStyle lipgloss.Style

	SearchMatchStyle lipgloss.Style

	StatusBarStyle      lipgloss.Style
	StatusKeyStyle      lipgloss.Style
	StatusRepoStyle     lipgloss.Style
	StatusBranchStyle   lipgloss.Style
	StatusAddedStyle    lipgloss.Style
	StatusDeletedStyle  lipgloss.Style
	StatusDividerStyle  lipgloss.Style
	StatusNotifyStyle   lipgloss.Style
	StatusApprovedStyle lipgloss.Style
	StatusChangedStyle  lipgloss.Style
	StatusViewedStyle   lipgloss.Style

	HelpDrawerStyle lipgloss.Style
	HelpTextStyle   lipgloss.Style

	EmptyLogoStyle   lipgloss.Style
	EmptyDescStyle   lipgloss.Style
	EmptyStatusStyle lipgloss.Style

	OverlayStyle lipgloss.Style
	OverlayTitle lipgloss.Style
)

func InitStyles(t Theme) {
	PaneStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderNormal).
		Background(t.CanvasBg).
		Padding(0, 1)

	FocusedPaneStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocused).
		Background(t.CanvasBg).
		Padding(0, 1)

	TopBarStyle = lipgloss.NewStyle().
		Background(t.TopBarBg).
		Foreground(t.TopBarFg).
		Height(1).
		MaxHeight(1)

	TopInfoStyle = lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1).
		Background(t.TopBarBg)

	TopStatsAddedStyle = lipgloss.NewStyle().
		Foreground(t.StatsAdded).
		Background(t.TopBarBg).
		PaddingLeft(1)

	TopStatsDeletedStyle = lipgloss.NewStyle().
		Foreground(t.StatsDeleted).
		Background(t.TopBarBg).
		PaddingLeft(1).
		PaddingRight(1)

	LineNumberStyle = lipgloss.NewStyle().
		Foreground(t.LineNumberFg).
		Background(t.CanvasBg).
		Width(4).
		Align(lipgloss.Right).
		MarginRight(1)

	DiffCtxGutter = lipgloss.NewStyle().Foreground(t.GutterCtx).Background(t.CanvasBg)

	DiffAddLineStyle = lipgloss.NewStyle().Background(t.AddBg)
	DiffDelLineStyle = lipgloss.NewStyle().Background(t.DelBg)

	CursorNormalStyle = lipgloss.NewStyle().
		Background(t.CursorCtxBg).
		Foreground(t.CursorCtxFg)
	CursorAddStyle = lipgloss.NewStyle().
		Background(t.CursorAddBg).
		Foreground(t.CursorAddFg)
	CursorDelStyle = lipgloss.NewStyle().
		Background(t.CursorDelBg).
		Foreground(t.CursorDelFg)

	CommentStyle = lipgloss.NewStyle().
		Foreground(t.CommentFg).
		Background(t.CommentBg).
		Italic(true)

	SearchMatchStyle = lipgloss.NewStyle().
		Background(t.SearchBg).
		Foreground(t.SearchFg)

	StatusBarStyle = lipgloss.NewStyle().
		Background(t.StatusBarBg).
		Foreground(t.StatusBarFg).
		Height(1).
		MaxHeight(1)

	StatusKeyStyle = lipgloss.NewStyle().
		Foreground(t.DimText).
		Background(t.StatusBarBg).
		Padding(0, 1)

	StatusRepoStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.AccentText).
		Padding(0, 1)

	StatusBranchStyle = lipgloss.NewStyle().
		Foreground(t.AccentText2).
		Padding(0, 1)

	StatusAddedStyle = lipgloss.NewStyle().
		Foreground(t.StatsAdded).
		Padding(0, 1)

	StatusDeletedStyle = lipgloss.NewStyle().
		Foreground(t.StatsDeleted).
		Padding(0, 1)

	StatusDividerStyle = lipgloss.NewStyle().
		Foreground(t.DimText).
		Padding(0, 1)

	StatusNotifyStyle = lipgloss.NewStyle().
		Foreground(t.AccentText).
		Background(t.StatusBarBg).
		Padding(0, 1)

	StatusApprovedStyle = lipgloss.NewStyle().
		Foreground(t.StatusApproved).
		Padding(0, 1)

	StatusChangedStyle = lipgloss.NewStyle().
		Foreground(t.StatusChanged).
		Padding(0, 1)

	StatusViewedStyle = lipgloss.NewStyle().
		Foreground(t.StatusViewed).
		Padding(0, 1)

	HelpDrawerStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(t.BorderNormal).
		Background(t.TopBarBg).
		Padding(1, 2)

	HelpTextStyle = lipgloss.NewStyle().
		Foreground(t.DimText).
		Background(t.TopBarBg)

	EmptyLogoStyle = lipgloss.NewStyle().
		Foreground(t.AccentText).
		Bold(true).
		MarginBottom(1)

	EmptyDescStyle = lipgloss.NewStyle().
		Foreground(t.DimText).
		MarginBottom(1)

	EmptyStatusStyle = lipgloss.NewStyle().
		Foreground(t.DimText).
		MarginBottom(2)

	OverlayStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocused).
		Padding(1, 2).
		Background(t.TopBarBg)

	OverlayTitle = lipgloss.NewStyle().
		Foreground(t.BrightText).
		Background(t.TopBarBg).
		Bold(true).
		MarginBottom(1)
}
