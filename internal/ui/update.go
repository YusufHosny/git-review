package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/YusufHosny/git-review/internal/config"
	"github.com/YusufHosny/git-review/internal/git"
	"github.com/YusufHosny/git-review/internal/review"
	"github.com/YusufHosny/git-review/internal/tree"
)

var ctrlRe = regexp.MustCompile(`[\x00-\x08\x0B-\x0C\x0E-\x1F]`)

// c1CtrlRe matches C1 control characters (U+0080–U+009F) encoded as valid UTF-8.
// Standard VTE terminals consume these as 8-bit control sequences (invisible), but
// Kitty renders them as visible placeholder glyphs (~1 cell). Since Go's width
// libraries measure them as 0-wide, they cause per-char overflow in Kitty only.
var c1CtrlRe = regexp.MustCompile(`[\x{0080}-\x{009F}]`)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Handle overlay modes first — they consume all input except their own keys
	if m.overlay == OverlayCommentInput {
		return m.updateCommentOverlay(msg)
	}
	if m.overlay == OverlayConfirm {
		return m.updateConfirmOverlay(msg)
	}
	if m.overlay == OverlayThemePicker {
		return m.updateThemePicker(msg)
	}

	// Handle search mode input
	if m.searchMode {
		return m.updateSearchMode(msg)
	}

	keyHandled := false

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()
		m.commentInput.SetWidth(m.width - 12)

	case StatsMsg:
		m.statsAdded = msg.Added
		m.statsDeleted = msg.Deleted
		if msg.ByFile != nil {
			m.fileStats = msg.ByFile
		}

	case batchMsg:
		var batchCmds []tea.Cmd
		for _, innerMsg := range msg {
			updated, c := m.Update(innerMsg)
			m = updated.(Model)
			if c != nil {
				batchCmds = append(batchCmds, c)
			}
		}
		return m, tea.Batch(batchCmds...)

	case ChangeCheckMsg:
		if msg.Changed {
			m.computedStatuses[msg.File] = review.StatusChanged
			m.updateTreeFocus()
			m.refreshTreeItems()
		}

	case FzfReadyMsg:
		return m, execFzfCmd(msg.InputPath, msg.OutputPath)

	case FzfDoneMsg:
		if msg.Err == nil && msg.File != "" {
			// Navigate to the file
			items := m.fileList.Items()
			for i, item := range items {
				if ti, ok := item.(tree.TreeItem); ok && ti.FullPath == msg.File {
					m.fileList.Select(i)
					m.selectedPath = ti.FullPath
					m.diffCursor = msg.Index
					from, to := m.diffRangeForFile(m.selectedPath)
					m.currentFrom, m.currentTo = from, to
					cmds = append(cmds, m.gitClient.DiffCmd(from, to, m.selectedPath))
					break
				}
			}
		}

	case ExportDoneMsg:
		if msg.Err != nil {
			m.statusNotify = "Export failed: " + msg.Err.Error()
		} else {
			m.statusNotify = "Exported to " + msg.Path
		}
		cmds = append(cmds, clearNotifyCmd())

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "?":
			m.showHelp = !m.showHelp
			m.updateSizes()
			return m, nil
		}

		if len(m.fileList.Items()) == 0 {
			return m, nil
		}

		// Handle pending z-command
		if m.pendingZ {
			m.pendingZ = false
			if m.focus == FocusDiff {
				switch msg.String() {
				case "z", ".":
					m.centerDiffCursor()
				case "t":
					m.setYOffset(m.diffCursor)
				case "b":
					m.setYOffset(m.diffCursor - m.diffViewport.Height + 1)
				}
			}
			return m, nil
		}

		// Handle pending bracket for ]c / [c
		if m.pendingBracket != 0 {
			bracket := m.pendingBracket
			m.pendingBracket = 0
			if m.focus == FocusDiff && msg.String() == "c" {
				if bracket == ']' {
					m.jumpToNextHunk()
				} else if bracket == '[' {
					m.jumpToPrevHunk()
				}
			}
			m.inputBuffer = ""
			return m, nil
		}

		// Digit accumulation for count prefix
		if len(msg.String()) == 1 && strings.ContainsAny(msg.String(), "0123456789") {
			m.inputBuffer += msg.String()
			return m, nil
		}

		switch msg.String() {
		// === Visual mode ===
		case "V":
			if m.focus == FocusDiff {
				m.visualMode = !m.visualMode
				if m.visualMode {
					m.visualStart = m.diffCursor
				}
			}
			m.inputBuffer = ""

		case "esc":
			m.visualMode = false
			m.inputBuffer = ""
			m.statusNotify = ""
			m.searchQuery = ""
			m.searchMatches = nil
			m.searchCursor = 0
			m.pendingBracket = 0
			m.pendingZ = false

		// === Focus switching ===
		case "tab":
			m.visualMode = false
			if m.focus == FocusTree {
				if item, ok := m.fileList.SelectedItem().(tree.TreeItem); ok && item.IsDir {
					return m, nil
				}
				m.focus = FocusDiff
			} else {
				m.focus = FocusTree
			}
			m.updateTreeFocus()
			m.inputBuffer = ""

		case "ctrl+h":
			m.visualMode = false
			m.focus = FocusTree
			m.updateTreeFocus()
			m.inputBuffer = ""

		case "ctrl+l":
			m.visualMode = false
			if m.focus == FocusTree {
				if item, ok := m.fileList.SelectedItem().(tree.TreeItem); ok && item.IsDir {
					return m, nil
				}
			}
			m.focus = FocusDiff
			m.updateTreeFocus()
			m.inputBuffer = ""

		case "h", "left":
			m.visualMode = false
			keyHandled = true
			m.focus = FocusTree
			m.updateTreeFocus()
			m.inputBuffer = ""

		case "l", "right":
			m.visualMode = false
			keyHandled = true
			if item, ok := m.fileList.SelectedItem().(tree.TreeItem); ok && item.IsDir {
				return m, nil
			}
			m.focus = FocusDiff
			m.updateTreeFocus()
			m.inputBuffer = ""

		case "]":
			if m.focus == FocusDiff {
				m.pendingBracket = ']'
			} else {
				m.focus = FocusDiff
				m.updateTreeFocus()
			}
			m.inputBuffer = ""

		case "[":
			if m.focus == FocusDiff {
				m.pendingBracket = '['
			} else {
				m.focus = FocusTree
				m.updateTreeFocus()
			}
			m.inputBuffer = ""

		// === Flat mode ===
		case "f":
			if m.focus == FocusTree {
				m.flatMode = !m.flatMode
				m.fileList.SetItems(m.treeState.Items(m.flatMode, m.computedStatuses))
				for i, item := range m.fileList.Items() {
					if ti, ok := item.(tree.TreeItem); ok && ti.FullPath == m.selectedPath {
						m.fileList.Select(i)
						break
					}
				}
			}
			m.inputBuffer = ""

		// === Editor ===
		case "e", "enter":
			m.visualMode = false
			if m.focus == FocusTree && msg.String() == "enter" {
				if i, ok := m.fileList.SelectedItem().(tree.TreeItem); ok && i.IsDir {
					m.treeState.ToggleExpand(i.FullPath)
					m.fileList.SetItems(m.treeState.Items(m.flatMode, m.computedStatuses))
					return m, nil
				}
			}
			if m.selectedPath != "" {
				if i, ok := m.fileList.SelectedItem().(tree.TreeItem); ok && i.IsDir {
					return m, nil
				}
				line := 0
				if m.focus == FocusDiff {
					line = m.gitClient.CalculateFileLine(m.diffLines, m.diffCursor)
				}
				m.inputBuffer = ""
				return m, m.gitClient.OpenEditorCmd(m.selectedPath, line, m.cfg.Editor)
			}

		// === z-prefix scrolling ===
		case "z":
			if m.focus == FocusDiff {
				m.pendingZ = true
			}
			return m, nil

		// === Viewport movement ===
		case "H":
			if m.focus == FocusDiff {
				m.diffCursor = m.snapCursor(m.diffViewport.YOffset, 1)
			}

		case "M":
			if m.focus == FocusDiff {
				half := m.diffViewport.Height / 2
				m.diffCursor = m.snapCursor(m.diffViewport.YOffset+half, 1)
			}

		case "L":
			if m.focus == FocusDiff {
				end := m.diffViewport.YOffset + m.diffViewport.Height - 1
				m.diffCursor = m.snapCursor(end, -1)
			}

		case "ctrl+d":
			if m.focus == FocusDiff {
				if m.splitView {
					m.splitOffset += m.diffViewport.Height / 2
				} else {
					target := m.diffCursor + m.diffViewport.Height/2
					m.diffCursor = m.snapCursor(target, 1)
					m.centerDiffCursor()
				}
			}
			m.inputBuffer = ""

		case "ctrl+u":
			if m.focus == FocusDiff {
				if m.splitView {
					m.splitOffset -= m.diffViewport.Height / 2
					if m.splitOffset < 0 {
						m.splitOffset = 0
					}
				} else {
					target := m.diffCursor - m.diffViewport.Height/2
					m.diffCursor = m.snapCursor(target, -1)
					m.centerDiffCursor()
				}
			}
			m.inputBuffer = ""

		case "j", "down":
			keyHandled = true
			for i := 0; i < m.getRepeatCount(); i++ {
				if m.focus == FocusDiff {
					if m.splitView {
						m.splitOffset++
					} else {
						m.diffCursor = m.snapCursor(m.diffCursor+1, 1)
						m.handleScrolling()
					}
				} else {
					m.fileList.CursorDown()
				}
			}
			m.inputBuffer = ""

		case "k", "up":
			keyHandled = true
			for i := 0; i < m.getRepeatCount(); i++ {
				if m.focus == FocusDiff {
					if m.splitView {
						if m.splitOffset > 0 {
							m.splitOffset--
						}
					} else {
						m.diffCursor = m.snapCursor(m.diffCursor-1, -1)
						m.handleScrolling()
					}
				} else {
					m.fileList.CursorUp()
				}
			}
			m.inputBuffer = ""

		case "g":
			if m.focus == FocusDiff {
				if m.inputBuffer == "g" {
					m.diffCursor = m.snapCursor(0, 1)
					m.setYOffset(m.diffCursor)
					m.splitOffset = 0
					m.inputBuffer = ""
				} else {
					m.inputBuffer = "g"
				}
			}

		case "G":
			if m.focus == FocusDiff {
				count, err := strconv.Atoi(m.inputBuffer)
				if err == nil && count > 0 {
					m.diffCursor = m.snapCursor(count-1, 1)
				} else {
					m.diffCursor = m.snapCursor(len(m.diffLines)-1, -1)
				}
				m.setYOffset(m.offsetToShowCursorAtBottom())
				m.inputBuffer = ""
			}

		// === Review actions ===
		case "a":
			if m.selectedPath != "" && !m.isDir() {
				m.computedStatuses[m.selectedPath] = review.StatusApproved
				m.reviewState.SetFileStatus(m.selectedPath, review.StatusApproved, m.headCommit)
				m.saveReviewState()
				m.updateTreeFocus()
				m.refreshTreeItems()
				m.selectedStatus = review.StatusApproved
				m.statusNotify = "✓ Approved: " + m.selectedPath
				cmds = append(cmds, clearNotifyCmd())
			}

		case "u":
			if m.selectedPath != "" && !m.isDir() {
				m.computedStatuses[m.selectedPath] = review.StatusUnreviewed
				m.reviewState.SetFileStatus(m.selectedPath, review.StatusUnreviewed, "")
				m.saveReviewState()
				m.updateTreeFocus()
				m.refreshTreeItems()
				m.statusNotify = "○ Marked unreviewed: " + m.selectedPath
				cmds = append(cmds, clearNotifyCmd())
			}

		case "r":
			if m.selectedPath != "" && !m.isDir() {
				m.computedStatuses[m.selectedPath] = review.StatusUnreviewed
				m.reviewState.SetFileStatus(m.selectedPath, review.StatusUnreviewed, "")
				m.saveReviewState()
				m.updateTreeFocus()
				m.refreshTreeItems()
				m.statusNotify = "Reset: " + m.selectedPath
				cmds = append(cmds, clearNotifyCmd())
			}

		case "R":
			m.confirmMsg = "Reset ALL review state for this branch? This clears all approvals and comments."
			m.confirmAction = func() tea.Cmd {
				m.reviewState.Reset()
				m.saveReviewState()
				for file := range m.computedStatuses {
					m.computedStatuses[file] = review.StatusUnreviewed
				}
				m.updateTreeFocus()
				m.refreshTreeItems()
				m.rebuildLineComments()
				m.statusNotify = "Review state reset."
				return clearNotifyCmd()
			}
			m.overlay = OverlayConfirm

		// === Search navigation (when a query is active) / next unreviewed file ===
		case "n":
			if m.searchQuery != "" && len(m.searchMatches) > 0 {
				m.searchCursor = (m.searchCursor + 1) % len(m.searchMatches)
				m.diffCursor = m.searchMatches[m.searchCursor]
				m.handleScrolling()
			} else {
				idx := m.findNextUnreviewedFile(1)
				if idx >= 0 {
					m.fileList.Select(idx)
				}
			}

		case "N":
			if m.searchQuery != "" && len(m.searchMatches) > 0 {
				m.searchCursor = (m.searchCursor - 1 + len(m.searchMatches)) % len(m.searchMatches)
				m.diffCursor = m.searchMatches[m.searchCursor]
				m.handleScrolling()
			}

		case "p":
			idx := m.findNextUnreviewedFile(-1)
			if idx >= 0 {
				m.fileList.Select(idx)
			}

		// === Comment ===
		case "c":
			if m.focus == FocusDiff && m.selectedPath != "" && !m.isDir() {
				m.commentLineIndex = m.diffCursor
				if m.diffCursor < len(m.diffLines) {
					m.commentLineContent = stripAnsi(m.diffLines[m.diffCursor])
				}
				m.commentInput.Reset()
				m.commentInput.Focus()
				m.overlay = OverlayCommentInput
			}

		case "d":
			if m.focus == FocusDiff && m.selectedPath != "" {
				if c, ok := m.lineComments[m.diffCursor]; ok {
					commentID := c.ID
					preview := trimLine(c.Body, 40)
					m.confirmMsg = fmt.Sprintf("Delete comment: \"%s\"?", preview)
					m.confirmAction = func() tea.Cmd {
						m.reviewState.DeleteComment(commentID)
						m.saveReviewState()
						m.rebuildLineComments()
						m.statusNotify = "Comment deleted."
						return clearNotifyCmd()
					}
					m.overlay = OverlayConfirm
				}
			}

		// === Export ===
		case "E":
			cmds = append(cmds, m.exportCmd())

		// === Side-by-side view ===
		case "s":
			m.splitView = !m.splitView
			m.splitOffset = 0

		// === Theme picker ===
		case "t":
			m.themePickerCursor = m.themeIndex
			m.overlay = OverlayThemePicker

		// === fzf jump ===
		case "F":
			cmds = append(cmds, m.launchFzfCmd())

		// === Search in diff ===
		case "/":
			if m.focus == FocusDiff {
				m.searchMode = true
				m.searchInput.Reset()
				m.searchInput.Focus()
				m.searchQuery = ""
				m.searchMatches = nil
				m.searchCursor = 0
			}

		default:
			m.inputBuffer = ""
		}
	}

	// Tree navigation
	if len(m.fileList.Items()) > 0 && m.focus == FocusTree {
		if !keyHandled {
			m.fileList, cmd = m.fileList.Update(msg)
			cmds = append(cmds, cmd)
		}

		if item, ok := m.fileList.SelectedItem().(tree.TreeItem); ok {
			if !item.IsDir && item.FullPath != m.selectedPath {
				m.selectedPath = item.FullPath
				m.selectedStatus = m.computedStatuses[item.FullPath]
				m.diffCursor = 0
				m.splitOffset = 0
				m.visualMode = false
				m.diffViewport.GotoTop()
				m.rebuildLineComments()

				from, to := m.diffRangeForFile(m.selectedPath)
				m.currentFrom, m.currentTo = from, to
				cmds = append(cmds, m.gitClient.DiffCmd(from, to, m.selectedPath))

				// Auto-mark unreviewed files as "viewed"
				if m.computedStatuses[m.selectedPath] == review.StatusUnreviewed {
					m.computedStatuses[m.selectedPath] = review.StatusViewed
					m.reviewState.SetFileStatus(m.selectedPath, review.StatusViewed, "")
					m.saveReviewState()
					m.updateTreeFocus()
					m.refreshTreeItems()
				}
			}
		}
	}

	// Handle async messages
	switch msg := msg.(type) {
	case git.DiffMsg:
		content := strings.ToValidUTF8(msg.Content, "?")
		content = strings.ReplaceAll(content, "\t", "    ")
		content = ctrlRe.ReplaceAllString(content, "?")
		content = c1CtrlRe.ReplaceAllString(content, "?")
		
		fullLines := strings.Split(content, "\n")
		var cleanLines, hlLines []string
		var added, deleted int
		foundHunk := false

		ext := strings.TrimPrefix(filepath.Ext(m.selectedPath), ".")
		if ext == "" {
			ext = "txt"
		}

		for _, line := range fullLines {
			cleanLine := stripAnsi(line)

			if strings.HasPrefix(cleanLine, "@@") {
				foundHunk = true
			}

			if !foundHunk {
				continue
			}

			cleanLines = append(cleanLines, line)

			isAdd := strings.HasPrefix(cleanLine, "+") && !strings.HasPrefix(cleanLine, "+++")
			isDel := strings.HasPrefix(cleanLine, "-") && !strings.HasPrefix(cleanLine, "---")

			if isAdd {
				added++
			} else if isDel {
				deleted++
			}

			codeContent := cleanLine
			if len(codeContent) > 0 && (isAdd || isDel || strings.HasPrefix(codeContent, " ")) {
				codeContent = codeContent[1:]
			}

			var buf strings.Builder
			err := quick.Highlight(&buf, codeContent, ext, "terminal16m", m.activeTheme.ChromaTheme)
			if err == nil && buf.String() != "" {
				hlLines = append(hlLines, strings.TrimSuffix(buf.String(), "\n"))
			} else {
				hlLines = append(hlLines, codeContent)
			}
		}

		// Trim trailing empty lines
		for len(cleanLines) > 0 {
			lastLine := strings.TrimRight(stripAnsi(cleanLines[len(cleanLines)-1]), "\r")
			if lastLine != "" {
				break
			}
			cleanLines = cleanLines[:len(cleanLines)-1]
			hlLines = hlLines[:len(hlLines)-1]
		}

		m.diffLines = cleanLines
		m.diffHighlighted = hlLines
		m.currentFileAdded = added
		m.currentFileDeleted = deleted
		m.diffCursor = m.snapCursor(0, 1)
		m.splitOffset = 0

		// Rebuild search matches if query is active
		if m.searchQuery != "" {
			m.rebuildSearchMatches()
		}
		// Rebuild line comments for this file
		m.rebuildLineComments()

	case git.EditorFinishedMsg:
		from, to := m.diffRangeForFile(m.selectedPath)
		return m, m.gitClient.DiffCmd(from, to, m.selectedPath)
	}

	return m, tea.Batch(cmds...)
}

// === Overlay update handlers ===

func (m Model) updateCommentOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.overlay = OverlayNone
			return m, nil
		case "ctrl+s":
			body := strings.TrimSpace(m.commentInput.Value())
			if body != "" {
				m.reviewState.AddComment(
					m.selectedPath,
					m.commentLineContent,
					m.commentLineIndex,
					body,
				)
				m.saveReviewState()
				m.rebuildLineComments()
				m.statusNotify = "Comment saved."
			}
			m.overlay = OverlayNone
			return m, clearNotifyCmd()
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()
		m.commentInput.SetWidth(m.width - 12)
	}

	var cmd tea.Cmd
	m.commentInput, cmd = m.commentInput.Update(msg)
	return m, cmd
}

func (m Model) updateConfirmOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.overlay = OverlayNone
			if m.confirmAction != nil {
				cmd := m.confirmAction()
				m.confirmAction = nil
				return m, cmd
			}
			return m, nil
		case "n", "N", "esc":
			m.overlay = OverlayNone
			m.confirmAction = nil
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()
	}
	return m, nil
}

// === Theme picker handler ===

func (m Model) updateThemePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.overlay = OverlayNone
		case "j", "down":
			m.themePickerCursor = (m.themePickerCursor + 1) % len(Themes)
		case "k", "up":
			m.themePickerCursor = (m.themePickerCursor - 1 + len(Themes)) % len(Themes)
		case "enter", " ":
			m.themeIndex = m.themePickerCursor
			m.activeTheme = Themes[m.themeIndex]
			InitStyles(m.activeTheme)
			m.treeDelegate.ActiveTheme = m.activeTheme
			m.fileList.SetDelegate(m.treeDelegate)
			m.overlay = OverlayNone
			go config.SaveTheme(m.activeTheme.Name) //nolint:errcheck
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()
	}
	return m, nil
}

// === Search mode handler ===

func (m Model) updateSearchMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.searchMode = false
			m.searchQuery = ""
			m.searchMatches = nil
			m.searchCursor = 0
			m.searchInput.Reset()
			return m, nil
		case "enter":
			// Confirm search: exit input mode, keep matches, jump to first.
			m.searchMode = false
			if len(m.searchMatches) > 0 {
				m.diffCursor = m.searchMatches[m.searchCursor]
				m.handleScrolling()
			} else if m.searchQuery != "" {
				m.statusNotify = "Pattern not found: " + m.searchQuery
				return m, clearNotifyCmd()
			}
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	newQuery := m.searchInput.Value()
	if newQuery != m.searchQuery {
		m.searchQuery = newQuery
		m.rebuildSearchMatches()
	}
	return m, cmd
}

func (m *Model) rebuildSearchMatches() {
	m.searchMatches = nil
	if m.searchQuery == "" {
		return
	}
	lowerQ := strings.ToLower(m.searchQuery)
	for i, line := range m.diffLines {
		clean := strings.ToLower(stripAnsi(line))
		if strings.Contains(clean, lowerQ) {
			m.searchMatches = append(m.searchMatches, i)
		}
	}
	// Pre-position searchCursor at the nearest match to current cursor,
	// but don't move diffCursor — the user hasn't pressed enter yet.
	m.searchCursor = 0
	for i, mi := range m.searchMatches {
		if mi >= m.diffCursor {
			m.searchCursor = i
			break
		}
	}
}

// === fzf command (two-step: prep → ExecProcess) ===

// launchFzfCmd prepares fzf input asynchronously, then FzfReadyMsg triggers ExecProcess.
func (m Model) launchFzfCmd() tea.Cmd {
	return func() tea.Msg {
		if _, err := exec.LookPath("fzf"); err != nil {
			return ExportDoneMsg{Err: fmt.Errorf("fzf not found in PATH")}
		}

		lines, err := m.gitClient.AllDiffLines(m.from, m.to)
		if err != nil || len(lines) == 0 {
			return ExportDoneMsg{Err: fmt.Errorf("no diff lines to search")}
		}

		inputFile, err := os.CreateTemp("", "git-review-fzf-in-*")
		if err != nil {
			return ExportDoneMsg{Err: err}
		}
		for _, dl := range lines {
			fmt.Fprintf(inputFile, "%s:%d:%s\n", dl.File, dl.Index, dl.Content)
		}
		inputFile.Close()

		outputFile, err := os.CreateTemp("", "git-review-fzf-out-*")
		if err != nil {
			os.Remove(inputFile.Name())
			return ExportDoneMsg{Err: err}
		}
		outputFile.Close()

		return FzfReadyMsg{InputPath: inputFile.Name(), OutputPath: outputFile.Name()}
	}
}

// execFzfCmd is returned from Update when FzfReadyMsg is received.
func execFzfCmd(inputPath, outputPath string) tea.Cmd {
	shCmd := fmt.Sprintf("fzf --ansi < '%s' > '%s'", inputPath, outputPath)
	cmd := exec.Command("sh", "-c", shCmd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer os.Remove(inputPath)
		defer os.Remove(outputPath)

		data, readErr := os.ReadFile(outputPath)
		if readErr != nil || len(data) == 0 {
			return FzfDoneMsg{}
		}
		selected := strings.TrimSpace(string(data))
		// Format: file:index:content
		parts := strings.SplitN(selected, ":", 3)
		if len(parts) < 2 {
			return FzfDoneMsg{}
		}
		file := parts[0]
		idx, _ := strconv.Atoi(parts[1])
		return FzfDoneMsg{File: file, Index: idx}
	})
}

// === Export command ===

func (m Model) exportCmd() tea.Cmd {
	return func() tea.Msg {
		if len(m.reviewState.Comments) == 0 {
			return ExportDoneMsg{Err: fmt.Errorf("no comments to export")}
		}

		content := review.ExportMarkdown(m.reviewState, m.rangeLabel)

		outPath := "review-comments.md"
		if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
			return ExportDoneMsg{Err: err}
		}

		// Try clipboard too (best-effort)
		_ = review.CopyToClipboard(content)

		return ExportDoneMsg{Path: outPath}
	}
}

// === Helpers ===

func (m *Model) isDir() bool {
	if item, ok := m.fileList.SelectedItem().(tree.TreeItem); ok {
		return item.IsDir
	}
	return false
}

// clearNotifyCmd returns a no-op command; the notify is cleared on next relevant event.
// For now we rely on next keypress or esc to clear.
func clearNotifyCmd() tea.Cmd {
	// Notification persists until next key press — cleared in esc handler.
	return nil
}
