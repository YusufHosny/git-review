package ui

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/yusuf/git-review/internal/config"
	"github.com/yusuf/git-review/internal/git"
	"github.com/yusuf/git-review/internal/review"
	"github.com/yusuf/git-review/internal/tree"
)

type Focus int

const (
	FocusTree Focus = iota
	FocusDiff
)

var ansiRe = regexp.MustCompile("[][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")
var bgAnsiRe = regexp.MustCompile(`\x1b\[48;2;\d+;\d+;\d+m|\x1b\[4[0-9]m`)

type StatsMsg struct {
	Added   int
	Deleted int
	ByFile  map[string][2]int
}

type ChangeCheckMsg struct {
	File    string
	Changed bool
}

type FzfReadyMsg struct {
	InputPath  string
	OutputPath string
}

type FzfDoneMsg struct {
	File  string
	Index int
	Err   error
}

type ExportDoneMsg struct {
	Path string
	Err  error
}

type Model struct {
	// === Panels ===
	fileList     list.Model
	treeState    *tree.FileTree
	treeDelegate TreeDelegate
	diffViewport viewport.Model

	// === Selection ===
	selectedPath   string
	selectedStatus review.FileStatus

	// === Diff content ===
	diffLines       []string
	diffHighlighted []string
	diffCursor      int
	visualMode      bool
	visualStart     int

	// The diff range actually shown (may differ from from/to for "changed" files)
	currentFrom string
	currentTo   string

	// === Input state ===
	inputBuffer    string
	pendingZ       bool
	pendingBracket rune // ']' or '[' for ]c/[c hunk navigation

	// === Layout ===
	focus     Focus
	showHelp  bool
	flatMode  bool
	splitView bool
	splitOffset int
	width, height int

	// === Git ===
	gitClient     *git.Client
	currentBranch string
	repoName      string
	headCommit    string
	from, to      string // main diff range
	rangeLabel    string

	// === Stats ===
	statsAdded, statsDeleted             int
	currentFileAdded, currentFileDeleted int
	fileStats                            map[string][2]int

	// === Review ===
	reviewState  *review.State
	gitDir       string
	// in-memory computed status (includes "changed" which is not stored)
	computedStatuses map[string]review.FileStatus
	// comments indexed by line index for the current file
	lineComments map[int]*review.Comment

	// === Theme ===
	themeIndex  int
	activeTheme Theme

	// === Config ===
	cfg config.Config

	// === Overlay ===
	overlay       OverlayMode
	confirmMsg    string
	confirmAction func() tea.Cmd

	// === Comment input ===
	commentInput       textarea.Model
	commentLineIndex   int
	commentLineContent string

	// === Search ===
	searchMode    bool
	searchInput   textinput.Model
	searchQuery   string
	searchMatches []int
	searchCursor  int

	// === Status bar notification ===
	statusNotify string
}

func NewModel(
	cfg config.Config,
	gitClient *git.Client,
	from, to, rangeLabel string,
	reviewState *review.State,
	gitDir string,
	headCommit string,
	themeIndex int,
) Model {
	theme := Themes[themeIndex%len(Themes)]
	InitStyles(theme)

	currentBranch := gitClient.GetCurrentBranch()
	repoName := gitClient.GetRepoName()

	files, _ := gitClient.ListChangedFiles(from, to)

	// Build computed statuses (start from stored, "changed" is computed on startup)
	computed := make(map[string]review.FileStatus)
	for _, f := range files {
		computed[f] = reviewState.GetFileStatus(f)
	}

	t := tree.New(files)
	items := t.Items(false, computed)

	ta := textarea.New()
	ta.Placeholder = "Write your comment here..."
	ta.SetWidth(60)
	ta.SetHeight(5)
	ta.ShowLineNumbers = false

	si := textinput.New()
	si.Placeholder = "Search..."
	si.CharLimit = 200

	delegate := TreeDelegate{
		Focused:      true,
		FileStatuses: computed,
		ActiveTheme:  theme,
	}

	l := list.New(items, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowPagination(false)
	l.DisableQuitKeybindings()

	m := Model{
		fileList:         l,
		treeState:        t,
		treeDelegate:     delegate,
		diffViewport:     viewport.New(0, 0),
		focus:            FocusTree,
		currentBranch:    currentBranch,
		repoName:         repoName,
		headCommit:       headCommit,
		from:             from,
		to:               to,
		rangeLabel:       rangeLabel,
		gitClient:        gitClient,
		reviewState:      reviewState,
		gitDir:           gitDir,
		computedStatuses: computed,
		lineComments:     make(map[int]*review.Comment),
		themeIndex:       themeIndex,
		activeTheme:      theme,
		cfg:              cfg,
		commentInput:     ta,
		searchInput:      si,
		fileStats:        make(map[string][2]int),
	}

	// Select first non-directory file
	for idx, item := range items {
		if ti, ok := item.(tree.TreeItem); ok && !ti.IsDir {
			m.selectedPath = ti.FullPath
			m.fileList.Select(idx)
			break
		}
	}

	return m
}

func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd

	if m.selectedPath != "" {
		from, to := m.diffRangeForFile(m.selectedPath)
		cmds = append(cmds, m.gitClient.DiffCmd(from, to, m.selectedPath))
	}

	cmds = append(cmds, m.fetchStatsCmd())
	cmds = append(cmds, m.checkAllApprovedFilesCmd())

	return tea.Batch(cmds...)
}

func (m Model) fetchStatsCmd() tea.Cmd {
	return func() tea.Msg {
		added, deleted, err := m.gitClient.DiffStats(m.from, m.to)
		if err != nil {
			return nil
		}
		byFile, _ := m.gitClient.DiffStatsByFile(m.from, m.to)
		return StatsMsg{Added: added, Deleted: deleted, ByFile: byFile}
	}
}

// checkAllApprovedFilesCmd checks each approved file for post-approval changes.
func (m Model) checkAllApprovedFilesCmd() tea.Cmd {
	return func() tea.Msg {
		// Collect all approved files and check for changes
		// Returns a batch of ChangeCheckMsg
		var msgs []tea.Msg
		for file, fs := range m.reviewState.Files {
			if fs.Status == review.StatusApproved && fs.ApprovedAtCommit != "" {
				changed := m.gitClient.HasChangedSince(fs.ApprovedAtCommit, file)
				msgs = append(msgs, ChangeCheckMsg{File: file, Changed: changed})
			}
		}
		return batchMsg(msgs)
	}
}

// batchMsg is a helper to return multiple messages from a single command.
type batchMsg []tea.Msg

func (m *Model) getRepeatCount() int {
	if m.inputBuffer == "" {
		return 1
	}
	count, err := strconv.Atoi(m.inputBuffer)
	if err != nil {
		return 1
	}
	m.inputBuffer = ""
	return count
}

func stripAnsi(str string) string {
	return ansiRe.ReplaceAllString(str, "")
}

func isDiffMetadata(cleanLine string) bool {
	return strings.HasPrefix(cleanLine, "diff --git") ||
		strings.HasPrefix(cleanLine, "index ") ||
		strings.HasPrefix(cleanLine, "new file mode") ||
		strings.HasPrefix(cleanLine, "deleted file mode") ||
		strings.HasPrefix(cleanLine, "old mode") ||
		strings.HasPrefix(cleanLine, "--- a/") ||
		strings.HasPrefix(cleanLine, "--- /dev/") ||
		strings.HasPrefix(cleanLine, "+++ b/") ||
		strings.HasPrefix(cleanLine, "+++ /dev/") ||
		strings.HasPrefix(cleanLine, "Binary files") ||
		strings.HasPrefix(cleanLine, "@@")
}

func isDiffContentLine(cleanLine string) bool {
	cleanLine = strings.TrimRight(cleanLine, "\r")
	return strings.HasPrefix(cleanLine, " ") ||
		strings.HasPrefix(cleanLine, "+") ||
		strings.HasPrefix(cleanLine, "-")
}

func (m *Model) setYOffset(offset int) {
	maxOffset := len(m.diffLines) - m.diffViewport.Height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}
	m.diffViewport.YOffset = offset
}

func (m *Model) snapCursor(idx int, dir int) int {
	if len(m.diffLines) == 0 {
		return 0
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(m.diffLines) {
		idx = len(m.diffLines) - 1
	}

	curr := idx
	for curr >= 0 && curr < len(m.diffLines) {
		cleanLine := stripAnsi(m.diffLines[curr])
		if isDiffContentLine(cleanLine) {
			return curr
		}
		curr += dir
	}

	curr = idx
	for curr >= 0 && curr < len(m.diffLines) {
		cleanLine := stripAnsi(m.diffLines[curr])
		if isDiffContentLine(cleanLine) {
			return curr
		}
		curr -= dir
	}

	return m.diffCursor
}

func (m *Model) handleScrolling() {
	if m.diffCursor < m.diffViewport.YOffset {
		m.setYOffset(m.diffCursor)
	} else if m.diffCursor >= m.diffViewport.YOffset+m.diffViewport.Height {
		m.setYOffset(m.diffCursor - m.diffViewport.Height + 1)
	}
}

func (m *Model) centerDiffCursor() {
	targetOffset := m.diffCursor - (m.diffViewport.Height / 2)
	m.setYOffset(targetOffset)
}

func (m *Model) updateSizes() {
	reservedHeight := 2 // top bar + status bar
	if m.showHelp {
		reservedHeight += 7
	}
	if m.overlay != OverlayNone {
		// overlay doesn't change the underlying size
	}

	contentHeight := m.height - reservedHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	treeWidth := int(float64(m.width) * 0.22)
	if treeWidth < 22 {
		treeWidth = 22
	}

	treePaneOverhead := 4
	treeInnerWidth := treeWidth - treePaneOverhead
	if treeInnerWidth < 10 {
		treeInnerWidth = 10
	}

	listHeight := contentHeight - 2
	if listHeight < 1 {
		listHeight = 1
	}
	m.fileList.SetSize(treeInnerWidth, listHeight)

	diffPaneWidth := m.width - treeWidth
	m.diffViewport.Width = diffPaneWidth
	m.diffViewport.Height = listHeight
}

func (m *Model) updateTreeFocus() {
	m.treeDelegate.Focused = (m.focus == FocusTree)
	m.treeDelegate.FileStatuses = m.computedStatuses
	m.fileList.SetDelegate(m.treeDelegate)
}

func (m *Model) refreshTreeItems() {
	m.fileList.SetItems(m.treeState.Items(m.flatMode, m.computedStatuses))
	// Re-select the current file
	for i, item := range m.fileList.Items() {
		if ti, ok := item.(tree.TreeItem); ok && ti.FullPath == m.selectedPath {
			m.fileList.Select(i)
			break
		}
	}
}

// diffRangeForFile returns the appropriate (from, to) range for a given file.
// For "changed" files (approved but with new commits), shows only the delta.
func (m *Model) diffRangeForFile(file string) (string, string) {
	if m.computedStatuses[file] == review.StatusChanged {
		approvedAt := m.reviewState.GetApprovedAtCommit(file)
		if approvedAt != "" {
			return approvedAt, "HEAD"
		}
	}
	return m.from, m.to
}

// rebuildLineComments rebuilds the lineIndex→comment map for the current file.
func (m *Model) rebuildLineComments() {
	m.lineComments = make(map[int]*review.Comment)
	if m.selectedPath == "" {
		return
	}
	for _, c := range m.reviewState.CommentsForFile(m.selectedPath) {
		m.lineComments[c.DiffLineIndex] = c
	}
}

// saveReviewState persists state to disk, silently ignoring errors.
func (m *Model) saveReviewState() {
	_ = review.Save(m.gitDir, m.reviewState)
}

// findNextUnreviewedFile finds the next (or prev) file that needs review.
func (m *Model) findNextUnreviewedFile(dir int) int {
	items := m.fileList.Items()
	currentIdx := m.fileList.Index()
	n := len(items)
	if n == 0 {
		return -1
	}
	for i := 1; i < n; i++ {
		idx := (currentIdx + dir*i + n) % n
		item, ok := items[idx].(tree.TreeItem)
		if !ok || item.IsDir {
			continue
		}
		status := m.computedStatuses[item.FullPath]
		if status == review.StatusUnreviewed || status == review.StatusChanged {
			return idx
		}
	}
	return -1
}

// jumpToNextHunk moves cursor to the first content line after the next @@ hunk header.
func (m *Model) jumpToNextHunk() {
	for i := m.diffCursor + 1; i < len(m.diffLines); i++ {
		clean := stripAnsi(m.diffLines[i])
		if strings.HasPrefix(clean, "@@") {
			newCursor := m.snapCursor(i+1, 1)
			if newCursor != m.diffCursor {
				m.diffCursor = newCursor
				m.handleScrolling()
				return
			}
		}
	}
}

// jumpToPrevHunk moves cursor to the first content line after the preceding @@ header.
func (m *Model) jumpToPrevHunk() {
	// Find the @@ before current cursor
	for i := m.diffCursor - 1; i >= 0; i-- {
		clean := stripAnsi(m.diffLines[i])
		if strings.HasPrefix(clean, "@@") {
			newCursor := m.snapCursor(i+1, 1)
			m.diffCursor = newCursor
			m.handleScrolling()
			return
		}
	}
}

// reviewSummary returns counts of files by status.
func (m *Model) reviewSummary() (approved, changed, viewed, total int) {
	for _, item := range m.fileList.Items() {
		ti, ok := item.(tree.TreeItem)
		if !ok || ti.IsDir {
			continue
		}
		total++
		switch m.computedStatuses[ti.FullPath] {
		case review.StatusApproved:
			approved++
		case review.StatusChanged:
			changed++
		case review.StatusViewed:
			viewed++
		}
	}
	return
}
