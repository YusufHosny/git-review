package ui

import "github.com/charmbracelet/lipgloss"

// Theme holds all color tokens for a visual theme.
type Theme struct {
	Name string

	// Diff backgrounds
	AddBg lipgloss.Color
	DelBg lipgloss.Color

	// Cursor line backgrounds
	CursorAddBg  lipgloss.Color
	CursorDelBg  lipgloss.Color
	CursorCtxBg  lipgloss.Color
	CursorAddFg  lipgloss.Color
	CursorDelFg  lipgloss.Color
	CursorCtxFg  lipgloss.Color

	// Gutter / diff markers
	GutterAdd lipgloss.Color
	GutterDel lipgloss.Color
	GutterCtx lipgloss.Color

	// Review status colors in the tree
	StatusApproved lipgloss.Color
	StatusChanged  lipgloss.Color
	StatusViewed   lipgloss.Color
	StatusNew      lipgloss.Color

	// Comment ghost lines
	CommentFg lipgloss.Color
	CommentBg lipgloss.Color

	// Search highlight
	SearchBg lipgloss.Color
	SearchFg lipgloss.Color

	// Borders / UI chrome
	BorderNormal  lipgloss.Color
	BorderFocused lipgloss.Color
	TopBarBg      lipgloss.Color
	TopBarFg      lipgloss.Color
	StatusBarBg   lipgloss.Color
	StatusBarFg   lipgloss.Color

	// Text
	DimText     lipgloss.Color
	NormalText  lipgloss.Color
	BrightText  lipgloss.Color
	AccentText  lipgloss.Color
	AccentText2 lipgloss.Color

	// Stats
	StatsAdded   lipgloss.Color
	StatsDeleted lipgloss.Color

	// Line numbers
	LineNumberFg lipgloss.Color

	// Syntax highlight theme name (for chroma)
	ChromaTheme string
}

var Themes = []Theme{
	nordTheme,
	draculaTheme,
	catppuccinTheme,
	gruvboxTheme,
	tokyoNightTheme,
	rosePineTheme,
	oneDarkTheme,
	solarizedTheme,
	lightTheme,
}

var nordTheme = Theme{
	Name:          "dark",
	AddBg:         "#1A251E",
	DelBg:         "#2D1A1A",
	CursorAddBg:   "#A3E4D7",
	CursorDelBg:   "#F5B7B1",
	CursorCtxBg:   "#434C5E",
	CursorAddFg:   "#1A251E",
	CursorDelFg:   "#2D1A1A",
	CursorCtxFg:   "#ECEFF4",
	GutterAdd:     "#A3BE8C",
	GutterDel:     "#BF616A",
	GutterCtx:     "#4C566A",
	StatusApproved: "#A3BE8C",
	StatusChanged:  "#BF616A",
	StatusViewed:   "#EBCB8B",
	StatusNew:      "#D8DEE9",
	CommentFg:     "#81A1C1",
	CommentBg:     "#2E3440",
	SearchBg:      "#EBCB8B",
	SearchFg:      "#2E3440",
	BorderNormal:  "#4C566A",
	BorderFocused: "#81A1C1",
	TopBarBg:      "#2E3440",
	TopBarFg:      "#D8DEE9",
	StatusBarBg:   "#2E3440",
	StatusBarFg:   "#D8DEE9",
	DimText:       "#4C566A",
	NormalText:    "#D8DEE9",
	BrightText:    "#ECEFF4",
	AccentText:    "#7aa2f7",
	AccentText2:   "#bb9af7",
	StatsAdded:    "#A3BE8C",
	StatsDeleted:  "#BF616A",
	LineNumberFg:  "#4C566A",
	ChromaTheme:   "nord",
}

var draculaTheme = Theme{
	Name:          "dracula",
	AddBg:         "#1a2a1f",
	DelBg:         "#2a1a1f",
	CursorAddBg:   "#50fa7b",
	CursorDelBg:   "#ff5555",
	CursorCtxBg:   "#44475a",
	CursorAddFg:   "#282a36",
	CursorDelFg:   "#282a36",
	CursorCtxFg:   "#f8f8f2",
	GutterAdd:     "#50fa7b",
	GutterDel:     "#ff5555",
	GutterCtx:     "#6272a4",
	StatusApproved: "#50fa7b",
	StatusChanged:  "#ff5555",
	StatusViewed:   "#f1fa8c",
	StatusNew:      "#f8f8f2",
	CommentFg:     "#8be9fd",
	CommentBg:     "#282a36",
	SearchBg:      "#f1fa8c",
	SearchFg:      "#282a36",
	BorderNormal:  "#6272a4",
	BorderFocused: "#bd93f9",
	TopBarBg:      "#282a36",
	TopBarFg:      "#f8f8f2",
	StatusBarBg:   "#282a36",
	StatusBarFg:   "#f8f8f2",
	DimText:       "#6272a4",
	NormalText:    "#f8f8f2",
	BrightText:    "#ffffff",
	AccentText:    "#bd93f9",
	AccentText2:   "#ff79c6",
	StatsAdded:    "#50fa7b",
	StatsDeleted:  "#ff5555",
	LineNumberFg:  "#6272a4",
	ChromaTheme:   "dracula",
}

var catppuccinTheme = Theme{
	Name:          "catppuccin",
	AddBg:         "#1a2420",
	DelBg:         "#2a1a1e",
	CursorAddBg:   "#a6e3a1",
	CursorDelBg:   "#f38ba8",
	CursorCtxBg:   "#45475a",
	CursorAddFg:   "#1e1e2e",
	CursorDelFg:   "#1e1e2e",
	CursorCtxFg:   "#cdd6f4",
	GutterAdd:     "#a6e3a1",
	GutterDel:     "#f38ba8",
	GutterCtx:     "#585b70",
	StatusApproved: "#a6e3a1",
	StatusChanged:  "#f38ba8",
	StatusViewed:   "#f9e2af",
	StatusNew:      "#cdd6f4",
	CommentFg:     "#89b4fa",
	CommentBg:     "#1e1e2e",
	SearchBg:      "#f9e2af",
	SearchFg:      "#1e1e2e",
	BorderNormal:  "#585b70",
	BorderFocused: "#89b4fa",
	TopBarBg:      "#181825",
	TopBarFg:      "#cdd6f4",
	StatusBarBg:   "#181825",
	StatusBarFg:   "#cdd6f4",
	DimText:       "#585b70",
	NormalText:    "#cdd6f4",
	BrightText:    "#ffffff",
	AccentText:    "#89b4fa",
	AccentText2:   "#cba6f7",
	StatsAdded:    "#a6e3a1",
	StatsDeleted:  "#f38ba8",
	LineNumberFg:  "#585b70",
	ChromaTheme:   "catppuccin-mocha",
}

var gruvboxTheme = Theme{
	Name:          "gruvbox",
	AddBg:         "#1d2b1f",
	DelBg:         "#2b1d1d",
	CursorAddBg:   "#b8bb26",
	CursorDelBg:   "#fb4934",
	CursorCtxBg:   "#504945",
	CursorAddFg:   "#282828",
	CursorDelFg:   "#282828",
	CursorCtxFg:   "#ebdbb2",
	GutterAdd:     "#b8bb26",
	GutterDel:     "#fb4934",
	GutterCtx:     "#665c54",
	StatusApproved: "#b8bb26",
	StatusChanged:  "#fb4934",
	StatusViewed:   "#fabd2f",
	StatusNew:      "#ebdbb2",
	CommentFg:     "#83a598",
	CommentBg:     "#282828",
	SearchBg:      "#fabd2f",
	SearchFg:      "#282828",
	BorderNormal:  "#504945",
	BorderFocused: "#83a598",
	TopBarBg:      "#1d2021",
	TopBarFg:      "#ebdbb2",
	StatusBarBg:   "#1d2021",
	StatusBarFg:   "#ebdbb2",
	DimText:       "#665c54",
	NormalText:    "#ebdbb2",
	BrightText:    "#fbf1c7",
	AccentText:    "#83a598",
	AccentText2:   "#d3869b",
	StatsAdded:    "#b8bb26",
	StatsDeleted:  "#fb4934",
	LineNumberFg:  "#665c54",
	ChromaTheme:   "gruvbox",
}

var tokyoNightTheme = Theme{
	Name:           "tokyo-night",
	AddBg:          "#1a2a1f",
	DelBg:          "#2a1825",
	CursorAddBg:    "#9ece6a",
	CursorDelBg:    "#f7768e",
	CursorCtxBg:    "#283457",
	CursorAddFg:    "#1a1b26",
	CursorDelFg:    "#1a1b26",
	CursorCtxFg:    "#c0caf5",
	GutterAdd:      "#9ece6a",
	GutterDel:      "#f7768e",
	GutterCtx:      "#3b4261",
	StatusApproved: "#9ece6a",
	StatusChanged:  "#f7768e",
	StatusViewed:   "#e0af68",
	StatusNew:      "#c0caf5",
	CommentFg:      "#7aa2f7",
	CommentBg:      "#1a1b26",
	SearchBg:       "#e0af68",
	SearchFg:       "#1a1b26",
	BorderNormal:   "#3b4261",
	BorderFocused:  "#7aa2f7",
	TopBarBg:       "#16161e",
	TopBarFg:       "#c0caf5",
	StatusBarBg:    "#16161e",
	StatusBarFg:    "#c0caf5",
	DimText:        "#3b4261",
	NormalText:     "#c0caf5",
	BrightText:     "#ffffff",
	AccentText:     "#7aa2f7",
	AccentText2:    "#bb9af7",
	StatsAdded:     "#9ece6a",
	StatsDeleted:   "#f7768e",
	LineNumberFg:   "#3b4261",
	ChromaTheme:    "monokai",
}

var rosePineTheme = Theme{
	Name:           "rose-pine",
	AddBg:          "#1a2226",
	DelBg:          "#261a22",
	CursorAddBg:    "#31748f",
	CursorDelBg:    "#eb6f92",
	CursorCtxBg:    "#26233a",
	CursorAddFg:    "#e0def4",
	CursorDelFg:    "#e0def4",
	CursorCtxFg:    "#e0def4",
	GutterAdd:      "#31748f",
	GutterDel:      "#eb6f92",
	GutterCtx:      "#403d52",
	StatusApproved: "#31748f",
	StatusChanged:  "#eb6f92",
	StatusViewed:   "#f6c177",
	StatusNew:      "#e0def4",
	CommentFg:      "#c4a7e7",
	CommentBg:      "#191724",
	SearchBg:       "#f6c177",
	SearchFg:       "#191724",
	BorderNormal:   "#403d52",
	BorderFocused:  "#c4a7e7",
	TopBarBg:       "#1f1d2e",
	TopBarFg:       "#e0def4",
	StatusBarBg:    "#1f1d2e",
	StatusBarFg:    "#e0def4",
	DimText:        "#403d52",
	NormalText:     "#e0def4",
	BrightText:     "#ffffff",
	AccentText:     "#c4a7e7",
	AccentText2:    "#ebbcba",
	StatsAdded:     "#31748f",
	StatsDeleted:   "#eb6f92",
	LineNumberFg:   "#403d52",
	ChromaTheme:    "rose-pine",
}

var oneDarkTheme = Theme{
	Name:           "one-dark",
	AddBg:          "#1e2a1e",
	DelBg:          "#2a1e1e",
	CursorAddBg:    "#98c379",
	CursorDelBg:    "#e06c75",
	CursorCtxBg:    "#3e4451",
	CursorAddFg:    "#282c34",
	CursorDelFg:    "#282c34",
	CursorCtxFg:    "#abb2bf",
	GutterAdd:      "#98c379",
	GutterDel:      "#e06c75",
	GutterCtx:      "#4b5263",
	StatusApproved: "#98c379",
	StatusChanged:  "#e06c75",
	StatusViewed:   "#e5c07b",
	StatusNew:      "#abb2bf",
	CommentFg:      "#61afef",
	CommentBg:      "#282c34",
	SearchBg:       "#e5c07b",
	SearchFg:       "#282c34",
	BorderNormal:   "#4b5263",
	BorderFocused:  "#61afef",
	TopBarBg:       "#21252b",
	TopBarFg:       "#abb2bf",
	StatusBarBg:    "#21252b",
	StatusBarFg:    "#abb2bf",
	DimText:        "#4b5263",
	NormalText:     "#abb2bf",
	BrightText:     "#ffffff",
	AccentText:     "#61afef",
	AccentText2:    "#c678dd",
	StatsAdded:     "#98c379",
	StatsDeleted:   "#e06c75",
	LineNumberFg:   "#4b5263",
	ChromaTheme:    "onedark",
}

var solarizedTheme = Theme{
	Name:           "solarized",
	AddBg:          "#002b1f",
	DelBg:          "#2b0a00",
	CursorAddBg:    "#859900",
	CursorDelBg:    "#dc322f",
	CursorCtxBg:    "#073642",
	CursorAddFg:    "#002b36",
	CursorDelFg:    "#002b36",
	CursorCtxFg:    "#839496",
	GutterAdd:      "#859900",
	GutterDel:      "#dc322f",
	GutterCtx:      "#586e75",
	StatusApproved: "#859900",
	StatusChanged:  "#dc322f",
	StatusViewed:   "#b58900",
	StatusNew:      "#839496",
	CommentFg:      "#268bd2",
	CommentBg:      "#002b36",
	SearchBg:       "#b58900",
	SearchFg:       "#002b36",
	BorderNormal:   "#073642",
	BorderFocused:  "#268bd2",
	TopBarBg:       "#002b36",
	TopBarFg:       "#839496",
	StatusBarBg:    "#002b36",
	StatusBarFg:    "#839496",
	DimText:        "#586e75",
	NormalText:     "#839496",
	BrightText:     "#93a1a1",
	AccentText:     "#268bd2",
	AccentText2:    "#6c71c4",
	StatsAdded:     "#859900",
	StatsDeleted:   "#dc322f",
	LineNumberFg:   "#586e75",
	ChromaTheme:    "solarized-dark",
}

var lightTheme = Theme{
	Name:          "light",
	AddBg:         "#e6f4ea",
	DelBg:         "#fce8e6",
	CursorAddBg:   "#137333",
	CursorDelBg:   "#a50e0e",
	CursorCtxBg:   "#dadce0",
	CursorAddFg:   "#ffffff",
	CursorDelFg:   "#ffffff",
	CursorCtxFg:   "#202124",
	GutterAdd:     "#137333",
	GutterDel:     "#a50e0e",
	GutterCtx:     "#80868b",
	StatusApproved: "#137333",
	StatusChanged:  "#a50e0e",
	StatusViewed:   "#b06000",
	StatusNew:      "#202124",
	CommentFg:     "#1a73e8",
	CommentBg:     "#f8f9fa",
	SearchBg:      "#fef08a",
	SearchFg:      "#202124",
	BorderNormal:  "#dadce0",
	BorderFocused: "#1a73e8",
	TopBarBg:      "#f8f9fa",
	TopBarFg:      "#202124",
	StatusBarBg:   "#f8f9fa",
	StatusBarFg:   "#202124",
	DimText:       "#80868b",
	NormalText:    "#202124",
	BrightText:    "#000000",
	AccentText:    "#1a73e8",
	AccentText2:   "#7c4dff",
	StatsAdded:    "#137333",
	StatsDeleted:  "#a50e0e",
	LineNumberFg:  "#80868b",
	ChromaTheme:   "github",
}

// === Dynamic styles set by InitStyles ===

var (
	PaneStyle        lipgloss.Style
	FocusedPaneStyle lipgloss.Style

	TopBarStyle          lipgloss.Style
	TopInfoStyle         lipgloss.Style
	TopStatsAddedStyle   lipgloss.Style
	TopStatsDeletedStyle lipgloss.Style

	DirectoryStyle lipgloss.Style
	FileStyle      lipgloss.Style

	DiffStyle       lipgloss.Style
	LineNumberStyle lipgloss.Style

	DiffAddGutter lipgloss.Style
	DiffDelGutter lipgloss.Style
	DiffCtxGutter lipgloss.Style

	DiffAddLineStyle lipgloss.Style
	DiffDelLineStyle lipgloss.Style

	CursorNormalStyle lipgloss.Style
	CursorAddStyle    lipgloss.Style
	CursorDelStyle    lipgloss.Style

	CommentStyle lipgloss.Style

	SearchMatchStyle lipgloss.Style

	StatusBarStyle     lipgloss.Style
	StatusKeyStyle     lipgloss.Style
	StatusRepoStyle    lipgloss.Style
	StatusBranchStyle  lipgloss.Style
	StatusAddedStyle   lipgloss.Style
	StatusDeletedStyle lipgloss.Style
	StatusDividerStyle lipgloss.Style
	StatusNotifyStyle  lipgloss.Style
	StatusApprovedStyle  lipgloss.Style
	StatusChangedStyle   lipgloss.Style
	StatusViewedStyle    lipgloss.Style

	HelpDrawerStyle lipgloss.Style
	HelpTextStyle   lipgloss.Style

	EmptyLogoStyle   lipgloss.Style
	EmptyDescStyle   lipgloss.Style
	EmptyStatusStyle lipgloss.Style
	EmptyHeaderStyle lipgloss.Style
	EmptyCodeStyle   lipgloss.Style

	OverlayStyle  lipgloss.Style
	OverlayTitle  lipgloss.Style

	ColorText lipgloss.Color
)

func InitStyles(t Theme) {
	ColorText = t.NormalText

	PaneStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderNormal).
		Padding(0, 1)

	FocusedPaneStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocused).
		Padding(0, 1)

	TopBarStyle = lipgloss.NewStyle().
		Background(t.TopBarBg).
		Foreground(t.TopBarFg).
		Height(1)

	TopInfoStyle = lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1)

	TopStatsAddedStyle = lipgloss.NewStyle().
		Foreground(t.StatsAdded).
		PaddingLeft(1)

	TopStatsDeletedStyle = lipgloss.NewStyle().
		Foreground(t.StatsDeleted).
		PaddingLeft(1).
		PaddingRight(1)

	DirectoryStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
	FileStyle = lipgloss.NewStyle().Foreground(t.NormalText)

	DiffStyle = lipgloss.NewStyle().Padding(0, 0)
	LineNumberStyle = lipgloss.NewStyle().
		Foreground(t.LineNumberFg).
		Width(4).
		Align(lipgloss.Right).
		MarginRight(1)

	DiffAddGutter = lipgloss.NewStyle().Foreground(t.GutterAdd).Bold(true)
	DiffDelGutter = lipgloss.NewStyle().Foreground(t.GutterDel).Bold(true)
	DiffCtxGutter = lipgloss.NewStyle().Foreground(t.GutterCtx)

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
		Height(1)

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
		Padding(1, 2)

	HelpTextStyle = lipgloss.NewStyle().
		Foreground(t.DimText).
		MarginRight(2)

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

	EmptyHeaderStyle = lipgloss.NewStyle().
		Foreground(t.DimText).
		Bold(true).
		MarginBottom(1)

	EmptyCodeStyle = lipgloss.NewStyle().Foreground(t.DimText)

	OverlayStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocused).
		Padding(1, 2).
		Background(t.TopBarBg)

	OverlayTitle = lipgloss.NewStyle().
		Foreground(t.BrightText).
		Bold(true).
		MarginBottom(1)
}
