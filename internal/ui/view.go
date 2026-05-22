package ui

import (
	"fmt"
	"math"
	"regexp"
	"slices"
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

	contentHeight := max(0, m.height-lipgloss.Height(topBar)-lipgloss.Height(bottomBar))

	var mainContent string
	if len(m.fileList.Items()) == 0 {
		mainContent = m.renderEmptyState(m.width, contentHeight)
	} else {
		treeStyle := PaneStyle
		if m.focus == FocusTree {
			treeStyle = FocusedPaneStyle
		}

		treeView := treeStyle.
			Width(m.fileList.Width()).
			Height(contentHeight - 2).
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

		scrollbar := m.renderDiffScrollbar(contentHeight)
		mainContent = lipgloss.JoinHorizontal(lipgloss.Top, treeView, rightPaneView, scrollbar)
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

func (m Model) renderDiffScrollbar(height int) string {
	trackStyle := lipgloss.NewStyle().Foreground(m.activeTheme.BorderNormal)
	thumbStyle := lipgloss.NewStyle().Foreground(m.activeTheme.AccentText)

	lines := make([]string, height)
	lines[0] = " "
	if height > 1 {
		lines[height-1] = " "
	}

	barH := height - 2
	if barH <= 0 || len(m.diffLines) == 0 {
		for i := 1; i < height-1; i++ {
			lines[i] = trackStyle.Render("╎")
		}
		return strings.Join(lines, "\n")
	}

	total := len(m.diffLines)
	var offset, viewportH int
	if m.splitView {
		offset = m.splitOffset
		viewportH = m.diffViewport.Height
	} else {
		offset = m.diffViewport.YOffset
		viewportH = m.diffContentHeight()
	}

	if total <= viewportH {
		for i := 1; i < height-1; i++ {
			lines[i] = trackStyle.Render("╎")
		}
	} else {
		thumbH := max(1, barH*viewportH/total)
		maxOff := barH - thumbH
		thumbOff := max(0, min(maxOff*offset/(total-viewportH), maxOff))
		for i := range barH {
			if i >= thumbOff && i < thumbOff+thumbH {
				lines[i+1] = thumbStyle.Render("█")
			} else {
				lines[i+1] = trackStyle.Render("╎")
			}
		}
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderUnifiedDiff(contentHeight int) string {
	var renderedDiff strings.Builder

	// Use the pane's inner height so the renderer fills exactly what the pane shows.
	// Reserve 1 row when the "changed since approval" header is shown.
	showsHeader := m.computedStatuses[m.selectedPath] == review.StatusChanged
	viewportHeight := m.diffViewport.Height
	if showsHeader && viewportHeight > 0 {
		viewportHeight--
	}
	start := m.diffViewport.YOffset

	// Width(diffViewport.Width-4) = content-box text area (excludes 2 padding + 2 border).
	// wrapAt = diffViewport.Width-4.
	// lineNum(5) + gutter(4) = 9 chars overhead for numbered lines.
	// So codeMaxW = diffViewport.Width-4-9 = diffViewport.Width-13.
	// maxLineWidth = codeMaxW+4 (gutter) = diffViewport.Width-9.
	maxLineWidth := max(1, m.diffViewport.Width-9)

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

	codeMaxW := max(1, maxLineWidth-4) // 4-char gutter: "+ │ "

	contGutter := DiffCtxGutter.Render("  │ ")

	visualRows := 0
	for i := start; i < len(m.diffLines) && visualRows < viewportHeight; i++ {
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
				visualRows++
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
		isSearchMatch := slices.Contains(m.searchMatches, i)

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
		lineNumPad := ""
		if numStr != "" {
			lineNumRendered = LineNumberStyle.Render(numStr)
			lineNumPad = strings.Repeat(" ", lipgloss.Width(lineNumRendered))
		} else if isDel && (lineNumMode == "absolute" || (lineNumMode == "hybrid" && isCursor)) {
			// Deleted lines have no new-file line number — pad with spaces matching
			// the width of the nearby line number so the code column stays aligned.
			padW := lipgloss.Width(LineNumberStyle.Render(fmt.Sprintf("%d", fileLineNo)))
			lineNumRendered = strings.Repeat(" ", padW)
			lineNumPad = lineNumRendered
		}

		// Wrap code content into visual rows.
		codeRows := wrapCodeRows(codeContent, codeMaxW)

		// Highlighted text (first row only): truncate to the same visual width as
		// codeRows[0] so the two agree on where the first row ends.
		// Continuation rows use plain text to avoid ANSI sequences confusing Hardwrap.
		var hlFirstRow string
		if !isCursor && i < len(m.diffHighlighted) {
			hl := bgAnsiRe.ReplaceAllString(m.diffHighlighted[i], "")
			hlFirstRow = ansi.Truncate(hl, lipgloss.Width(codeRows[0]), "")
		}

		for rowIdx, rowContent := range codeRows {
			if visualRows >= viewportHeight {
				break
			}
			isFirstRow := rowIdx == 0

			var line string
			if isCursor {
				var bgColor, fgColor lipgloss.Color
				indicator := " "
				if isAdd {
					bgColor, fgColor = m.activeTheme.CursorAddBg, m.activeTheme.CursorAddFg
					if isFirstRow {
						indicator = "+"
					}
				} else if isDel {
					bgColor, fgColor = m.activeTheme.CursorDelBg, m.activeTheme.CursorDelFg
					if isFirstRow {
						indicator = "-"
					}
				} else {
					bgColor, fgColor = m.activeTheme.CursorCtxBg, m.activeTheme.CursorCtxFg
				}
				bg := ansiColorBg(bgColor)
				fg := ansiColorFg(fgColor)
				pad := strings.Repeat(" ", max(0, codeMaxW-lipgloss.Width(rowContent)))
				gutterPart := indicator + " " + separator + " "
				if !isFirstRow {
					gutterPart = "  " + separator + " "
				}
				line = bg + fg + "\x1b[1m" + gutterPart + "\x1b[22m" +
					rowContent + pad + "\x1b[0m"

			} else if isAdd || isDel {
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

				var gutterStr string
				if isFirstRow {
					gutterStr = bg + "\x1b[1m" + ansiColorFg(gutterFg) +
						indicator + " " + separator + " \x1b[22m\x1b[39m"
				} else {
					gutterStr = bg + "  " + separator + " "
				}

				var code string
				if isSearchMatch && m.searchQuery != "" && isFirstRow {
					code = bg + SearchMatchStyle.Render(rowContent)
				} else if isFirstRow && hlFirstRow != "" {
					code = injectBgAfterResets(hlFirstRow, bg)
				} else {
					code = bg + rowContent
				}

				pad := strings.Repeat(" ", max(0, codeMaxW-lipgloss.Width(rowContent)))
				line = gutterStr + code + bg + pad + "\x1b[0m"

			} else {
				// Context line: syntax highlighting, explicit canvas-bg fill to full width.
				var gutterStr string
				if isFirstRow {
					gutterStr = DiffCtxGutter.Render("  " + separator + " ")
				} else {
					gutterStr = contGutter
				}
				var hlCode string
				if isSearchMatch && m.searchQuery != "" && isFirstRow {
					hlCode = SearchMatchStyle.Render(rowContent)
				} else if isFirstRow && hlFirstRow != "" {
					hlCode = hlFirstRow
				} else {
					hlCode = rowContent
				}
				ctxPad := strings.Repeat(" ", max(0, codeMaxW-lipgloss.Width(rowContent)))
				ctxBg := ansiColorBg(m.activeTheme.CanvasBg)
				line = gutterStr + hlCode + ctxBg + ctxPad + "\x1b[0m"
			}

			if isFirstRow {
				renderedDiff.WriteString(lineNumRendered + line + "\n")
			} else {
				renderedDiff.WriteString(lineNumPad + line + "\n")
			}
			visualRows++
		}

		// Render comment ghost line if there's a comment on this diff line
		if c, ok := m.lineComments[i]; ok && m.cfg.UI.ShowCommentsInline && visualRows < viewportHeight {
			commentPrefix := "     ▶ "
			commentText := trimLine(c.Body, maxLineWidth-len(commentPrefix)-2)
			ghost := CommentStyle.Render(commentPrefix + commentText)
			renderedDiff.WriteString(ghost + "\n")
			visualRows++
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

	diffContentStr := strings.TrimRight(renderedDiff.String(), "\n")
	if headerStr != "" {
		diffContentStr = headerStr + "\n" + diffContentStr
	}

	// Enforce canvas background: re-apply after every ANSI reset, then explicitly
	// pad each line to the full inner width and fill any empty rows so the
	// terminal's background color cannot show through anywhere in the pane.
	if canvasBgSeq := ansiColorBg(m.activeTheme.CanvasBg); canvasBgSeq != "" {
		diffContentStr = injectBgAfterResets(diffContentStr, canvasBgSeq)
		innerW := max(1, m.diffViewport.Width-4)
		targetH := max(1, contentHeight-2)
		lines := strings.Split(diffContentStr, "\n")
		for i, l := range lines {
			if w := lipgloss.Width(l); w < innerW {
				lines[i] = l + canvasBgSeq + strings.Repeat(" ", innerW-w) + "\x1b[0m"
			}
		}
		for len(lines) < targetH {
			lines = append(lines, canvasBgSeq+strings.Repeat(" ", innerW)+"\x1b[0m")
		}
		diffContentStr = strings.Join(lines, "\n")
	}

	diffPaneStyle := PaneStyle
	if m.focus == FocusDiff {
		diffPaneStyle = FocusedPaneStyle
	}

	return diffPaneStyle.
		Width(m.diffViewport.Width - 4).
		Height(contentHeight - 2).
		MaxHeight(contentHeight).
		Render(diffContentStr)
}

func (m Model) renderTopBar() string {
	// Left: repo + range (rangeLabel already contains branch info for default mode)
	info := fmt.Sprintf(" %s  %s", m.repoName, m.rangeLabel)
	leftSide := TopInfoStyle.Render(info)

	sep := TopInfoStyle.Render(" · ")

	// Review counts
	approved, changed, _, total := m.reviewSummary()
	reviewInfo := ""
	if total > 0 {
		reviewInfo = sep + TopInfoStyle.Render(fmt.Sprintf("✓%d  !%d  ○ %d/%d", approved, changed, total-approved-changed, total))
	}

	statsStr := ""
	if m.statsAdded > 0 || m.statsDeleted > 0 {
		statsStr = sep +
			TopStatsAddedStyle.Render(fmt.Sprintf("+%d", m.statsAdded)) +
			TopStatsDeletedStyle.Render(fmt.Sprintf("-%d", m.statsDeleted))
	}

	modeLabel := ""
	if m.isDirtyMode {
		modeLabel = sep + lipgloss.NewStyle().
			Foreground(m.activeTheme.StatusChanged).
			Background(m.activeTheme.TopBarBg).
			Render("[dirty]")
	} else {
		activeTabStyle := lipgloss.NewStyle().
			Foreground(m.activeTheme.AccentText).
			Background(m.activeTheme.TopBarBg).
			Bold(true).
			Padding(0, 1)
		inactiveTabStyle := lipgloss.NewStyle().
			Foreground(m.activeTheme.DimText).
			Background(m.activeTheme.TopBarBg).
			Padding(0, 1)
		tabSepStyle := lipgloss.NewStyle().
			Foreground(m.activeTheme.DimText).
			Background(m.activeTheme.TopBarBg)

		var tabNotApproved, tabAll string
		if m.filterTab == FilterNotApproved {
			tabNotApproved = activeTabStyle.Render("not approved")
			tabAll = inactiveTabStyle.Render("all")
		} else {
			tabNotApproved = inactiveTabStyle.Render("not approved")
			tabAll = activeTabStyle.Render("all")
		}
		modeLabel = sep + tabNotApproved + tabSepStyle.Render("·") + tabAll
	}

	// Right: current file + file stats
	rightSide := ""
	if sel, ok := m.fileList.SelectedItem().(tree.TreeItem); ok && !sel.IsDir {
		fileStats := ""
		if fs, ok := m.fileStats[sel.FullPath]; ok {
			fileStats = fmt.Sprintf("  +%d -%d", fs[0], fs[1])
		}

		status := m.computedStatuses[sel.FullPath]
		var statusLabel string
		tbStatus := lipgloss.NewStyle().Background(m.activeTheme.TopBarBg)
		switch status {
		case review.StatusApproved:
			statusLabel = tbStatus.Foreground(m.activeTheme.StatusApproved).Render("  [APPROVED]")
		case review.StatusChanged:
			statusLabel = tbStatus.Foreground(m.activeTheme.StatusChanged).Render("  [CHANGED]")
		case review.StatusViewed:
			statusLabel = tbStatus.Foreground(m.activeTheme.StatusViewed).Render("  [VIEWED]")
		}

		leftW := lipgloss.Width(leftSide) + lipgloss.Width(statsStr) + lipgloss.Width(reviewInfo) + lipgloss.Width(modeLabel)
		maxPathW := m.width - leftW - lipgloss.Width(fileStats) - lipgloss.Width(statusLabel) - 3
		if maxPathW >= 4 {
			fgStyle := lipgloss.NewStyle().Foreground(m.activeTheme.NormalText).Background(m.activeTheme.TopBarBg)
			truncPath := fgStyle.Render(ansi.Truncate(sel.FullPath, maxPathW, "…"))
			styledFileStats := ""
			if fileStats != "" {
				styledFileStats = fgStyle.Render(fileStats)
			}
			// trailing space gives the right side internal padding before the edge
			rightSide = truncPath + styledFileStats + statusLabel + fgStyle.Render(" ")
		}
	}

	availWidth := max(0, m.width-lipgloss.Width(leftSide)-lipgloss.Width(statsStr)-lipgloss.Width(reviewInfo)-lipgloss.Width(modeLabel)-lipgloss.Width(rightSide))
	padding := strings.Repeat(" ", availWidth)

	finalBar := lipgloss.JoinHorizontal(lipgloss.Top,
		leftSide, statsStr, reviewInfo, modeLabel, padding, rightSide)

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
		avail := max(0, m.width-lipgloss.Width(prompt)-lipgloss.Width(input)-lipgloss.Width(matchInfo)-lipgloss.Width(hint))
		pad := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", avail))
		return StatusBarStyle.Width(m.width).Render(
			lipgloss.JoinHorizontal(lipgloss.Top, prompt, input, matchInfo, pad, hint),
		)
	}

	if m.statusNotify != "" {
		notify := StatusNotifyStyle.Render(m.statusNotify)
		avail := max(0, m.width-lipgloss.Width(notify))
		pad := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", avail))
		return StatusBarStyle.Width(m.width).Render(
			lipgloss.JoinHorizontal(lipgloss.Top, notify, pad),
		)
	}

	shortcuts := StatusKeyStyle.Render("?help  q quit  Tab focus  a approve  c comment  E export  s split  t theme  v toggle all  b range  F fuzzy")
	avail := max(0, m.width-lipgloss.Width(shortcuts))
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
		HelpTextStyle.Render("H/M/L  Top/Mid/Bottom"),
	)
	col2 := lipgloss.JoinVertical(lipgloss.Left,
		HelpTextStyle.Render("h/l    Focus Left/Right"),
		HelpTextStyle.Render("Tab    Toggle Panel"),
		HelpTextStyle.Render("]c/[c  Next/Prev Hunk"),
		HelpTextStyle.Render("zz/zt  Center/Top Cursor"),
		HelpTextStyle.Render("e      Edit File"),
	)
	col3 := lipgloss.JoinVertical(lipgloss.Left,
		HelpTextStyle.Render("a      Approve"),
		HelpTextStyle.Render("u      Mark Unreviewed"),
		HelpTextStyle.Render("r      Reset File"),
		HelpTextStyle.Render("R      Reset Review"),
		HelpTextStyle.Render("n/p    Next/Prev Unreviewed"),
	)
	col4 := lipgloss.JoinVertical(lipgloss.Left,
		HelpTextStyle.Render("c      Comment"),
		HelpTextStyle.Render("d      Delete Comment"),
		HelpTextStyle.Render("E      Export"),
		HelpTextStyle.Render("s      Split View"),
		HelpTextStyle.Render("t      Theme Picker"),
		HelpTextStyle.Render("b      Change Range"),
	)
	col5 := lipgloss.JoinVertical(lipgloss.Left,
		HelpTextStyle.Render("F      Fuzzy Search"),
		HelpTextStyle.Render("/      Search"),
		HelpTextStyle.Render("f      Toggle Flat View"),
		HelpTextStyle.Render("v      Toggle View All"),
		HelpTextStyle.Render("V      Visual Mode"),
		HelpTextStyle.Render("?      Toggle Help"),
	)

	spacer := lipgloss.NewStyle().Width(4).Render("")
	return HelpDrawerStyle.Width(m.width).Render(
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
	restore := "\x1b[39m\x1b[22m\x1b[23m\x1b[24m\x1b[27m" + bgAnsi
	s = strings.ReplaceAll(s, "\x1b[0m", restore)
	s = strings.ReplaceAll(s, "\x1b[m", restore)
	return bgAnsi + s
}

// wrapCodeRows splits content into visual rows of at most width columns.
// Continuation rows are prefixed with the same leading whitespace as the first
// row so wrapped lines stay visually indented at the correct level.
func wrapCodeRows(content string, width int) []string {
	if width <= 0 || lipgloss.Width(content) <= width {
		return []string{content}
	}

	// Count leading spaces so continuation rows can re-indent.
	indent := 0
	for _, ch := range content {
		if ch == ' ' {
			indent++
		} else {
			break
		}
	}

	wrapped := ansi.Hardwrap(content, width, true)
	rows := strings.Split(wrapped, "\n")
	if len(rows) == 0 {
		return []string{content}
	}

	if indent > 0 {
		prefix := strings.Repeat(" ", indent)
		for i := 1; i < len(rows); i++ {
			if rows[i] != "" {
				rows[i] = prefix + rows[i]
			}
		}
	}

	return rows
}

func (m Model) renderEmptyState(w, h int) string {
	logo := EmptyLogoStyle.Render("git-review")
	desc := EmptyDescStyle.Render("GitHub-style code review in your terminal.")

	statusMsg := "No changes to review."
	if len(m.fileList.Items()) == 0 {
		switch m.to {
		case "--cached":
			statusMsg = "No staged changes — run 'git add' to stage files for review."
		default:
			fromLabel := m.prettyRef(m.from, "base")
			toLabel := m.prettyRef(m.to, "head")
			statusMsg = fmt.Sprintf("No changes between %s and %s.", fromLabel, toLabel)
		}
	}
	status := EmptyStatusStyle.Render(statusMsg)

	content := lipgloss.JoinVertical(lipgloss.Center, logo, desc, status)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, content)
}
