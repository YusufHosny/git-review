package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
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

	file := ""
	if m.selectedPath != "" {
		file = HelpTextStyle.Render("File: " + m.selectedPath)
	}
	linePreview := ""
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

	maxW := m.width - 8
	if maxW < 40 {
		maxW = 40
	}

	return OverlayStyle.Copy().Width(maxW).Render(inner)
}

func (m Model) renderConfirmOverlay() string {
	title := OverlayTitle.Render("Confirm")
	msg := lipgloss.NewStyle().
		Foreground(ColorText).
		Render(m.confirmMsg)

	hint := HelpTextStyle.Render("y confirm  n/esc cancel")

	inner := lipgloss.JoinVertical(lipgloss.Left,
		title,
		msg,
		"",
		hint,
	)

	maxW := m.width / 2
	if maxW < 40 {
		maxW = 40
	}

	return OverlayStyle.Copy().Width(maxW).Render(inner)
}

func (m Model) renderThemePickerOverlay() string {
	title := OverlayTitle.Render("Select Theme")
	hint := HelpTextStyle.Render("j/k move · enter apply · esc cancel")

	var rows []string
	for i, theme := range Themes {
		isCurrent := i == m.themePickerCursor

		// Color swatches — rendered in each theme's own colors
		addSwatch := lipgloss.NewStyle().Background(theme.StatusApproved).Render("  ")
		delSwatch := lipgloss.NewStyle().Background(theme.StatusChanged).Render("  ")
		accentSwatch := lipgloss.NewStyle().Background(theme.AccentText).Render("  ")
		bgSwatch := lipgloss.NewStyle().Background(theme.TopBarBg).Foreground(theme.DimText).Render("  ")

		swatches := addSwatch + " " + delSwatch + " " + accentSwatch + " " + bgSwatch

		namePad := 14
		name := theme.Name
		if len(name) < namePad {
			name += strings.Repeat(" ", namePad-len(name))
		}

		var row string
		if isCurrent {
			cursor := lipgloss.NewStyle().Foreground(m.activeTheme.AccentText).Bold(true).Render("▶ ")
			nameStyled := lipgloss.NewStyle().Foreground(m.activeTheme.BrightText).Bold(true).Render(name)
			row = cursor + nameStyled + " " + swatches
		} else {
			cursor := "  "
			nameStyled := lipgloss.NewStyle().Foreground(m.activeTheme.NormalText).Render(name)
			row = cursor + nameStyled + " " + swatches
		}
		rows = append(rows, row)
	}

	inner := lipgloss.JoinVertical(lipgloss.Left,
		append([]string{title, ""}, append(rows, "", hint)...)...,
	)

	return OverlayStyle.Copy().Render(inner)
}

// placeOverlay centers the overlay string on top of the base content.
func placeOverlay(base, overlay string, baseW, baseH int) string {
	overlayW := lipgloss.Width(overlay)
	overlayH := lipgloss.Height(overlay)

	x := (baseW - overlayW) / 2
	y := (baseH - overlayH) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	for i, ol := range overlayLines {
		targetRow := y + i
		if targetRow >= len(baseLines) {
			break
		}
		bl := baseLines[targetRow]
		// Pad base line if needed
		blW := lipgloss.Width(bl)
		if x > blW {
			bl += strings.Repeat(" ", x-blW)
		}
		// Replace portion of base line with overlay line
		newLine := ansiSafeReplace(bl, x, overlayW, ol)
		baseLines[targetRow] = newLine
	}

	return strings.Join(baseLines, "\n")
}

// ansiSafeReplace replaces characters in a line at a visual position.
// This is a simplified version that works well for ASCII/single-width chars.
func ansiSafeReplace(line string, x, w int, overlay string) string {
	// Strip any existing content and rebuild
	// For simplicity: truncate at x, add overlay, no right-side recovery
	// A full implementation would need ANSI-aware string manipulation
	stripped := StripAnsiSimple(line)
	lineW := len([]rune(stripped))

	if x >= lineW {
		return line + strings.Repeat(" ", x-lineW) + overlay
	}

	runes := []rune(stripped)
	left := string(runes[:x])
	rightStart := x + w
	right := ""
	if rightStart < lineW {
		right = string(runes[rightStart:])
	}
	return left + overlay + right
}

func StripAnsiSimple(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func trimLine(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
