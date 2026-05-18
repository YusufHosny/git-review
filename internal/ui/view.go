package ui

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/YusufHosny/git-review/internal/review"
	"github.com/YusufHosny/git-review/internal/tree"
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

		// Line number
		var numStr string
		switch lineNumMode {
		case "absolute":
			if realLineNo > 0 {
				numStr = fmt.Sprintf("%d", realLineNo)
			}
		case "hybrid":
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

		codeMaxW := maxLineWidth - 4 // 4-char gutter: "+ │ "

		var line string
		if isCursor {
			// Cursor: single full-width block using direct ANSI for clean background.
			var bgColor, fgColor lipgloss.Color
			indicator := " "
			if isAdd {
				bgColor, fgColor = m.activeTheme.CursorAddBg, m.activeTheme.CursorAddFg
				indicator = "+"
			} else if isDel {
				bgColor, fgColor = m.activeTheme.CursorDelBg, m.activeTheme.CursorDelFg
				indicator = "-"
			} else {
				bgColor, fgColor = m.activeTheme.CursorCtxBg, m.activeTheme.CursorCtxFg
			}
			bg := ansiColorBg(bgColor)
			fg := ansiColorFg(fgColor)
			truncCode := ansi.Truncate(codeContent, codeMaxW, "")
			pad := strings.Repeat(" ", max(0, codeMaxW-lipgloss.Width(truncCode)))
			line = bg + fg + "\x1b[1m" + indicator + " " + separator + " \x1b[22m" +
				truncCode + pad + "\x1b[0m"

		} else if isAdd || isDel {
			// Add/del: full-width colored background with syntax highlighting.
			// We re-inject the diff background after every chroma reset so the
			// background stays consistent across syntax tokens (delta-style).
			var bgColor lipgloss.Color
			var gutterFg lipgloss.Color
			indicator := "+"
			if isDel {
				bgColor = m.activeTheme.DelBg
				gutterFg = m.activeTheme.GutterDel
				indicator = "-"
			} else {
				bgColor = m.activeTheme.AddBg
				gutterFg = m.activeTheme.GutterAdd
			}
			bg := ansiColorBg(bgColor)

			// Gutter: bold indicator fg on diff bg, then reset fg/bold (bg stays).
			gutter := bg + "\x1b[1m" + ansiColorFg(gutterFg) +
				indicator + " " + separator + " \x1b[22m\x1b[39m"

			// Code: syntax highlight with bg preserved across resets.
			var code string
			if isSearchMatch && m.searchQuery != "" {
				code = bg + SearchMatchStyle.Render(ansi.Truncate(codeContent, codeMaxW, ""))
			} else if i < len(m.diffHighlighted) {
				hl := bgAnsiRe.ReplaceAllString(m.diffHighlighted[i], "")
				hl = ansi.Truncate(hl, codeMaxW, "")
				code = injectBgAfterResets(hl, bg)
			} else {
				code = bg + ansi.Truncate(codeContent, codeMaxW, "")
			}

			truncPlain := ansi.Truncate(codeContent, codeMaxW, "")
			pad := strings.Repeat(" ", max(0, codeMaxW-lipgloss.Width(truncPlain)))
			line = gutter + code + bg + pad + "\x1b[0m"

		} else {
			// Context line: syntax highlighting, no background.
			var gutterStr string
			if isAdd {
				gutterStr = DiffAddGutter.Render("+ " + separator + " ")
			} else if isDel {
				gutterStr = DiffDelGutter.Render("- " + separator + " ")
			} else {
				gutterStr = DiffCtxGutter.Render("  " + separator + " ")
			}
			var hlCode string
			if isSearchMatch && m.searchQuery != "" {
				hlCode = SearchMatchStyle.Render(ansi.Truncate(codeContent, codeMaxW, ""))
			} else if i < len(m.diffHighlighted) {
				hlCode = bgAnsiRe.ReplaceAllString(m.diffHighlighted[i], "")
				hlCode = ansi.Truncate(hlCode, codeMaxW, "")
			} else {
				hlCode = ansi.Truncate(codeContent, codeMaxW, "")
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

	sep := TopInfoStyle.Render(" · ")

	// Stats counts
	approved, changed, _, total := m.reviewSummary()
	reviewInfo := ""
	if total > 0 {
		reviewInfo = sep + TopInfoStyle.Render(fmt.Sprintf("✓%d  !%d  ○%d/%d", approved, changed, total-approved-changed, total))
	}

	statsStr := ""
	if m.statsAdded > 0 || m.statsDeleted > 0 {
		statsStr = sep +
			TopStatsAddedStyle.Render(fmt.Sprintf("+%d", m.statsAdded)) +
			TopStatsDeletedStyle.Render(fmt.Sprintf("-%d", m.statsDeleted))
	}

	themeLabel := sep + StatusKeyStyle.Render("["+m.activeTheme.Name+"]")

	// Right: current file + file stats
	rightSide := ""
	if sel, ok := m.fileList.SelectedItem().(tree.TreeItem); ok && !sel.IsDir {
		fileStats := ""
		if fs, ok := m.fileStats[sel.FullPath]; ok {
			fileStats = fmt.Sprintf("  +%d -%d", fs[0], fs[1])
		}

		status := m.computedStatuses[sel.FullPath]
		var statusLabel string
		switch status {
		case review.StatusApproved:
			statusLabel = "  " + StatusApprovedStyle.Render("[APPROVED]")
		case review.StatusChanged:
			statusLabel = "  " + StatusChangedStyle.Render("[CHANGED]")
		case review.StatusViewed:
			statusLabel = "  " + StatusViewedStyle.Render("[VIEWED]")
		}

		leftW := lipgloss.Width(leftSide) + lipgloss.Width(statsStr) + lipgloss.Width(reviewInfo) + lipgloss.Width(themeLabel)
		maxPathW := m.width - leftW - lipgloss.Width(fileStats) - lipgloss.Width(statusLabel) - 6
		if maxPathW < 10 {
			maxPathW = 10
		}
		truncPath := ansi.Truncate(sel.FullPath, maxPathW, "…")
		rightSide = truncPath + fileStats + statusLabel
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

// ansiColorBg converts a #RRGGBB lipgloss color to an ANSI 24-bit background escape.
func ansiColorBg(c lipgloss.Color) string {
	h := string(c)
	if len(h) == 7 && h[0] == '#' {
		r, _ := strconv.ParseInt(h[1:3], 16, 64)
		g, _ := strconv.ParseInt(h[3:5], 16, 64)
		b, _ := strconv.ParseInt(h[5:7], 16, 64)
		return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
	}
	return ""
}

// ansiColorFg converts a #RRGGBB lipgloss color to an ANSI 24-bit foreground escape.
func ansiColorFg(c lipgloss.Color) string {
	h := string(c)
	if len(h) == 7 && h[0] == '#' {
		r, _ := strconv.ParseInt(h[1:3], 16, 64)
		g, _ := strconv.ParseInt(h[3:5], 16, 64)
		b, _ := strconv.ParseInt(h[5:7], 16, 64)
		return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
	}
	return ""
}

// injectBgAfterResets re-applies bgAnsi after every ANSI reset sequence so that
// a background color persists through chroma's per-token reset codes.
func injectBgAfterResets(s, bgAnsi string) string {
	restore := "\x1b[39m\x1b[22m" + bgAnsi
	s = strings.ReplaceAll(s, "\x1b[0m", restore)
	s = strings.ReplaceAll(s, "\x1b[m", restore)
	return bgAnsi + s
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
