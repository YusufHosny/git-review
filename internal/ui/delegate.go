package ui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/YusufHosny/git-review/internal/review"
	"github.com/YusufHosny/git-review/internal/tree"
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
	maxWidth = max(maxWidth, 4)
	title = ansi.Truncate(title, maxWidth, "…")

	isSelected := index == m.Index()
	status := d.FileStatuses[i.FullPath]

	if isSelected {
		bg := d.ActiveTheme.CursorCtxBg
		fg := d.ActiveTheme.BrightText
		if !d.Focused {
			bg = d.ActiveTheme.BorderNormal
			fg = d.ActiveTheme.NormalText
		}
		style := lipgloss.NewStyle().
			Background(bg).
			Foreground(fg).
			Bold(d.Focused).
			Width(maxWidth)
		fmt.Fprint(w, style.Render(title))
		return
	}

	var fg lipgloss.Color
	if i.IsDir {
		fg = d.ActiveTheme.AccentText2
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
		Background(d.ActiveTheme.CanvasBg).
		Width(maxWidth)
	fmt.Fprint(w, style.Render(title))
}
