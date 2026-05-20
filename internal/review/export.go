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

	var reviewed, total int
	for _, fs := range s.Files {
		total++
		if fs.Status == StatusApproved || fs.Status == StatusViewed {
			reviewed++
		}
	}

	fmt.Fprintf(&sb, "# Code Review — %s\n", rangeLabel)
	fmt.Fprintf(&sb, "**Date:** %s  \n", time.Now().Format("2006-01-02"))
	fmt.Fprintf(&sb, "**Range:** %s..%s  \n", s.RangeFrom, s.RangeTo)
	if total > 0 {
		fmt.Fprintf(&sb, "**Files reviewed:** %d/%d  \n", reviewed, total)
	}

	if len(s.Comments) == 0 {
		sb.WriteString("\n*No comments.*\n")
		return sb.String()
	}

	byFile := make(map[string][]*Comment)
	var fileOrder []string
	for _, c := range s.Comments {
		if _, ok := byFile[c.File]; !ok {
			fileOrder = append(fileOrder, c.File)
		}
		byFile[c.File] = append(byFile[c.File], c)
	}

	for _, file := range fileOrder {
		fmt.Fprintf(&sb, "\n---\n\n## %s\n", file)
		for _, c := range byFile[file] {
			fmt.Fprintf(&sb, "\n**Line:** `%s`  \n", strings.TrimSpace(c.DiffLineContent))
			fmt.Fprintf(&sb, "> %s\n", strings.ReplaceAll(c.Body, "\n", "\n> "))
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
