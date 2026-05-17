package review

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func ExportMarkdown(s *State, rangeLabel string) string {
	var sb strings.Builder

	reviewed := 0
	total := 0
	for _, fs := range s.Files {
		total++
		if fs.Status == StatusApproved || fs.Status == StatusViewed {
			reviewed++
		}
	}

	sb.WriteString(fmt.Sprintf("# Code Review — %s\n", rangeLabel))
	sb.WriteString(fmt.Sprintf("**Date:** %s  \n", time.Now().Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("**Range:** %s..%s  \n", s.RangeFrom, s.RangeTo))
	if total > 0 {
		sb.WriteString(fmt.Sprintf("**Files reviewed:** %d/%d  \n", reviewed, total))
	}

	if len(s.Comments) == 0 {
		sb.WriteString("\n*No comments.*\n")
		return sb.String()
	}

	// Group comments by file
	fileOrder := make([]string, 0)
	byFile := make(map[string][]*Comment)
	for _, c := range s.Comments {
		if _, seen := byFile[c.File]; !seen {
			fileOrder = append(fileOrder, c.File)
		}
		byFile[c.File] = append(byFile[c.File], c)
	}

	for _, file := range fileOrder {
		comments := byFile[file]
		sb.WriteString(fmt.Sprintf("\n---\n\n## %s\n", file))
		for _, c := range comments {
			lineContent := strings.TrimSpace(c.DiffLineContent)
			sb.WriteString(fmt.Sprintf("\n**Line:** `%s`  \n", lineContent))
			sb.WriteString(fmt.Sprintf("> %s\n", strings.ReplaceAll(c.Body, "\n", "\n> ")))
		}
	}

	return sb.String()
}

func CopyToClipboard(content string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	default:
		// Try wl-copy (Wayland) first, then xclip (X11)
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-sel", "clip")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else {
			return fmt.Errorf("no clipboard utility found (install wl-copy, xclip, or xsel)")
		}
	}

	cmd.Stdin = strings.NewReader(content)
	return cmd.Run()
}
