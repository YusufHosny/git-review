package ui

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/yusuf/git-review/internal/review"
	"github.com/yusuf/git-review/internal/tree"
)

var hunkStartRe = regexp.MustCompile(`^@@ \-\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	topBar := m.renderTopBar()

	var bottomBar string
	if m.showHelp {
		bottomBar = m.renderHelpDrawer()
	} else {
		bottomBar = m.viewStatusBar()
	}

	contentHeight := m.height - lipgloss.Height(topBar) - lipgloss.Height(bottomBar)
	if contentHeight < 0 {
		contentHeight = 0
	}

	var mainContent string
	if len(m.fileList.Items()) == 0 {
		mainContent = m.renderEmptyState(m.width, contentHeight)
	} else {
		treeStyle := PaneStyle
		if m.focus == FocusTree {
			treeStyle = FocusedPaneStyle
		}

		treeView := treeStyle.Copy().
			Width(m.fileList.Width()).
			Height(contentHeight).
			MaxHeight(contentHeight).
			Render(m.fileList.View())

		var rightPaneView string
		selectedItem, ok := m.fileList.SelectedItem().(tree.TreeItem)

		if ok && selectedItem.IsDir {
			rightPaneView = m.renderEmptyState(m.diffViewport.Width, contentHeight)
		} else if m.splitView {
			rightPaneView = m.renderSplitDiff(contentHeight)
		} else {
			rightPaneView = m.renderUnifiedDiff(contentHeight)
		}

		mainContent = lipgloss.JoinHorizontal(lipgloss.Top, treeView, rightPaneView)
	}

	base := lipgloss.JoinVertical(lipgloss.Top, topBar, mainContent, bottomBar)

	// Render overlay on top
	if m.overlay != OverlayNone {
		overlayStr := m.renderOverlay()
		baseH := lipgloss.Height(base)
		return placeOverlay(base, overlayStr, m.width, baseH)
	}

	return base
}

func (m Model) renderUnifiedDiff(contentHeight int) string {
	var renderedDiff strings.Builder

	viewportHeight := contentHeight
	start := m.diffViewport.YOffset
	end := start + viewportHeight
	if end > len(m.diffLines) {
		end = len(m.diffLines)
	}

	maxLineWidth := m.diffViewport.Width - 7
	if maxLineWidth < 1 {
		maxLineWidth = 1
	}

	lineNumMode := m.cfg.UI.LineNumbers

	// Pre-scan from line 0 to 'start' so we know the real file line number
	// at the top of the viewport (hunk headers tell us where each hunk begins).
	fileLineNo := 0
	inHunk := false
	for i := 0; i < start && i < len(m.diffLines); i++ {
		cl := stripAnsi(m.diffLines[i])
		if matches := hunkStartRe.FindStringSubmatch(cl); len(matches) > 1 {
			fileLineNo, _ = strconv.Atoi(matches[1])
			inHunk = true
			continue
		}
		if !inHunk {
			continue
		}
		if strings.HasPrefix(cl, "+") || strings.HasPrefix(cl, " ") {
			fileLineNo++
		}
	}

	for i := start; i < end; i++ {
		rawLine := m.diffLines[i]
		cleanLine := stripAnsi(rawLine)

		if isDiffMetadata(cleanLine) {
			// Keep file line tracking up to date for hunk headers.
			if strings.HasPrefix(cleanLine, "@@") {
				if matches := hunkStartRe.FindStringSubmatch(cleanLine); len(matches) > 1 {
					fileLineNo, _ = strconv.Atoi(matches[1])
					inHunk = true
				}
				hunkStyle := lipgloss.NewStyle().
					Foreground(m.activeTheme.CommentFg).
					Background(m.activeTheme.TopBarBg)
				line := hunkStyle.Render(ansi.Truncate(cleanLine, maxLineWidth+6, ""))
				renderedDiff.WriteString(line + "\n")
			}
			if end < len(m.diffLines) {
				end++
			}
			continue
		}

		// Advance the real file line counter.
		var realLineNo int
		if inHunk {
			switch {
			case strings.HasPrefix(cleanLine, "+"), strings.HasPrefix(cleanLine, " "):
				realLineNo = fileLineNo
				fileLineNo++
			case strings.HasPrefix(cleanLine, "-"):
				realLineNo = 0 // deleted line has no new-file line number
			}
		}

		isAdd := strings.HasPrefix(cleanLine, "+")
		isDel := strings.HasPrefix(cleanLine, "-")

		codeContent := cleanLine
		if len(codeContent) > 0 && (isAdd || isDel || strings.HasPrefix(codeContent, " ")) {
			codeContent = codeContent[1:]
		}

		// Determine cursor / visual selection
		isCursor := false
		if m.focus == FocusDiff {
			if m.visualMode {
				minIdx, maxIdx := m.visualStart, m.diffCursor
				if minIdx > maxIdx {
					minIdx, maxIdx = maxIdx, minIdx
				}
				isCursor = (i >= minIdx && i <= maxIdx)
			} else {
				isCursor = (i == m.diffCursor)
			}
		}

		// Search highlight
		isSearchMatch := false
		for _, mi := range m.searchMatches {
			if mi == i {
				isSearchMatch = true
				break
			}
		}

		separator := "│"
		if isCursor {
			separator = "┃"
		}

		var gutterStr string
		if isAdd {
			gutterStr = DiffAddGutter.Render("+ " + separator + " ")
		} else if isDel {
			gutterStr = DiffDelGutter.Render("- " + separator + " ")
		} else {
			gutterStr = DiffCtxGutter.Render("  " + separator + " ")
		}

		// Line number
		var numStr string
		switch lineNumMode {
		case "absolute":
			if realLineNo > 0 {
				numStr = fmt.Sprintf("%d", realLineNo)
			}
		case "hybrid":
			// Cursor line: real file line; other lines: distance from cursor.
			if isCursor {
				if realLineNo > 0 {
					numStr = fmt.Sprintf("%d", realLineNo)
				}
			} else {
				dist := int(math.Abs(float64(i - m.diffCursor)))
				numStr = fmt.Sprintf("%d", dist)
			}
		case "relative":
			if isCursor {
				numStr = "0"
			} else {
				dist := int(math.Abs(float64(i - m.diffCursor)))
				numStr = fmt.Sprintf("%d", dist)
			}
		}

		lineNumRendered := ""
		if numStr != "" {
			lineNumRendered = LineNumberStyle.Render(numStr)
		}

		var line string
		if isCursor {
			fullStr := gutterStr + ansi.Truncate(codeContent, maxLineWidth-4, "")
			visibleLen := lipgloss.Width(fullStr)
			padLen := maxLineWidth - visibleLen
			if padLen > 0 {
				fullStr += strings.Repeat(" ", padLen)
			}
			if isAdd {
				line = CursorAddStyle.Copy().Width(maxLineWidth).Render(fullStr)
			} else if isDel {
				line = CursorDelStyle.Copy().Width(maxLineWidth).Render(fullStr)
			} else {
				line = CursorNormalStyle.Copy().Width(maxLineWidth).Render(fullStr)
			}
		} else {
			var hlCode string

			if i < len(m.diffHighlighted) {
				hlCode = m.diffHighlighted[i]
				hlCode = bgAnsiRe.ReplaceAllString(hlCode, "")
			} else {
				hlCode = codeContent
			}

			hlCode = ansi.Truncate(hlCode, maxLineWidth-4, "")

			if isSearchMatch && m.searchQuery != "" {
				hlCode = SearchMatchStyle.Render(codeContent)
				hlCode = ansi.Truncate(hlCode, maxLineWidth-4, "")
			} else if isAdd {
				hlCode = DiffAddLineStyle.Render(hlCode)
			} else if isDel {
				hlCode = DiffDelLineStyle.Render(hlCode)
			}

			line = gutterStr + hlCode
		}

		renderedDiff.WriteString(lineNumRendered + line + "\n")

		// Render comment ghost line if there's a comment on this diff line
		if c, ok := m.lineComments[i]; ok && m.cfg.UI.ShowCommentsInline {
			commentPrefix := "     ▶ "
			commentText := trimLine(c.Body, maxLineWidth-len(commentPrefix)-2)
			ghost := CommentStyle.Render(commentPrefix + commentText)
			renderedDiff.WriteString(ghost + "\n")
			if end < len(m.diffLines) {
				end++
			}
		}
	}

	// Show delta indicator if viewing "changed since approval" range
	var headerStr string
	if m.computedStatuses[m.selectedPath] == review.StatusChanged {
		headerStr = lipgloss.NewStyle().
			Foreground(m.activeTheme.StatusChanged).
			Bold(true).
			Render(" ↑ showing new changes since approval — press 'a' to approve again")
	}

	diffContentStr := "\n" + strings.TrimRight(renderedDiff.String(), "\n")
	if headerStr != "" {
		diffContentStr = headerStr + "\n" + diffContentStr
	}

	diffPaneStyle := PaneStyle
	if m.focus == FocusDiff {
		diffPaneStyle = FocusedPaneStyle
	}

	return diffPaneStyle.Copy().
		Width(m.diffViewport.Width - 2).
		Height(contentHeight).
		MaxHeight(contentHeight).
		Render(diffContentStr)
}

func (m Model) renderTopBar() string {
	// Left: repo, branch, range
	info := fmt.Sprintf(" %s  %s → %s", m.repoName, m.currentBranch, m.rangeLabel)
	leftSide := TopInfoStyle.Render(info)

	// Stats counts
	approved, changed, _, total := m.reviewSummary()
	reviewInfo := ""
	if total > 0 {
		reviewInfo = fmt.Sprintf(" ✓%d !%d ○%d/%d", approved, changed, total-approved-changed, total)
	}

	statsStr := ""
	if m.statsAdded > 0 || m.statsDeleted > 0 {
		statsStr = TopStatsAddedStyle.Render(fmt.Sprintf("+%d", m.statsAdded)) +
			TopStatsDeletedStyle.Render(fmt.Sprintf("-%d", m.statsDeleted))
	}

	themeLabel := StatusKeyStyle.Render("[" + m.activeTheme.Name + "]")

	// Right: current file + file stats
	rightSide := ""
	if sel, ok := m.fileList.SelectedItem().(tree.TreeItem); ok && !sel.IsDir {
		fileStats := ""
		if fs, ok := m.fileStats[sel.FullPath]; ok {
			fileStats = fmt.Sprintf(" +%d -%d", fs[0], fs[1])
		}

		status := m.computedStatuses[sel.FullPath]
		var statusLabel string
		switch status {
		case review.StatusApproved:
			statusLabel = StatusApprovedStyle.Render("[APPROVED]")
		case review.StatusChanged:
			statusLabel = StatusChangedStyle.Render("[CHANGED]")
		case review.StatusViewed:
			statusLabel = StatusViewedStyle.Render("[VIEWED]")
		}

		maxPathW := m.width - lipgloss.Width(leftSide) - lipgloss.Width(statsStr) - lipgloss.Width(reviewInfo) - lipgloss.Width(themeLabel) - 10
		if maxPathW < 10 {
			maxPathW = 10
		}
		truncPath := ansi.Truncate(sel.FullPath, maxPathW, "…")
		rightSide = truncPath + fileStats + " " + statusLabel
	}

	availWidth := m.width - lipgloss.Width(leftSide) - lipgloss.Width(statsStr) - lipgloss.Width(reviewInfo) - lipgloss.Width(themeLabel) - lipgloss.Width(rightSide)
	if availWidth < 0 {
		availWidth = 0
	}
	padding := strings.Repeat(" ", availWidth)

	finalBar := lipgloss.JoinHorizontal(lipgloss.Top,
		leftSide, statsStr, reviewInfo, themeLabel, padding, rightSide)

	return TopBarStyle.Width(m.width).Render(finalBar)
}

func (m Model) viewStatusBar() string {
	bg := m.activeTheme.StatusBarBg

	if m.searchMode {
		prompt := lipgloss.NewStyle().
			Foreground(m.activeTheme.AccentText).
			Background(bg).
			Padding(0, 1).
			Render("/")
		input := m.searchInput.View()
		matchInfo := ""
		if len(m.searchMatches) > 0 {
			idx := m.searchCursor + 1
			matchInfo = lipgloss.NewStyle().
				Foreground(m.activeTheme.DimText).
				Background(bg).
				Padding(0, 1).
				Render(fmt.Sprintf("[%d/%d]", idx, len(m.searchMatches)))
		}
		hint := StatusKeyStyle.Render("enter confirm  esc clear")
		avail := m.width - lipgloss.Width(prompt) - lipgloss.Width(input) - lipgloss.Width(matchInfo) - lipgloss.Width(hint)
		if avail < 0 {
			avail = 0
		}
		pad := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", avail))
		return StatusBarStyle.Width(m.width).Render(
			lipgloss.JoinHorizontal(lipgloss.Top, prompt, input, matchInfo, pad, hint),
		)
	}

	if m.statusNotify != "" {
		notify := StatusNotifyStyle.Render(m.statusNotify)
		avail := m.width - lipgloss.Width(notify)
		if avail < 0 {
			avail = 0
		}
		pad := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", avail))
		return StatusBarStyle.Width(m.width).Render(
			lipgloss.JoinHorizontal(lipgloss.Top, notify, pad),
		)
	}

	shortcuts := StatusKeyStyle.Render("?help  q quit  Tab switch  a approve  c comment  E export  s split  t theme  F fzf")
	avail := m.width - lipgloss.Width(shortcuts)
	if avail < 0 {
		avail = 0
	}
	pad := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", avail))
	return StatusBarStyle.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Top, shortcuts, pad),
	)
}

func (m Model) renderHelpDrawer() string {
	col1 := lipgloss.JoinVertical(lipgloss.Left,
		HelpTextStyle.Render("↑/k    Move Up"),
		HelpTextStyle.Render("↓/j    Move Down"),
		HelpTextStyle.Render("gg/G   Top/Bottom"),
		HelpTextStyle.Render("C-d/u  Page ½ Dn/Up"),
		HelpTextStyle.Render("H/M/L  High/Mid/Low"),
	)
	col2 := lipgloss.JoinVertical(lipgloss.Left,
		HelpTextStyle.Render("h/l    Switch Panel"),
		HelpTextStyle.Render("Tab    Toggle Focus"),
		HelpTextStyle.Render("]c/[c  Next/Prev Hunk"),
		HelpTextStyle.Render("zz/zt  Center/Top"),
		HelpTextStyle.Render("e      Edit File"),
	)
	col3 := lipgloss.JoinVertical(lipgloss.Left,
		HelpTextStyle.Render("a      Approve"),
		HelpTextStyle.Render("u      Unreviewed"),
		HelpTextStyle.Render("r      Reset file"),
		HelpTextStyle.Render("R      Reset all"),
		HelpTextStyle.Render("n/p    Next/Prev Todo"),
	)
	col4 := lipgloss.JoinVertical(lipgloss.Left,
		HelpTextStyle.Render("c      Comment"),
		HelpTextStyle.Render("d      Del Comment"),
		HelpTextStyle.Render("E      Export"),
		HelpTextStyle.Render("s      Split View"),
		HelpTextStyle.Render("t      Cycle Theme"),
	)
	col5 := lipgloss.JoinVertical(lipgloss.Left,
		HelpTextStyle.Render("F      fzf Jump"),
		HelpTextStyle.Render("/      Search"),
		HelpTextStyle.Render("f      Flat Mode"),
		HelpTextStyle.Render("V      Visual Mode"),
		HelpTextStyle.Render("?      Toggle Help"),
	)

	spacer := lipgloss.NewStyle().Width(4).Render("")
	return HelpDrawerStyle.Copy().Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Top,
			col1, spacer, col2, spacer, col3, spacer, col4, spacer, col5,
		),
	)
}

func (m Model) renderEmptyState(w, h int) string {
	logo := EmptyLogoStyle.Render("git-review")
	desc := EmptyDescStyle.Render("GitHub-style code review in your terminal.")

	statusMsg := "No changes to review."
	if len(m.fileList.Items()) == 0 {
		switch m.to {
		case "HEAD":
			statusMsg = fmt.Sprintf("No uncommitted changes from %s — working tree is clean.", m.from[:min(len(m.from), 12)])
		case "--cached":
			statusMsg = "No staged changes — run 'git add' to stage files for review."
		default:
			statusMsg = fmt.Sprintf("No changes between %s and %s.", m.from[:min(len(m.from), 12)], m.to[:min(len(m.to), 12)])
		}
	}
	status := EmptyStatusStyle.Render(statusMsg)

	content := lipgloss.JoinVertical(lipgloss.Center, logo, desc, status)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, content)
}
