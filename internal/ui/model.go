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
	"github.com/YusufHosny/git-review/internal/config"
	"github.com/YusufHosny/git-review/internal/git"
	"github.com/YusufHosny/git-review/internal/review"
	"github.com/YusufHosny/git-review/internal/tree"
)

type Focus int

const (
	FocusTree Focus = iota
	FocusDiff
)

var ansiRe = regexp.MustCompile(`\x1b\[[\x30-\x3f]*[\x20-\x2f]*[\x40-\x7e]`)
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
	fileList     list.Model
	treeState    *tree.FileTree
	treeDelegate TreeDelegate
	diffViewport viewport.Model

	selectedPath   string
	selectedStatus review.FileStatus

	diffLines       []string
	diffHighlighted []string
	diffCursor      int
	visualMode      bool
	visualStart     int

	currentFrom string
	currentTo   string

	inputBuffer    string
	pendingZ       bool
	pendingBracket rune

	focus     Focus
	showHelp  bool
	flatMode  bool
	splitView bool
	splitOffset int
	width, height int

	gitClient     *git.Client
	currentBranch string
	repoName      string
	headCommit    string
	from, to      string
	rangeLabel    string

	statsAdded, statsDeleted             int
	currentFileAdded, currentFileDeleted int
	fileStats                            map[string][2]int

	reviewState      *review.State
	gitDir           string
	computedStatuses map[string]review.FileStatus
	lineComments     map[int]*review.Comment

	themeIndex        int
	activeTheme       Theme
	themePickerCursor int

	cfg config.Config

	overlay       OverlayMode
	confirmMsg    string
	confirmAction func() tea.Cmd

	commentInput       textarea.Model
	commentLineIndex   int
	commentLineContent string

	searchMode    bool
	searchInput   textinput.Model
	searchQuery   string
	searchMatches []int
	searchCursor  int

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

func (m Model) checkAllApprovedFilesCmd() tea.Cmd {
	return func() tea.Msg {
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
	m.diffViewport.YOffset = max(0, min(offset, m.maxScrollOffset()))
}

// codeMaxW returns the maximum code column width given the current viewport width.
// Must match the calculation in renderUnifiedDiff.
func (m *Model) codeMaxW() int {
	return max(m.diffViewport.Width-13, 1)
}

func (m *Model) visualRowsForLine(i int) int {
	if i < 0 || i >= len(m.diffLines) {
		return 0
	}
	cleanLine := stripAnsi(m.diffLines[i])
	if isDiffMetadata(cleanLine) {
		if strings.HasPrefix(cleanLine, "@@") {
			return 1
		}
		return 0
	}
	codeContent := cleanLine
	if len(codeContent) > 0 {
		codeContent = codeContent[1:]
	}
	codeMaxW := m.codeMaxW()
	w := len([]rune(codeContent))
	var rows int
	if w == 0 || codeMaxW == 0 {
		rows = 1
	} else {
		rows = (w + codeMaxW - 1) / codeMaxW
	}
	if m.cfg.UI.ShowCommentsInline {
		if _, hasComment := m.lineComments[i]; hasComment {
			rows++
		}
	}
	return rows
}

func (m *Model) diffContentHeight() int {
	h := m.diffViewport.Height
	if m.computedStatuses[m.selectedPath] == review.StatusChanged && h > 0 {
		h--
	}
	return h
}

func (m *Model) maxScrollOffset() int {
	h := m.diffContentHeight()
	if h <= 0 || len(m.diffLines) == 0 {
		return 0
	}
	needed := h
	for i := len(m.diffLines) - 1; i >= 0; i-- {
		needed -= m.visualRowsForLine(i)
		if needed <= 0 {
			return i
		}
	}
	return 0
}

func (m *Model) offsetToShowCursorAtBottom() int {
	h := m.diffContentHeight()
	if h <= 0 {
		return m.diffCursor
	}
	needed := h
	for i := m.diffCursor; i >= 0; i-- {
		needed -= m.visualRowsForLine(i)
		if needed <= 0 {
			return i
		}
	}
	return 0
}

func (m *Model) snapCursor(idx int, dir int) int {
	if len(m.diffLines) == 0 {
		return 0
	}
	idx = max(0, min(idx, len(m.diffLines)-1))

	for curr := idx; curr >= 0 && curr < len(m.diffLines); curr += dir {
		if isDiffContentLine(stripAnsi(m.diffLines[curr])) {
			return curr
		}
	}

	for curr := idx; curr >= 0 && curr < len(m.diffLines); curr -= dir {
		if isDiffContentLine(stripAnsi(m.diffLines[curr])) {
			return curr
		}
	}

	return m.diffCursor
}

func (m *Model) handleScrolling() {
	if m.diffCursor < m.diffViewport.YOffset {
		m.setYOffset(m.diffCursor)
		return
	}
	h := m.diffContentHeight()
	visualRows := 0
	for i := m.diffViewport.YOffset; i <= m.diffCursor; i++ {
		visualRows += m.visualRowsForLine(i)
	}
	if visualRows > h {
		m.setYOffset(m.offsetToShowCursorAtBottom())
	}
}

func (m *Model) centerDiffCursor() {
	needed := m.diffContentHeight() / 2
	for i := m.diffCursor; i >= 0; i-- {
		needed -= m.visualRowsForLine(i)
		if needed <= 0 {
			m.setYOffset(i)
			return
		}
	}
	m.setYOffset(0)
}

func (m *Model) updateSizes() {
	reservedHeight := 2
	if m.showHelp {
		reservedHeight += 7
	}

	contentHeight := max(m.height-reservedHeight, 1)
	treeWidth := max(int(float64(m.width)*0.22), 22)
	treeInnerWidth := max(treeWidth-4, 10)
	listHeight := max(contentHeight-2, 1)

	m.fileList.SetSize(treeInnerWidth, listHeight)
	m.diffViewport.Width = m.width - treeWidth
	m.diffViewport.Height = listHeight
}

func (m *Model) updateTreeFocus() {
	m.treeDelegate.Focused = (m.focus == FocusTree)
	m.treeDelegate.FileStatuses = m.computedStatuses
	m.fileList.SetDelegate(m.treeDelegate)
}

func (m *Model) refreshTreeItems() {
	m.fileList.SetItems(m.treeState.Items(m.flatMode, m.computedStatuses))
	for i, item := range m.fileList.Items() {
		if ti, ok := item.(tree.TreeItem); ok && ti.FullPath == m.selectedPath {
			m.fileList.Select(i)
			break
		}
	}
}

func (m *Model) diffRangeForFile(file string) (string, string) {
	if m.computedStatuses[file] == review.StatusChanged {
		if approvedAt := m.reviewState.GetApprovedAtCommit(file); approvedAt != "" {
			return approvedAt, "HEAD"
		}
	}
	return m.from, m.to
}

func (m *Model) rebuildLineComments() {
	m.lineComments = make(map[int]*review.Comment)
	if m.selectedPath == "" {
		return
	}
	for _, c := range m.reviewState.CommentsForFile(m.selectedPath) {
		m.lineComments[c.DiffLineIndex] = c
	}
}

func (m *Model) saveReviewState() {
	_ = review.Save(m.gitDir, m.reviewState)
}

func (m *Model) setFileStatus(status review.FileStatus, commit string) {
	if m.selectedPath == "" || m.isDir() {
		return
	}
	m.computedStatuses[m.selectedPath] = status
	m.reviewState.SetFileStatus(m.selectedPath, status, commit)
	m.saveReviewState()
	m.updateTreeFocus()
	m.refreshTreeItems()
}

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

func (m *Model) jumpToNextHunk() {
	for i := m.diffCursor + 1; i < len(m.diffLines); i++ {
		if strings.HasPrefix(stripAnsi(m.diffLines[i]), "@@") {
			if newCursor := m.snapCursor(i+1, 1); newCursor != m.diffCursor {
				m.diffCursor = newCursor
				m.handleScrolling()
				return
			}
		}
	}
}

func (m *Model) jumpToPrevHunk() {
	for i := m.diffCursor - 1; i >= 0; i-- {
		if strings.HasPrefix(stripAnsi(m.diffLines[i]), "@@") {
			m.diffCursor = m.snapCursor(i+1, 1)
			m.handleScrolling()
			return
		}
	}
}

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
