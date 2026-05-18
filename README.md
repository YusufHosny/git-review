# git-review

A GitHub-style code review TUI for the terminal. Browse diffs, mark files approved, leave inline comments, and track what's changed since your last review — all without leaving the shell.

```
┌──────────────────────────────────────────────────────────────────────┐
│  git-review  ·  feature → main  ·  +234 -89  ·  ✓3 !1 ○8  ·  [dark]│
├──────────────────┬───────────────────────────────────────────────────┤
│ ○ src/           │ src/auth/handler.go             +45 -12  [VIEWED] │
│ ✓   auth/       ├───────────────────────────────────────────────────┤
│ !     handler.go│  42 +func (h *Handler) Login(w http.ResponseWriter │
│ ○     token.go  │  43 +   if err := h.validate(r); err != nil {     │
│ ✓   api/        │  44 +     return err                              │
│ ○     routes.go │  45 +   }                                         │
│                 │     ▶ needs input validation                       │
├──────────────────┴───────────────────────────────────────────────────┤
│ ?help  a approve  c comment  n next  E export  s split  t theme  q  │
└──────────────────────────────────────────────────────────────────────┘
```

## Features

- **PR-style diff view** — compares your branch against its merge base, just like GitHub shows a PR. Also supports arbitrary ref ranges, last commit, and since-a-ref modes.
- **Review tracking** — mark files as approved (`✓`), viewed (`~`), or unreviewed (`○`). State is saved in `.git/review/` and persists between sessions.
- **Change detection** — if you approve a file and then new commits land on it, it re-appears as changed (`!`) and shows only the delta since your approval.
- **Inline comments** — attach notes to any diff line. They render as ghost lines in the diff and export to Markdown.
- **Full-width diff backgrounds** — add/del lines are painted across the full terminal width with syntax highlighting preserved, like delta.
- **Side-by-side view** — toggle a split unified→side-by-side view with `s`.
- **fzf integration** — jump to any diff line across all files with `F`.
- **Built-in themes** — switchable live with a visual picker (`t`).
- **Works on staged/unstaged changes** — the default mode includes your uncommitted work, so you can review before you commit.

## Install

Requires Go 1.24+.

```sh
go install github.com/YusufHosny/git-review/cmd/git-review@latest
```

## Usage

```
git review [flags] [ref1 [ref2]]
```

### Diff modes

| Command | What it shows |
|---|---|
| `git review` | Branch vs merge-base — includes staged + unstaged changes (like a live PR) |
| `git review -l` | Last commit only (`HEAD~1..HEAD`) |
| `git review -s <ref>` | Everything since `<ref>` up to HEAD |
| `git review <ref1> <ref2>` | Between two explicit refs |
| `git review <ref1>..<ref2>` | Same, range notation |

When there are no commits yet, `git review` shows staged changes (index vs empty tree), so you can review before your first commit.

Base branch is auto-detected (`main` → `master` → `develop`). Override with `--base <branch>`.

### Non-TUI commands

```sh
git review --status          # summary: branch, range, approval counts
git review --export out.md   # write all comments to a Markdown file
git review -e                # copy all comments to clipboard only
git review --reset           # clear all review state for the current branch
```

`--export` also copies the output to the clipboard (`wl-copy` / `xclip` / `xsel` / `pbcopy`, whichever is available).

## Keybindings

### Navigation

| Key | Action |
|---|---|
| `j` / `k` | Move down / up (accepts numeric prefix: `5j`) |
| `h` / `l` | Focus file tree / diff pane |
| `Tab` | Toggle focus |
| `gg` / `G` | Top / bottom of diff |
| `ctrl+d` / `ctrl+u` | Half-page down / up |
| `H` / `M` / `L` | Cursor to top / middle / bottom of viewport |
| `zz` / `zt` / `zb` | Center / top / bottom cursor in viewport |
| `]c` / `[c` | Next / prev hunk |
| `e` / `Enter` | Open file in editor at the current line |

### Review

| Key | Action |
|---|---|
| `a` | Approve current file |
| `u` | Mark current file unreviewed |
| `r` | Reset current file status |
| `R` | Reset all review state (confirms first) |
| `n` / `p` | Jump to next / prev unreviewed or changed file |

### Comments

| Key | Action |
|---|---|
| `c` | Add comment on current diff line |
| `d` | Delete comment on current line (confirms first) |
| `E` | Export all comments to `review-comments.md` + clipboard |

### Other

| Key | Action |
|---|---|
| `s` | Toggle side-by-side split view |
| `f` | Toggle flat / tree file list |
| `t` | Open theme picker |
| `F` | Launch fzf over all diff content |
| `/` | Search in current diff |
| `n` / `N` | Next / prev search match |
| `?` | Toggle help drawer |
| `q` / `ctrl+c` | Quit |

## File status icons

| Icon | Meaning |
|---|---|
| `○` | Unreviewed |
| `~` | Viewed (scrolled through but not approved) |
| `✓` | Approved |
| `!` | Approved, but changed since approval |

## Review state

State is stored in `.git/review/<branch>.json` — local to your repo, never committed. It tracks per-file status, the commit SHA at time of approval (for change detection), and all comments.

## Themes

Themes are loaded from `~/.config/git-review/themes/`. On first run, the 9 built-in themes are written there as plain YAML files you can inspect and edit.

To add your own theme, drop any [base16](https://github.com/tinted-theming/base16-schemes)-format YAML into that directory and restart — it'll appear in the picker automatically. The only extra field beyond standard base16 is `chroma_theme`, which names the [chroma](https://github.com/alecthomas/chroma) syntax-highlighting style to pair with your theme.

```yaml
scheme: "my-theme"
chroma_theme: "monokai"
base00: "282828"
base01: "3c3836"
# ... base02 through base0F
```

## Configuration

Config file: `~/.config/git-review/config.yaml`

```yaml
editor: ""           # defaults to $EDITOR / $VISUAL / vi
default_base: ""     # auto-detected if empty (main → master → develop)
ui:
  theme: dark
  line_numbers: absolute   # absolute | relative | hybrid | hidden
  split_width_ratio: 0.5
  show_comments_inline: true
```

Environment variable overrides: `GIT_REVIEW_EDITOR`, `GIT_REVIEW_BASE`.

### Line number modes

| Mode | Behaviour |
|---|---|
| `absolute` | Real file line numbers for all lines (default) |
| `hybrid` | Real line number at cursor, relative distance elsewhere (vim-style) |
| `relative` | Distance from cursor for all lines |
| `hidden` | No line numbers |

## Export format

`E` in the TUI (or `--export file.md` / `-e` for clipboard-only) produces Markdown with all comments grouped by file:

```markdown
# Code Review — feature → main
**Date:** 2026-05-18
**Range:** abc1234..HEAD

---

## internal/auth/handler.go

**Line:** `+func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {`
> needs input validation before processing
```
