package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type OverlayMode int

const (
	OverlayNone OverlayMode = iota
	OverlayCommentInput
	OverlayConfirm
	OverlayNotify
	OverlayThemePicker
)

func (m Model) renderOverlay() string {
	switch m.overlay {
	case OverlayCommentInput:
		return m.renderCommentOverlay()
	case OverlayConfirm:
		return m.renderConfirmOverlay()
	case OverlayThemePicker:
		return m.renderThemePickerOverlay()
	default:
		return ""
	}
}

func (m Model) renderCommentOverlay() string {
	title := OverlayTitle.Render("Add Comment")
	hint := HelpTextStyle.Render("ctrl+s save  esc cancel")

	var file, linePreview string
	if m.selectedPath != "" {
		file = HelpTextStyle.Render("File: " + m.selectedPath)
	}
	if m.commentLineContent != "" {
		linePreview = HelpTextStyle.Render("Line: " + trimLine(m.commentLineContent, 60))
	}

	inner := lipgloss.JoinVertical(lipgloss.Left,
		title,
		file,
		linePreview,
		"",
		m.commentInput.View(),
		"",
		hint,
	)

	return OverlayStyle.Width(max(m.width-8, 40)).Render(inner)
}

func (m Model) renderConfirmOverlay() string {
	inner := lipgloss.JoinVertical(lipgloss.Left,
		OverlayTitle.Render("Confirm"),
		lipgloss.NewStyle().Foreground(ColorText).Render(m.confirmMsg),
		"",
		HelpTextStyle.Render("y confirm  n/esc cancel"),
	)
	return OverlayStyle.Width(max(m.width/2, 40)).Render(inner)
}

func (m Model) renderThemePickerOverlay() string {
	bg := m.activeTheme.TopBarBg
	bgSeq := ansiColorBg(bg)
	sep := bgSeq + " \x1b[0m"

	var rows []string
	for i, theme := range Themes {
		isCurrent := i == m.themePickerCursor

		swatches := lipgloss.NewStyle().Background(theme.StatusApproved).Render("  ") + sep +
			lipgloss.NewStyle().Background(theme.StatusChanged).Render("  ") + sep +
			lipgloss.NewStyle().Background(theme.AccentText).Render("  ") + sep +
			lipgloss.NewStyle().Background(theme.TopBarBg).Render("  ")

		namePad := 14
		name := theme.Name
		if len(name) < namePad {
			name += strings.Repeat(" ", namePad-len(name))
		}

		var row string
		if isCurrent {
			row = bgSeq + "\x1b[1m" + ansiColorFg(m.activeTheme.AccentText) + "> " +
				ansiColorFg(m.activeTheme.BrightText) + name + "\x1b[0m" +
				sep + swatches
		} else {
			row = bgSeq + ansiColorFg(m.activeTheme.NormalText) + "  " + name + "\x1b[0m" +
				sep + swatches
		}
		rows = append(rows, row)
	}

	allRows := append([]string{OverlayTitle.Render("Select Theme"), ""}, rows...)
	allRows = append(allRows, "", HelpTextStyle.Render("j/k move · enter apply · esc cancel"))
	return OverlayStyle.Render(strings.Join(allRows, "\n"))
}

func placeOverlay(base, overlay string, baseW, baseH int) string {
	overlayW := lipgloss.Width(overlay)
	overlayH := lipgloss.Height(overlay)

	x := max((baseW-overlayW)/2, 0)
	y := max((baseH-overlayH)/2, 0)

	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	for i, ol := range overlayLines {
		targetRow := y + i
		if targetRow >= len(baseLines) {
			break
		}
		baseLines[targetRow] = ansiSafeReplace(baseLines[targetRow], x, overlayW, ol)
	}

	return strings.Join(baseLines, "\n")
}

// ansiSafeReplace safely replaces a segment of an ANSI-colored string without stripping colors.
func ansiSafeReplace(line string, x, w int, overlay string) string {
	lineW := ansi.StringWidth(line)

	if x >= lineW {
		return line + strings.Repeat(" ", x-lineW) + overlay
	}

	left := ansi.Truncate(line, x, "")
	right := ""
	if rightStart := x + w; rightStart < lineW {
		right = ansi.TruncateLeft(line, rightStart, "")
	}
	return left + overlay + right
}

func trimLine(s string, max int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) > max {
		return string(runes[:max]) + "…"
	}
	return s
}
