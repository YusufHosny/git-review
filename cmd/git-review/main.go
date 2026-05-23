package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/YusufHosny/git-review/internal/config"
	"github.com/YusufHosny/git-review/internal/git"
	"github.com/YusufHosny/git-review/internal/review"
	"github.com/YusufHosny/git-review/internal/themes"
	"github.com/YusufHosny/git-review/internal/ui"
)

const version = "0.1.0"

func main() {
	showVersion := flag.Bool("version", false, "show version")
	flag.BoolVar(showVersion, "v", false, "show version")

	fromRef := flag.String("from", "", "diff from <ref> to HEAD")
	flag.StringVar(fromRef, "f", "", "diff from <ref> to HEAD")

	dirty := flag.Bool("dirty", false, "dirty working tree vs HEAD")
	flag.BoolVar(dirty, "d", false, "dirty working tree vs HEAD")

	staged := flag.Bool("staged", false, "staged changes only vs HEAD")
	flag.BoolVar(staged, "S", false, "staged changes only vs HEAD")

	base := flag.String("base", "", "override auto-detected base branch")
	flag.StringVar(base, "b", "", "override auto-detected base branch")

	doReset := flag.Bool("reset", false, "reset review state for this branch")
	flag.BoolVar(doReset, "r", false, "reset review state for this branch")

	doExport := flag.String("export", "", "export comments to markdown file")
	flag.StringVar(doExport, "e", "", "export comments to markdown file")

	doStatus := flag.Bool("status", false, "print review status summary")

	flag.Usage = printHelp

	// Check for help before flag.Parse so printHelp can call flag.PrintDefaults().
	// git intercepts --help for subcommands (man page lookup), so -help and
	// 'git review help' are the workarounds.
	for _, arg := range os.Args[1:] {
		if arg == "--help" || arg == "-help" || arg == "help" {
			printHelp()
			os.Exit(0)
		}
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("git-review version %s\n", version)
		os.Exit(0)
	}

	cfg := config.Load()
	ui.Themes = themes.LoadAll()
	gitClient := &git.Client{}

	from, to, rangeLabel, baseBranchName, isDirty, err := computeRange(gitClient, cfg, *fromRef, *dirty, *staged, *base, flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Default mode with no committed changes: fall back to dirty (working tree) view.
	if !isDirty && *fromRef == "" && len(flag.Args()) == 0 {
		if files, _ := gitClient.ListChangedFiles(from, to); len(files) == 0 {
			from = "HEAD"
			to = "HEAD"
			rangeLabel = "dirty changes"
			baseBranchName = ""
			isDirty = true
		}
	}

	gitDir := gitClient.GetGitDir()
	if gitDir == "" {
		fmt.Fprintf(os.Stderr, "Error: not in a git repository\n")
		os.Exit(1)
	}

	currentBranch := gitClient.GetCurrentBranch()
	headCommit := gitClient.HeadCommit()

	reviewState, err := review.Load(gitDir, currentBranch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load review state: %v\n", err)
		reviewState, _ = review.Load("", currentBranch)
	}

	reviewState.Branch = currentBranch
	reviewState.RangeFrom = from
	reviewState.RangeTo = to

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
		content := review.ExportMarkdown(reviewState, rangeLabel)
		if err := os.WriteFile(*doExport, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing export: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Exported to %s\n", *doExport)
		if err := review.CopyToClipboard(content); err == nil {
			fmt.Println("(also copied to clipboard)")
		}
		os.Exit(0)
	}

	p := tea.NewProgram(
		ui.NewModel(cfg, gitClient, from, to, rangeLabel, baseBranchName, reviewState, gitDir, headCommit, themeIndexForName(cfg.UI.Theme), isDirty),
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
	fromFlag string,
	dirty bool,
	stagedOnly bool,
	baseOverride string,
	args []string,
) (from, to, label, baseBranchName string, isDirty bool, err error) {
	switch {
	case dirty:
		from = "HEAD"
		to = "HEAD"
		label = "dirty changes"
		isDirty = true

	case stagedOnly:
		from = "HEAD"
		to = "--cached"
		label = "staged changes"
		isDirty = true

	case fromFlag != "":
		resolved, resolveErr := gitClient.ResolveRef(fromFlag)
		if resolveErr != nil {
			return "", "", "", "", false, fmt.Errorf("cannot resolve ref %q: %w", fromFlag, resolveErr)
		}
		headSHA, resolveErr := gitClient.ResolveRef("HEAD")
		if resolveErr != nil {
			return "", "", "", "", false, fmt.Errorf("cannot resolve HEAD: %w", resolveErr)
		}
		from = resolved
		to = headSHA
		label = fromFlag + "..HEAD"
		baseBranchName = fromFlag

	case len(args) == 1 && strings.Contains(args[0], ".."):
		parts := strings.SplitN(args[0], "..", 2)
		from = parts[0]
		to = parts[1]
		baseBranchName = parts[0] // preserve user input as readable label
		if sha, e := gitClient.ResolveRef(to); e == nil {
			to = sha
		}
		label = args[0]

	case len(args) == 2:
		from = args[0]
		to = args[1]
		baseBranchName = args[0]
		label = from + ".." + to
		if sha, e := gitClient.ResolveRef(to); e == nil {
			to = sha
		}

	default:
		headSHA, headErr := gitClient.ResolveRef("HEAD")
		if headErr != nil {
			from = emptyTreeSHA
			to = "--cached"
			label = "staged changes"
			isDirty = true
			break
		}
		baseBranch := gitClient.AutoDetectBase(baseOverride)
		if baseBranch == "" {
			baseBranch = cfg.DefaultBase
		}
		mergeBase, mbErr := gitClient.MergeBase("HEAD", baseBranch)
		if mbErr != nil {
			mergeBase = baseBranch
		}
		from = mergeBase
		to = headSHA
		baseBranchName = baseBranch
		label = fmt.Sprintf("%s → %s", gitClient.GetCurrentBranch(), baseBranch)
	}

	return from, to, label, baseBranchName, isDirty, nil
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

func printHelp() {
	fmt.Fprintf(os.Stderr, "git-review %s\n\n", version)
	fmt.Fprintf(os.Stderr, "Usage: git review [options] [ref1 [ref2]]\n\n")
	fmt.Fprintf(os.Stderr, "Modes\n")
	fmt.Fprintf(os.Stderr, "  git review                   branch vs auto-detected base (PR view)\n")
	fmt.Fprintf(os.Stderr, "  git review -d                dirty working tree (staged + unstaged) vs HEAD\n")
	fmt.Fprintf(os.Stderr, "  git review -S                staged changes only vs HEAD\n")
	fmt.Fprintf(os.Stderr, "  git review -f <ref>          changes from <ref> to HEAD\n")
	fmt.Fprintf(os.Stderr, "  git review <ref1> <ref2>     between two refs  (also: ref1..ref2)\n\n")
	fmt.Fprintf(os.Stderr, "Diff options\n")
	fmt.Fprintf(os.Stderr, "  -d, --dirty            dirty working tree vs HEAD\n")
	fmt.Fprintf(os.Stderr, "  -S, --staged           staged changes only vs HEAD\n")
	fmt.Fprintf(os.Stderr, "  -f, --from <ref>       diff from <ref> to HEAD\n")
	fmt.Fprintf(os.Stderr, "  -b, --base <branch>    override auto-detected base branch\n\n")
	fmt.Fprintf(os.Stderr, "Utilities  (non-TUI)\n")
	fmt.Fprintf(os.Stderr, "  -e, --export <file>    export comments to markdown file\n")
	fmt.Fprintf(os.Stderr, "      --status           print review status summary\n")
	fmt.Fprintf(os.Stderr, "  -r, --reset            reset review state for this branch\n")
	fmt.Fprintf(os.Stderr, "  -v, --version          show version\n\n")
	fmt.Fprintf(os.Stderr, "Tip: git intercepts --help — use  git review -h  or  git review help\n")
}

func themeIndexForName(name string) int {
	for i, t := range ui.Themes {
		if t.Name == name {
			return i
		}
	}
	return 0
}
