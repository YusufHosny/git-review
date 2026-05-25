package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type splitLine struct {
	leftNum   int
	leftLine  string
	rightNum  int
	rightLine string
	kind      string // "context", "add", "del", "change", "hunk"
}

func parseSplitLines(diffLines []string) []splitLine {
	var result []splitLine
	var delBuf []string
	var addBuf []string

	leftLine := 1
	rightLine := 1

	flush := func() {
		maxLen := max(len(delBuf), len(addBuf))
		for i := range maxLen {
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
				leftNum:   ln,
				leftLine:  left,
				rightNum:  rn,
				rightLine: right,
				kind:      kind,
			})
		}
		delBuf = delBuf[:0]
		addBuf = addBuf[:0]
	}

	for _, raw := range diffLines {
		clean := strings.TrimRight(stripAnsi(raw), "\r")

		if isDiffMetadata(clean) {
			flush()
			if strings.HasPrefix(clean, "@@") {
				result = append(result, splitLine{leftLine: clean, rightLine: clean, kind: "hunk"})
			}
			continue
		}

		if strings.HasPrefix(clean, "-") {
			delBuf = append(delBuf, expandTabs(clean[1:]))
		} else if strings.HasPrefix(clean, "+") {
			addBuf = append(addBuf, expandTabs(clean[1:]))
		} else {
			flush()
			content := clean
			if len(content) > 0 {
				content = expandTabs(content[1:])
			}
			result = append(result, splitLine{
				leftNum:   leftLine,
				leftLine:  content,
				rightNum:  rightLine,
				rightLine: content,
				kind:      "context",
			})
			leftLine++
			rightLine++
		}
	}
	flush()
	return result
}

func (m Model) renderSplitDiff(contentHeight int) string {
	paneW := m.diffViewport.Width - 4
	colW := max((paneW-3)/2, 10)
	numW := 5

	splitLines := parseSplitLines(m.diffLines)

	start := m.splitOffset
	if start >= len(splitLines) {
		start = 0
	}
	end := min(start+m.diffViewport.Height-1, len(splitLines))

	var sb strings.Builder

	addBg := m.activeTheme.AddBg
	delBg := m.activeTheme.DelBg
	hunkFg := m.activeTheme.CommentFg
	numStyle := lipgloss.NewStyle().Foreground(m.activeTheme.LineNumberFg)

	for _, sl := range splitLines[start:end] {
		if sl.kind == "hunk" {
			sb.WriteString(lipgloss.NewStyle().Foreground(hunkFg).Render(ansi.Truncate(sl.leftLine, paneW, "")) + "\n")
			continue
		}

		contentW := max(colW-numW-1, 1)

		leftNum := strings.Repeat(" ", numW)
		if sl.leftNum > 0 {
			leftNum = fmt.Sprintf("%*d", numW, sl.leftNum)
		}
		rightNum := strings.Repeat(" ", numW)
		if sl.rightNum > 0 {
			rightNum = fmt.Sprintf("%*d", numW, sl.rightNum)
		}

		leftContent := padRight(ansi.Truncate(sl.leftLine, contentW, ""), contentW)
		rightContent := padRight(ansi.Truncate(sl.rightLine, contentW, ""), contentW)

		var leftStr, rightStr string
		switch sl.kind {
		case "del":
			leftStr = numStyle.Render(leftNum) + " " + lipgloss.NewStyle().Background(delBg).Render(leftContent)
			rightStr = strings.Repeat(" ", numW+1+contentW)
		case "add":
			leftStr = strings.Repeat(" ", numW+1+contentW)
			rightStr = numStyle.Render(rightNum) + " " + lipgloss.NewStyle().Background(addBg).Render(rightContent)
		case "change":
			leftStr = numStyle.Render(leftNum) + " " + lipgloss.NewStyle().Background(delBg).Render(leftContent)
			rightStr = numStyle.Render(rightNum) + " " + lipgloss.NewStyle().Background(addBg).Render(rightContent)
		default:
			leftStr = numStyle.Render(leftNum) + " " + leftContent
			rightStr = numStyle.Render(rightNum) + " " + rightContent
		}

		divider := lipgloss.NewStyle().Foreground(m.activeTheme.BorderNormal).Render(" │ ")
		sb.WriteString(leftStr + divider + rightStr + "\n")
	}

	content := strings.TrimRight(sb.String(), "\n")
	if canvasBg := ansiColorBg(m.activeTheme.CanvasBg); canvasBg != "" {
		content = injectBgAfterResets(content, canvasBg)
	}

	return m.renderDiffPane("\n"+content, contentHeight)
}

func padRight(s string, width int) string {
	if w := lipgloss.Width(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}
