package git

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ansiRe matches ANSI escape sequences (colors, cursor movement, etc.)
var ansiRe = regexp.MustCompile(`[][[\]()#;?]*(?:(?:(?:[a-zA-Z\d]*(?:;[a-zA-Z\d]*)*)?)|(?:(?:\d{1,4}(?:;\d{0,4})*)?[\dA-PRZcf-ntqry=><~]))`)

// hunkHeaderRe matches unified diff hunk headers and captures the new-file start line.
var hunkHeaderRe = regexp.MustCompile(`^@@ \-\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

type DiffMsg struct{ Content string }
type EditorFinishedMsg struct{ Err error }
type FzfResultMsg struct {
	File  string
	Index int
}

type DiffLine struct {
	File    string
	Index   int
	Content string
}

type Client struct{}

func (c *Client) gitCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("git", append([]string{"--no-pager"}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd
}

// gitOutput runs a git command and returns trimmed stdout, or an error.
func (c *Client) gitOutput(args ...string) (string, error) {
	out, err := c.gitCmd(args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *Client) GetCurrentBranch() string {
	out, err := c.gitOutput("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "HEAD"
	}
	return out
}

func (c *Client) GetRepoName() string {
	out, err := c.gitOutput("rev-parse", "--show-toplevel")
	if err != nil {
		return "repo"
	}
	parts := strings.Split(out, "/")
	return parts[len(parts)-1]
}

func (c *Client) GetGitDir() string {
	out, err := c.gitOutput("rev-parse", "--git-dir")
	if err != nil {
		return ".git"
	}
	return out
}

func (c *Client) HeadCommit() string {
	out, _ := c.gitOutput("rev-parse", "HEAD")
	return out
}

func (c *Client) ResolveRef(ref string) (string, error) {
	out, err := c.gitOutput("rev-parse", ref)
	if err != nil {
		return "", fmt.Errorf("cannot resolve ref %q: %w", ref, err)
	}
	return out, nil
}

func (c *Client) MergeBase(ref1, ref2 string) (string, error) {
	out, err := c.gitOutput("merge-base", ref1, ref2)
	if err != nil {
		return "", fmt.Errorf("git merge-base %s %s: %w", ref1, ref2, err)
	}
	return out, nil
}

func (c *Client) AutoDetectBase(override string) string {
	if override != "" {
		return override
	}
	// Try common default branches in order; fall back to HEAD if none exist.
	for _, candidate := range []string{"main", "master", "develop"} {
		if _, err := c.gitCmd("rev-parse", "--verify", candidate).Output(); err == nil {
			return candidate
		}
	}
	return "HEAD"
}

// When to is "HEAD", omit it so git diff compares against the working tree.
func diffArg(from, to string) string {
	if to == "HEAD" {
		return from
	}
	return from + ".." + to
}

// truncateSHA returns the first 7 characters of a SHA, or the full string if shorter.
func truncateSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func appendUniqueLines(files []string, seen map[string]bool, data []byte) []string {
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		if f := strings.TrimSpace(line); f != "" && !seen[f] {
			seen[f] = true
			files = append(files, f)
		}
	}
	return files
}

func (c *Client) ListChangedFiles(from, to string) ([]string, error) {
	var out []byte
	var err error
	if to == "--cached" {
		out, err = c.gitCmd("diff", "--cached", "--name-only").Output()
	} else {
		out, err = c.gitCmd("diff", "--name-only", diffArg(from, to)).Output()
	}
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	files := appendUniqueLines(nil, seen, out)

	// Untracked files are best-effort; silently skip if unavailable.
	if to == "HEAD" {
		if untracked, err := c.gitCmd("ls-files", "--others", "--exclude-standard").Output(); err == nil {
			files = appendUniqueLines(files, seen, untracked)
		}
	}

	return files, nil
}

// rawDiff fetches the diff output for a single file, falling back to --no-index
// for untracked files that produce an empty diff against HEAD.
func (c *Client) rawDiff(from, to, path string) []byte {
	var out []byte
	if to == "--cached" {
		out, _ = c.gitCmd("diff", "--cached", "--", path).Output()
	} else {
		out, _ = c.gitCmd("diff", diffArg(from, to), "--", path).Output()
		if len(out) == 0 && to == "HEAD" {
			if _, err := os.Stat(path); err == nil {
				out, _ = c.gitCmd("diff", "--no-index", "/dev/null", path).Output()
			}
		}
	}
	return out
}

func (c *Client) DiffCmd(from, to, path string) tea.Cmd {
	return func() tea.Msg {
		out := c.rawDiff(from, to, path)
		if len(out) == 0 {
			return DiffMsg{Content: ""}
		}
		return DiffMsg{Content: string(out)}
	}
}

// OpenEditorCmd returns a BubbleTea command that opens path at lineNumber in editor.
func OpenEditorCmd(path string, lineNumber int, editor string) tea.Cmd {
	var args []string
	if lineNumber > 0 {
		args = append(args, fmt.Sprintf("+%d", lineNumber))
	}
	args = append(args, path)

	cmd := exec.Command(editor, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return EditorFinishedMsg{Err: err}
	})
}

func (c *Client) numstat(from, to string) ([]byte, error) {
	if to == "--cached" {
		return c.gitCmd("diff", "--cached", "--numstat").Output()
	}
	return c.gitCmd("diff", "--numstat", diffArg(from, to)).Output()
}

// parseNumstatLine parses one line of git diff --numstat output.
func parseNumstatLine(line string) (added, deleted int, filePath string, ok bool) {
	if line == "" {
		return
	}
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return
	}
	if parts[0] != "-" {
		added, _ = strconv.Atoi(parts[0])
	}
	if parts[1] != "-" {
		deleted, _ = strconv.Atoi(parts[1])
	}
	filePath = strings.Join(parts[2:], " ")
	if idx := strings.LastIndex(filePath, " => "); idx != -1 {
		filePath = filePath[idx+4:]
	}
	ok = true
	return
}

func (c *Client) DiffStats(from, to string) (added int, deleted int, err error) {
	out, err := c.numstat(from, to)
	if err != nil {
		return 0, 0, fmt.Errorf("git diff numstat: %w", err)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		a, d, _, ok := parseNumstatLine(line)
		if ok {
			added += a
			deleted += d
		}
	}
	return added, deleted, nil
}

func (c *Client) DiffStatsByFile(from, to string) (map[string][2]int, error) {
	out, err := c.numstat(from, to)
	if err != nil {
		return nil, fmt.Errorf("git diff numstat: %w", err)
	}
	result := make(map[string][2]int)
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		a, d, filePath, ok := parseNumstatLine(line)
		if ok {
			result[filePath] = [2]int{a, d}
		}
	}
	return result, nil
}

func (c *Client) GetBlobHash(ref, path string) (string, error) {
	var gitRef string
	if ref == "--cached" {
		gitRef = ":" + path
	} else {
		gitRef = ref + ":" + path
	}
	out, err := c.gitOutput("rev-parse", gitRef)
	if err != nil {
		return "", fmt.Errorf("blob hash %s:%s: %w", ref, path, err)
	}
	return out, nil
}

// RefEntry represents a git ref (branch or commit) for display in the range picker.
type RefEntry struct {
	FullSHA  string
	ShortSHA string
	Ref      string // branch name if available, else full SHA
	Display  string
	IsBranch bool
	IsHead   bool
}

// ListBranchesAndCommits returns branches (by name) followed by recent commits not
// already represented by a branch tip, decorated with branch names and HEAD indicator.
func (c *Client) ListBranchesAndCommits(n int) ([]RefEntry, error) {
	seen := make(map[string]bool)
	var entries []RefEntry

	headSHA, _ := c.ResolveRef("HEAD")

	branchOut, _ := c.gitCmd("branch", "--format=%(refname:short)%09%(objectname)").Output()
	for line := range strings.SplitSeq(strings.TrimSpace(string(branchOut)), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "\t", 2)
		if len(parts) != 2 || parts[0] == "" {
			continue
		}
		name, sha := parts[0], parts[1]
		short := truncateSHA(sha)
		isHead := sha == headSHA || short == truncateSHA(headSHA)
		display := name + " (" + short + ")"
		if isHead {
			display += " (HEAD)"
		}
		entries = append(entries, RefEntry{
			FullSHA:  sha,
			ShortSHA: short,
			Ref:      name,
			Display:  display,
			IsBranch: true,
			IsHead:   isHead,
		})
		seen[sha] = true
		seen[short] = true
	}

	logOut, err := c.gitCmd("log", fmt.Sprintf("-n%d", n), "--format=%H%x00%h%x00%D").Output()
	if err != nil {
		return entries, nil
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(logOut)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 3)
		if len(parts) < 2 {
			continue
		}
		fullSHA := strings.TrimSpace(parts[0])
		shortSHA := strings.TrimSpace(parts[1])
		if seen[fullSHA] || seen[shortSHA] {
			continue
		}
		seen[fullSHA] = true

		isHead := fullSHA == headSHA
		decor := ""
		if len(parts) == 3 {
			decor = strings.TrimSpace(parts[2])
			if strings.HasPrefix(decor, "HEAD -> ") {
				isHead = true
				decor = decor[8:]
			} else if decor == "HEAD" {
				isHead = true
				decor = ""
			}
		}

		display := shortSHA
		if decor != "" {
			display += " (" + decor + ")"
		}
		if isHead {
			display += " (HEAD)"
		}

		entries = append(entries, RefEntry{
			FullSHA:  fullSHA,
			ShortSHA: shortSHA,
			Ref:      fullSHA,
			Display:  display,
			IsHead:   isHead,
		})
	}

	return entries, nil
}

func (c *Client) HasChangedSince(commitSHA, path string) bool {
	out, err := c.gitCmd("diff", "--name-only", commitSHA, "--", path).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

func (c *Client) AllDiffLines(from, to string) ([]DiffLine, error) {
	files, err := c.ListChangedFiles(from, to)
	if err != nil {
		return nil, err
	}

	var result []DiffLine
	for _, file := range files {
		out := c.rawDiff(from, to, file)
		if len(out) == 0 {
			continue
		}
		for i, line := range strings.Split(string(out), "\n") {
			clean := StripAnsi(line)
			if clean == "" {
				continue
			}
			if strings.HasPrefix(clean, "+") && !strings.HasPrefix(clean, "+++") {
				result = append(result, DiffLine{File: file, Index: i, Content: "+" + clean[1:]})
			} else if strings.HasPrefix(clean, "-") && !strings.HasPrefix(clean, "---") {
				result = append(result, DiffLine{File: file, Index: i, Content: "-" + clean[1:]})
			}
		}
	}
	return result, nil
}

func (c *Client) CalculateFileLine(diffLines []string, visualLineIndex int) int {
	if len(diffLines) == 0 {
		return 1
	}
	visualLineIndex = max(0, min(visualLineIndex, len(diffLines)-1))

	currentLineNo := 1
	mappedLineNo := 1
	inHunk := false

	for i := 0; i <= visualLineIndex; i++ {
		cleanLine := strings.TrimRight(StripAnsi(diffLines[i]), "\r")

		if matches := hunkHeaderRe.FindStringSubmatch(cleanLine); len(matches) > 1 {
			startLine, _ := strconv.Atoi(matches[1])
			if startLine < 1 {
				startLine = 1
			}
			currentLineNo = startLine
			mappedLineNo = currentLineNo
			inHunk = true
			continue
		}

		if !inHunk {
			continue
		}

		switch {
		case strings.HasPrefix(cleanLine, " "), strings.HasPrefix(cleanLine, "+"):
			mappedLineNo = currentLineNo
			currentLineNo++
		case strings.HasPrefix(cleanLine, "-"):
			mappedLineNo = currentLineNo
		}
	}

	if mappedLineNo < 1 {
		return 1
	}
	return mappedLineNo
}

func (c *Client) ParseFilesFromDiff(diffText string) []string {
	var files []string
	seen := make(map[string]bool)
	for line := range strings.SplitSeq(diffText, "\n") {
		if strings.HasPrefix(line, "diff --git a/") {
			parts := strings.SplitN(line, " b/", 2)
			if len(parts) == 2 {
				file := strings.TrimPrefix(parts[0], "diff --git a/")
				if !seen[file] {
					seen[file] = true
					files = append(files, file)
				}
			}
		}
	}
	return files
}

func StripAnsi(str string) string {
	return ansiRe.ReplaceAllString(str, "")
}
