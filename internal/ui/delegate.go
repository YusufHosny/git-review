package ui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/yusuf/git-review/internal/review"
	"github.com/yusuf/git-review/internal/tree"
)

type TreeDelegate struct {
	Focused      bool
	FileStatuses map[string]review.FileStatus
	ActiveTheme  Theme
}

func (d TreeDelegate) Height() int  { return 1 }
func (d TreeDelegate) Spacing() int { return 0 }

func (d TreeDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

func (d TreeDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(tree.TreeItem)
	if !ok {
		return
	}

	title := i.Title()
	maxWidth := m.Width() - 2
	if maxWidth < 4 {
		maxWidth = 4
	}
	title = ansi.Truncate(title, maxWidth, "…")

	isSelected := index == m.Index()
	status := d.FileStatuses[i.FullPath]

	if isSelected {
		style := lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Foreground(lipgloss.Color("255")).
			Bold(true).
			Width(maxWidth)
		if !d.Focused {
			style = style.Foreground(lipgloss.Color("245"))
		}
		fmt.Fprint(w, style.Render(title))
		return
	}

	// Apply per-status color to the status icon part
	var fg lipgloss.Color
	if i.IsDir {
		fg = lipgloss.Color("99")
	} else {
		switch status {
		case review.StatusApproved:
			fg = d.ActiveTheme.StatusApproved
		case review.StatusChanged:
			fg = d.ActiveTheme.StatusChanged
		case review.StatusViewed:
			fg = d.ActiveTheme.StatusViewed
		default:
			fg = d.ActiveTheme.NormalText
		}
	}

	style := lipgloss.NewStyle().
		Foreground(fg).
		Width(maxWidth)
	fmt.Fprint(w, style.Render(title))
}
