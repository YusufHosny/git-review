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

var ansiRe = regexp.MustCompile("[][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")
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

func (c *Client) GetCurrentBranch() string {
	out, err := c.gitCmd("rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "HEAD"
	}
	return strings.TrimSpace(string(out))
}

func (c *Client) GetRepoName() string {
	out, err := c.gitCmd("rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "repo"
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "/")
	return parts[len(parts)-1]
}

func (c *Client) GetGitDir() string {
	out, err := c.gitCmd("rev-parse", "--git-dir").Output()
	if err != nil {
		return ".git"
	}
	return strings.TrimSpace(string(out))
}

func (c *Client) HeadCommit() string {
	out, err := c.gitCmd("rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (c *Client) ResolveRef(ref string) (string, error) {
	out, err := c.gitCmd("rev-parse", ref).Output()
	if err != nil {
		return "", fmt.Errorf("cannot resolve ref %q: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *Client) MergeBase(ref1, ref2 string) (string, error) {
	out, err := c.gitCmd("merge-base", ref1, ref2).Output()
	if err != nil {
		return "", fmt.Errorf("git merge-base %s %s: %w", ref1, ref2, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *Client) AutoDetectBase(override string) string {
	if override != "" {
		return override
	}
	for _, candidate := range []string{"main", "master", "develop"} {
		if _, err := c.gitCmd("rev-parse", "--verify", candidate).Output(); err == nil {
			return candidate
		}
	}
	return "HEAD"
}

// When to is "HEAD", omit it so git diff compares against the working tree (not just committed changes).
func diffArg(from, to string) string {
	if to == "HEAD" {
		return from
	}
	return from + ".." + to
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

	if to == "HEAD" {
		if untracked, err := c.gitCmd("ls-files", "--others", "--exclude-standard").Output(); err == nil {
			files = appendUniqueLines(files, seen, untracked)
		}
	}

	return files, nil
}

func (c *Client) DiffCmd(from, to, path string) tea.Cmd {
	return func() tea.Msg {
		var out []byte
		var err error
		if to == "--cached" {
			out, err = c.gitCmd("diff", "--cached", "--", path).Output()
		} else {
			out, err = c.gitCmd("diff", diffArg(from, to), "--", path).Output()
		}
		if err != nil {
			return DiffMsg{Content: "Error fetching diff: " + err.Error()}
		}
		content := string(out)
		// Untracked files produce an empty diff — fall back to no-index diff.
		if content == "" && to == "HEAD" {
			if _, statErr := os.Stat(path); statErr == nil {
				out, _ = c.gitCmd("diff", "--no-index", "/dev/null", path).Output()
				content = string(out)
			}
		}
		return DiffMsg{Content: content}
	}
}

func (c *Client) OpenEditorCmd(path string, lineNumber int, editor string) tea.Cmd {
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

func (c *Client) DiffStats(from, to string) (added int, deleted int, err error) {
	out, err := c.numstat(from, to)
	if err != nil {
		return 0, 0, fmt.Errorf("git diff numstat: %w", err)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		if parts[0] != "-" {
			if n, e := strconv.Atoi(parts[0]); e == nil {
				added += n
			}
		}
		if parts[1] != "-" {
			if n, e := strconv.Atoi(parts[1]); e == nil {
				deleted += n
			}
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
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		var a, d int
		if parts[0] != "-" {
			a, _ = strconv.Atoi(parts[0])
		}
		if parts[1] != "-" {
			d, _ = strconv.Atoi(parts[1])
		}
		filePath := strings.Join(parts[2:], " ")
		if idx := strings.LastIndex(filePath, " => "); idx != -1 {
			filePath = filePath[idx+4:]
		}
		result[filePath] = [2]int{a, d}
	}
	return result, nil
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
		var out []byte
		if to == "--cached" {
			out, _ = c.gitCmd("diff", "--cached", "--", file).Output()
		} else {
			out, err = c.gitCmd("diff", diffArg(from, to), "--", file).Output()
			if (err != nil || len(out) == 0) && to == "HEAD" {
				if _, statErr := os.Stat(file); statErr == nil {
					out, _ = c.gitCmd("diff", "--no-index", "/dev/null", file).Output()
				}
			}
		}
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
	for _, line := range strings.Split(diffText, "\n") {
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
