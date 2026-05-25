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
	"github.com/charmbracelet/lipgloss"
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

type FilterTab int

const (
	FilterNotApproved FilterTab = iota
	FilterAll
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

type notifyClearMsg struct{}

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

	gitClient      *git.Client
	currentBranch  string
	repoName       string
	headCommit     string
	from, to       string
	rangeLabel     string
	baseBranchName string

	statsAdded, statsDeleted             int
	currentFileAdded, currentFileDeleted int
	fileStats                            map[string][2]int

	reviewState      *review.State
	gitDir           string
	computedStatuses map[string]review.FileStatus
	lineComments     map[int]*review.Comment

	isDirtyMode    bool
	showCommit     string
	filterTab      FilterTab
	sessionApproved map[string]bool

	rangePickerFocus   int
	rangePickerItems   []git.RefEntry
	rangePickerBaseIdx int
	rangePickerHeadIdx int

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
	baseBranchName string,
	reviewState *review.State,
	gitDir string,
	headCommit string,
	themeIndex int,
	isDirtyMode bool,
	showCommit string,
) Model {
	theme := Themes[themeIndex%len(Themes)]
	InitStyles(theme)

	currentBranch := gitClient.GetCurrentBranch()
	repoName := gitClient.GetRepoName()

	var files []string
	if showCommit != "" {
		files, _ = gitClient.ListChangedFilesShow(showCommit)
	} else {
		files, _ = gitClient.ListChangedFiles(from, to)
	}

	computed := make(map[string]review.FileStatus)
	for _, f := range files {
		if isDirtyMode {
			computed[f] = review.StatusUnreviewed
		} else {
			computed[f] = reviewState.GetFileStatus(f)
		}
	}

	t := tree.New(files)
	allItems := t.Items(false, computed)

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

	l := list.New(allItems, delegate, 0, 0)
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
		baseBranchName:   baseBranchName,
		gitClient:        gitClient,
		reviewState:      reviewState,
		gitDir:           gitDir,
		computedStatuses: computed,
		lineComments:     make(map[int]*review.Comment),
		sessionApproved:  make(map[string]bool),
		themeIndex:       themeIndex,
		activeTheme:      theme,
		cfg:              cfg,
		commentInput:     ta,
		searchInput:      si,
		fileStats:        make(map[string][2]int),
		isDirtyMode:      isDirtyMode,
		showCommit:       showCommit,
		filterTab:        FilterNotApproved,
	}

	// Apply filter and select the first visible file.
	m.refreshTreeItems()
	for idx, item := range m.fileList.Items() {
		if ti, ok := item.(tree.TreeItem); ok && !ti.IsDir {
			m.selectedPath = ti.FullPath
			m.fileList.Select(idx)
			break
		}
	}

	// currentFrom/currentTo must be set before Init fires DiffCmd so that the
	// returning DiffMsg passes the stale-check in the Update handler.
	if m.selectedPath != "" {
		m.currentFrom, m.currentTo = m.diffRangeForFile(m.selectedPath)
	}

	m.applyThemeToCommentInput()
	return m
}

func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd

	if m.selectedPath != "" {
		cmds = append(cmds, m.fetchDiffCmd(m.selectedPath))
	}

	cmds = append(cmds, m.fetchStatsCmd())
	if !m.isDirtyMode && m.showCommit == "" {
		cmds = append(cmds, m.checkAllApprovedFilesCmd())
	}

	return tea.Batch(cmds...)
}

func (m Model) fetchStatsCmd() tea.Cmd {
	return func() tea.Msg {
		var added, deleted int
		var byFile map[string][2]int
		var err error
		if m.showCommit != "" {
			added, deleted, err = m.gitClient.DiffStatsShow(m.showCommit)
			if err == nil {
				byFile, _ = m.gitClient.DiffStatsByFileShow(m.showCommit)
			}
		} else {
			added, deleted, err = m.gitClient.DiffStats(m.from, m.to)
			if err == nil {
				byFile, _ = m.gitClient.DiffStatsByFile(m.from, m.to)
			}
		}
		if err != nil {
			return nil
		}
		return StatsMsg{Added: added, Deleted: deleted, ByFile: byFile}
	}
}

func (m *Model) fetchDiffCmd(path string) tea.Cmd {
	if m.showCommit != "" {
		return m.gitClient.DiffShowCmd(m.showCommit, path)
	}
	from, to := m.diffRangeForFile(path)
	return m.gitClient.DiffCmd(from, to, path)
}

func (m Model) checkAllApprovedFilesCmd() tea.Cmd {
	return func() tea.Msg {
		var msgs []tea.Msg
		for file, fs := range m.reviewState.Files {
			if fs.Status != review.StatusApproved {
				continue
			}
			var changed bool
			if fs.ApprovedBlobHash != "" {
				currentHash, err := m.gitClient.GetBlobHash(m.to, file)
				changed = err != nil || currentHash != fs.ApprovedBlobHash
			} else if fs.ApprovedAtCommit != "" {
				changed = m.gitClient.HasChangedSince(fs.ApprovedAtCommit, file)
			}
			msgs = append(msgs, ChangeCheckMsg{File: file, Changed: changed})
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
	rows := len(wrapCodeRows(codeContent, codeMaxW))
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
	m.diffViewport.Width = m.width - treeWidth - 1
	m.diffViewport.Height = listHeight
}

func (m *Model) updateTreeFocus() {
	m.treeDelegate.Focused = (m.focus == FocusTree)
	m.treeDelegate.FileStatuses = m.computedStatuses
	m.fileList.SetDelegate(m.treeDelegate)
}

func (m *Model) buildFileListItems() []list.Item {
	items := m.treeState.Items(m.flatMode, m.computedStatuses)
	if m.filterTab == FilterAll {
		return items
	}

	// Collect paths of files that should be visible.
	activePaths := make(map[string]bool)
	for _, item := range items {
		ti, ok := item.(tree.TreeItem)
		if !ok || ti.IsDir {
			continue
		}
		status := m.computedStatuses[ti.FullPath]
		// Include Viewed too — auto-view-on-navigate would otherwise make every
		// file disappear from the list as the user scrolls past it.
		if status == review.StatusUnreviewed || status == review.StatusChanged || status == review.StatusViewed {
			activePaths[ti.FullPath] = true
		}
	}
	// Always keep the currently selected file visible.
	if m.selectedPath != "" {
		activePaths[m.selectedPath] = true
	}
	// Keep files approved this session visible until the app closes.
	for path := range m.sessionApproved {
		activePaths[path] = true
	}

	var filtered []list.Item
	for _, item := range items {
		ti, ok := item.(tree.TreeItem)
		if !ok {
			filtered = append(filtered, item)
			continue
		}
		if ti.IsDir {
			// Only keep directory nodes that contain at least one active file.
			dirPrefix := ti.FullPath + "/"
			hasChild := false
			for path := range activePaths {
				if strings.HasPrefix(path, dirPrefix) {
					hasChild = true
					break
				}
			}
			if hasChild {
				filtered = append(filtered, item)
			}
		} else if activePaths[ti.FullPath] {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (m *Model) refreshTreeItems() {
	m.fileList.SetItems(m.buildFileListItems())
	for i, item := range m.fileList.Items() {
		if ti, ok := item.(tree.TreeItem); ok && ti.FullPath == m.selectedPath {
			m.fileList.Select(i)
			break
		}
	}
}

// reloadRange switches to a new diff range, recomputing statuses using blob hashes.
func (m *Model) reloadRange(from, to, label string) tea.Cmd {
	m.from = from
	m.to = to
	m.rangeLabel = label
	// If the new from is a branch name (not a raw SHA), use it as baseBranchName.
	if len(from) != 40 {
		m.baseBranchName = from
	}

	files, _ := m.gitClient.ListChangedFiles(from, to)

	newComputed := make(map[string]review.FileStatus)
	for _, f := range files {
		newComputed[f] = review.StatusUnreviewed
	}

	for f, fs := range m.reviewState.Files {
		if _, inRange := newComputed[f]; !inRange {
			continue
		}
		if fs.Status == review.StatusApproved {
			if fs.ApprovedBlobHash != "" {
				currentHash, err := m.gitClient.GetBlobHash(to, f)
				if err == nil && currentHash == fs.ApprovedBlobHash {
					newComputed[f] = review.StatusApproved
				} else {
					newComputed[f] = review.StatusChanged
				}
			} else if fs.ApprovedAtCommit != "" {
				if m.gitClient.HasChangedSince(fs.ApprovedAtCommit, f) {
					newComputed[f] = review.StatusChanged
				} else {
					newComputed[f] = review.StatusApproved
				}
			}
		} else if fs.Status != review.StatusUnreviewed {
			newComputed[f] = fs.Status
		}
	}

	m.computedStatuses = newComputed
	m.treeState = tree.New(files)
	m.updateTreeFocus()
	m.refreshTreeItems()

	m.reviewState.RangeFrom = from
	m.reviewState.RangeTo = to

	from2, to2 := m.diffRangeForFile(m.selectedPath)
	m.currentFrom, m.currentTo = from2, to2

	return tea.Batch(
		m.gitClient.DiffCmd(from2, to2, m.selectedPath),
		m.fetchStatsCmd(),
	)
}

// prettyRef formats a git ref for human display.
// role "base" → prefer "branchname (REVBASE)", role "head" → prefer "branchname (HEAD)".
func (m *Model) prettyRef(ref, role string) string {
	switch ref {
	case "--cached":
		return "index (staged)"
	case "HEAD":
		if role == "head" {
			return m.currentBranch + " (HEAD)"
		}
		return m.currentBranch
	}

	// Matches current HEAD commit SHA.
	if m.headCommit != "" && ref == m.headCommit {
		return m.currentBranch + " (HEAD)"
	}

	// Full 40-char SHA.
	if len(ref) == 40 {
		short := ref[:8]
		if role == "base" && m.baseBranchName != "" {
			return m.baseBranchName + " (REVBASE)"
		}
		if role == "head" {
			return m.currentBranch + " (HEAD)"
		}
		return short
	}

	// Already a readable ref (branch name, user-typed value, etc.).
	switch role {
	case "base":
		if m.baseBranchName != "" {
			return ref + " (REVBASE)"
		}
	case "head":
		return ref + " (HEAD)"
	}
	return ref
}

func (m *Model) diffRangeForFile(file string) (string, string) {
	if m.showCommit == "" && m.computedStatuses[file] == review.StatusChanged {
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

func (m *Model) applyThemeToCommentInput() {
	bg := m.activeTheme.TopBarBg
	fg := m.activeTheme.NormalText
	dim := m.activeTheme.DimText

	base := lipgloss.NewStyle().Background(bg).Foreground(fg)
	focused := textarea.Style{
		Base:             base,
		CursorLine:       lipgloss.NewStyle().Background(m.activeTheme.CursorCtxBg).Foreground(fg),
		CursorLineNumber: base.Foreground(dim),
		EndOfBuffer:      base.Foreground(dim),
		LineNumber:       base.Foreground(dim),
		Placeholder:      base.Foreground(dim),
		Prompt:           base.Foreground(m.activeTheme.AccentText),
		Text:             base,
	}
	blurred := textarea.Style{
		Base:             base,
		CursorLine:       base,
		CursorLineNumber: base.Foreground(dim),
		EndOfBuffer:      base.Foreground(dim),
		LineNumber:       base.Foreground(dim),
		Placeholder:      base.Foreground(dim),
		Prompt:           base.Foreground(dim),
		Text:             base.Foreground(dim),
	}
	m.commentInput.FocusedStyle = focused
	m.commentInput.BlurredStyle = blurred
	m.commentInput.Cursor.Style = lipgloss.NewStyle().Foreground(m.activeTheme.AccentText)
	m.commentInput.Cursor.TextStyle = base
}

func (m *Model) saveReviewState() {
	if m.isDirtyMode {
		return
	}
	_ = review.Save(m.gitDir, m.reviewState)
}

func (m *Model) fetchDiffForSelected() tea.Cmd {
	from, to := m.diffRangeForFile(m.selectedPath)
	m.currentFrom, m.currentTo = from, to
	return m.fetchDiffCmd(m.selectedPath)
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
	total = len(m.computedStatuses)
	for _, status := range m.computedStatuses {
		switch status {
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
