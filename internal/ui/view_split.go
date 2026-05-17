package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type splitLine struct {
	leftNum  int
	leftLine string
	rightNum int
	rightLine string
	kind     string // "context", "add", "del", "hunk"
}

// parseSplitLines converts unified diff lines into side-by-side pairs.
func parseSplitLines(diffLines []string) []splitLine {
	var result []splitLine
	var delBuf []string
	var addBuf []string

	leftLine := 1
	rightLine := 1

	flush := func() {
		maxLen := len(delBuf)
		if len(addBuf) > maxLen {
			maxLen = len(addBuf)
		}
		for i := 0; i < maxLen; i++ {
			var left, right string
			var ln, rn int
			if i < len(delBuf) {
				left = delBuf[i]
				ln = leftLine
				leftLine++
			}
			if i < len(addBuf) {
				right = addBuf[i]
				rn = rightLine
				rightLine++
			}
			kind := "del"
			if left != "" && right != "" {
				kind = "change"
			} else if right != "" {
				kind = "add"
			}
			result = append(result, splitLine{
				leftNum:  ln,
				leftLine: left,
				rightNum: rn,
				rightLine: right,
				kind:     kind,
			})
		}
		delBuf = delBuf[:0]
		addBuf = addBuf[:0]
	}

	for _, raw := range diffLines {
		clean := stripAnsi(raw)
		clean = strings.TrimRight(clean, "\r")

		if isDiffMetadata(clean) {
			flush()
			if strings.HasPrefix(clean, "@@") {
				result = append(result, splitLine{leftLine: clean, rightLine: clean, kind: "hunk"})
			}
			continue
		}

		if strings.HasPrefix(clean, "-") {
			delBuf = append(delBuf, clean[1:])
		} else if strings.HasPrefix(clean, "+") {
			addBuf = append(addBuf, clean[1:])
		} else {
			flush()
			content := clean
			if len(content) > 0 {
				content = content[1:]
			}
			result = append(result, splitLine{
				leftNum:  leftLine,
				leftLine: content,
				rightNum: rightLine,
				rightLine: content,
				kind:     "context",
			})
			leftLine++
			rightLine++
		}
	}
	flush()
	return result
}

func (m Model) renderSplitDiff(contentHeight int) string {
	paneW := m.diffViewport.Width - 2
	colW := (paneW - 3) / 2 // 3 for the middle divider
	if colW < 10 {
		colW = 10
	}
	numW := 5

	splitLines := parseSplitLines(m.diffLines)

	// Apply scroll offset
	start := m.splitOffset
	if start >= len(splitLines) {
		start = 0
	}
	end := start + contentHeight
	if end > len(splitLines) {
		end = len(splitLines)
	}

	var sb strings.Builder

	addBg := m.activeTheme.AddBg
	delBg := m.activeTheme.DelBg
	hunkFg := m.activeTheme.CommentFg

	for _, sl := range splitLines[start:end] {
		switch sl.kind {
		case "hunk":
			hunkStyle := lipgloss.NewStyle().Foreground(hunkFg)
			full := hunkStyle.Render(ansi.Truncate(sl.leftLine, paneW, ""))
			sb.WriteString(full + "\n")
			continue
		}

		// Left column
		var leftNum string
		if sl.leftNum > 0 {
			leftNum = fmt.Sprintf("%*d", numW, sl.leftNum)
		} else {
			leftNum = strings.Repeat(" ", numW)
		}

		// Right column
		var rightNum string
		if sl.rightNum > 0 {
			rightNum = fmt.Sprintf("%*d", numW, sl.rightNum)
		} else {
			rightNum = strings.Repeat(" ", numW)
		}

		contentW := colW - numW - 1
		if contentW < 1 {
			contentW = 1
		}

		leftContent := ansi.Truncate(sl.leftLine, contentW, "")
		rightContent := ansi.Truncate(sl.rightLine, contentW, "")

		// Pad to content width
		leftContent = padRight(leftContent, contentW)
		rightContent = padRight(rightContent, contentW)

		numStyle := lipgloss.NewStyle().Foreground(m.activeTheme.LineNumberFg)

		var leftStr, rightStr string
		switch sl.kind {
		case "del":
			delStyle := lipgloss.NewStyle().Background(delBg)
			leftStr = numStyle.Render(leftNum) + " " + delStyle.Render(leftContent)
			rightStr = strings.Repeat(" ", numW+1+contentW)
		case "add":
			addStyle := lipgloss.NewStyle().Background(addBg)
			leftStr = strings.Repeat(" ", numW+1+contentW)
			rightStr = numStyle.Render(rightNum) + " " + addStyle.Render(rightContent)
		case "change":
			delStyle := lipgloss.NewStyle().Background(delBg)
			addStyle := lipgloss.NewStyle().Background(addBg)
			leftStr = numStyle.Render(leftNum) + " " + delStyle.Render(leftContent)
			rightStr = numStyle.Render(rightNum) + " " + addStyle.Render(rightContent)
		default: // context
			leftStr = numStyle.Render(leftNum) + " " + leftContent
			rightStr = numStyle.Render(rightNum) + " " + rightContent
		}

		divider := lipgloss.NewStyle().Foreground(m.activeTheme.BorderNormal).Render(" │ ")
		sb.WriteString(leftStr + divider + rightStr + "\n")
	}

	diffPaneStyle := PaneStyle
	if m.focus == FocusDiff {
		diffPaneStyle = FocusedPaneStyle
	}

	return diffPaneStyle.Copy().
		Width(m.diffViewport.Width - 2).
		Height(contentHeight).
		MaxHeight(contentHeight).
		Render("\n" + strings.TrimRight(sb.String(), "\n"))
}

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
