package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yusuf/git-review/internal/config"
	"github.com/yusuf/git-review/internal/git"
	"github.com/yusuf/git-review/internal/review"
	"github.com/yusuf/git-review/internal/ui"
)

var version = "0.1.0"

func main() {
	showVersion := flag.Bool("version", false, "Show version")
	flag.BoolVar(showVersion, "v", false, "Show version (shorthand)")

	since := flag.String("since", "", "Diff from <ref> to HEAD")
	flag.StringVar(since, "s", "", "Diff from <ref> to HEAD (shorthand)")

	last := flag.Bool("last", false, "Diff HEAD~1..HEAD")
	flag.BoolVar(last, "l", false, "Diff HEAD~1..HEAD (shorthand)")

	base := flag.String("base", "", "Override auto-detected base branch")

	doReset := flag.Bool("reset", false, "Reset all review state for current branch (non-TUI)")
	doExport := flag.String("export", "", "Export comments to markdown file (non-TUI)")
	doStatus := flag.Bool("status", false, "Print review status summary (non-TUI)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: git review [flags] [ref1 [ref2]]\n\n")
		fmt.Fprintf(os.Stderr, "Diff modes (default: branch vs merge-base):\n")
		fmt.Fprintf(os.Stderr, "  git review                    Branch vs auto-detected base (like a PR)\n")
		fmt.Fprintf(os.Stderr, "  git review -s <ref>           Changes since <ref>\n")
		fmt.Fprintf(os.Stderr, "  git review -l                 Last commit (HEAD~1..HEAD)\n")
		fmt.Fprintf(os.Stderr, "  git review <ref1> <ref2>      Between two refs\n")
		fmt.Fprintf(os.Stderr, "  git review <ref1>..<ref2>     Between two refs (range notation)\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("git-review version %s\n", version)
		os.Exit(0)
	}

	cfg := config.Load()

	gitClient := &git.Client{}

	// Determine diff range
	from, to, rangeLabel, err := computeRange(gitClient, cfg, *since, *last, *base, flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	gitDir := gitClient.GetGitDir()
	if gitDir == "" {
		fmt.Fprintf(os.Stderr, "Error: not in a git repository\n")
		os.Exit(1)
	}

	currentBranch := gitClient.GetCurrentBranch()
	headCommit := gitClient.HeadCommit()

	// Load review state
	reviewState, err := review.Load(gitDir, currentBranch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load review state: %v\n", err)
		reviewState, _ = review.Load("", currentBranch) // create blank state
	}

	// Update range info in state if it has changed
	reviewState.Branch = currentBranch
	reviewState.RangeFrom = from
	reviewState.RangeTo = to

	// === Non-TUI commands ===

	if *doReset {
		reviewState.Reset()
		if err := review.Save(gitDir, reviewState); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Review state reset for branch %q.\n", currentBranch)
		os.Exit(0)
	}

	if *doStatus {
		printStatus(reviewState, rangeLabel)
		os.Exit(0)
	}

	if *doExport != "" {
		outputPath := *doExport
		if outputPath == "" {
			outputPath = "review-comments.md"
		}
		content := review.ExportMarkdown(reviewState, rangeLabel)
		if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing export: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Exported to %s\n", outputPath)
		if err := review.CopyToClipboard(content); err == nil {
			fmt.Println("(also copied to clipboard)")
		}
		os.Exit(0)
	}

	// === TUI ===

	themeIndex := themeIndexForName(cfg.UI.Theme)

	p := tea.NewProgram(
		ui.NewModel(cfg, gitClient, from, to, rangeLabel, reviewState, gitDir, headCommit, themeIndex),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// emptyTreeSHA is the SHA of git's empty tree object — used to diff the very first commit.
const emptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

func computeRange(
	gitClient *git.Client,
	cfg config.Config,
	since string,
	last bool,
	baseOverride string,
	args []string,
) (from, to, label string, err error) {
	// to = "HEAD" (string) is a sentinel meaning "compare against working tree".
	// For explicit commit-to-commit modes we resolve HEAD to its SHA so diffArg
	// does a two-commit comparison and does not include uncommitted changes.
	to = "HEAD"

	switch {
	case last:
		headSHA, resolveErr := gitClient.ResolveRef("HEAD")
		if resolveErr != nil {
			return "", "", "", fmt.Errorf("no commits yet — nothing to compare with HEAD~1")
		}
		to = headSHA // commit-to-commit, not working tree
		if _, resolveErr = gitClient.ResolveRef("HEAD~1"); resolveErr != nil {
			// Single-commit repo: diff the root commit against the empty tree.
			from = emptyTreeSHA
			label = "∅..HEAD"
		} else {
			from = "HEAD~1"
			label = "HEAD~1..HEAD"
		}

	case since != "":
		resolved, resolveErr := gitClient.ResolveRef(since)
		if resolveErr != nil {
			return "", "", "", fmt.Errorf("cannot resolve ref %q: %w", since, resolveErr)
		}
		headSHA, resolveErr := gitClient.ResolveRef("HEAD")
		if resolveErr != nil {
			return "", "", "", fmt.Errorf("cannot resolve HEAD: %w", resolveErr)
		}
		from = resolved
		to = headSHA // commit-to-commit
		label = since + "..HEAD"

	case len(args) == 1 && strings.Contains(args[0], ".."):
		// "ref1..ref2" notation
		parts := strings.SplitN(args[0], "..", 2)
		from = parts[0]
		to = parts[1]
		// If the user wrote "..HEAD" resolve it to a SHA for commit-to-commit comparison.
		if to == "HEAD" {
			if sha, e := gitClient.ResolveRef("HEAD"); e == nil {
				to = sha
			}
		}
		label = args[0]

	case len(args) == 2:
		from = args[0]
		to = args[1]
		if to == "HEAD" {
			if sha, e := gitClient.ResolveRef("HEAD"); e == nil {
				to = sha
			}
		}
		label = from + ".." + to

	default:
		// Default: branch vs merge-base.
		// Keep to = "HEAD" so diffArg compares against the working tree
		// (includes staged + unstaged changes, like a live PR view).
		if _, headErr := gitClient.ResolveRef("HEAD"); headErr != nil {
			// No commits yet: show staged changes (index vs empty tree).
			from = emptyTreeSHA
			to = "--cached"
			label = "staged changes"
			break
		}
		baseBranch := gitClient.AutoDetectBase(baseOverride)
		if baseBranch == "" {
			baseBranch = cfg.DefaultBase
		}
		mergeBase, mbErr := gitClient.MergeBase("HEAD", baseBranch)
		if mbErr != nil {
			// Fallback: diff against base directly
			mergeBase = baseBranch
		}
		from = mergeBase
		currentBranch := gitClient.GetCurrentBranch()
		label = fmt.Sprintf("%s → %s", currentBranch, baseBranch)
	}

	return from, to, label, nil
}

func printStatus(s *review.State, rangeLabel string) {
	fmt.Printf("Branch: %s\n", s.Branch)
	fmt.Printf("Range:  %s\n", rangeLabel)
	fmt.Printf("Comments: %d\n\n", len(s.Comments))

	var approved, changed, viewed, total int
	for file, fs := range s.Files {
		total++
		switch fs.Status {
		case review.StatusApproved:
			approved++
			fmt.Printf("  ✓  %s\n", file)
		case review.StatusViewed:
			viewed++
			fmt.Printf("  ~  %s\n", file)
		case review.StatusChanged:
			changed++
			fmt.Printf("  !  %s\n", file)
		}
	}
	fmt.Printf("\nTotal tracked: %d  Approved: %d  Viewed: %d\n", total, approved, viewed)
}

func themeIndexForName(name string) int {
	for i, t := range ui.Themes {
		if t.Name == name {
			return i
		}
	}
	return 0 // default to dark/nord
}
