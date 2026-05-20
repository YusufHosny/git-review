package tree

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/YusufHosny/git-review/internal/review"
)

type FileTree struct {
	Root *Node
}

type Node struct {
	Name     string
	FullPath string
	IsDir    bool
	Children map[string]*Node
	Expanded bool
	Depth    int
}

type TreeItem struct {
	Name     string
	FullPath string
	IsDir    bool
	Depth    int
	Expanded bool
	Icon     string
	Status   review.FileStatus
	FlatMode bool
}

func (i TreeItem) FilterValue() string { return i.FullPath }
func (i TreeItem) Description() string { return "" }
func (i TreeItem) Title() string {
	statusIcon := statusIcon(i.Status, i.IsDir)
	if i.FlatMode {
		return fmt.Sprintf("%s %s %s", statusIcon, i.Icon, i.FullPath)
	}
	indent := strings.Repeat("  ", i.Depth)
	disclosure := " "
	if i.IsDir {
		if i.Expanded {
			disclosure = "▾"
		} else {
			disclosure = "▸"
		}
	}
	return fmt.Sprintf("%s%s %s %s %s", indent, disclosure, statusIcon, i.Icon, i.Name)
}

func statusIcon(s review.FileStatus, isDir bool) string {
	if isDir {
		return " "
	}
	switch s {
	case review.StatusApproved:
		return "✓"
	case review.StatusViewed:
		return "~"
	case review.StatusChanged:
		return "!"
	default:
		return "○"
	}
}

func New(paths []string) *FileTree {
	root := &Node{
		Name:     "root",
		IsDir:    true,
		Children: make(map[string]*Node),
		Expanded: true,
		Depth:    -1,
	}
	for _, path := range paths {
		addPath(root, path)
	}
	return &FileTree{Root: root}
}

func addPath(root *Node, path string) {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(path)), "/")
	current := root
	for i, name := range parts {
		if _, exists := current.Children[name]; !exists {
			isFile := i == len(parts)-1
			nodePath := name
			if current.FullPath != "" {
				nodePath = current.FullPath + "/" + name
			}
			current.Children[name] = &Node{
				Name:     name,
				FullPath: nodePath,
				IsDir:    !isFile,
				Children: make(map[string]*Node),
				Expanded: true,
				Depth:    current.Depth + 1,
			}
		}
		current = current.Children[name]
	}
}

func (t *FileTree) Items(flat bool, statuses map[string]review.FileStatus) []list.Item {
	var items []list.Item
	if flat {
		t.flattenFiles(t.Root, &items, statuses)
	} else {
		t.flatten(t.Root, &items, statuses)
	}
	return items
}

func (t *FileTree) flattenFiles(node *Node, items *[]list.Item, statuses map[string]review.FileStatus) {
	for _, child := range sortedChildren(node) {
		if !child.IsDir {
			*items = append(*items, TreeItem{
				Name:     child.Name,
				FullPath: child.FullPath,
				Icon:     getIcon(child.Name, false),
				Status:   statuses[child.FullPath],
				FlatMode: true,
			})
		}
		t.flattenFiles(child, items, statuses)
	}
}

func (t *FileTree) flatten(node *Node, items *[]list.Item, statuses map[string]review.FileStatus) {
	for _, child := range sortedChildren(node) {
		*items = append(*items, TreeItem{
			Name:     child.Name,
			FullPath: child.FullPath,
			IsDir:    child.IsDir,
			Depth:    child.Depth,
			Expanded: child.Expanded,
			Icon:     getIcon(child.Name, child.IsDir),
			Status:   statuses[child.FullPath],
		})
		if child.IsDir && child.Expanded {
			t.flatten(child, items, statuses)
		}
	}
}

func sortedChildren(node *Node) []*Node {
	children := make([]*Node, 0, len(node.Children))
	for _, child := range node.Children {
		children = append(children, child)
	}
	sort.Slice(children, func(i, j int) bool {
		if children[i].IsDir != children[j].IsDir {
			return children[i].IsDir
		}
		return strings.ToLower(children[i].Name) < strings.ToLower(children[j].Name)
	})
	return children
}

func (t *FileTree) ToggleExpand(fullPath string) {
	if node := findNode(t.Root, fullPath); node != nil && node.IsDir {
		node.Expanded = !node.Expanded
	}
}

func findNode(node *Node, fullPath string) *Node {
	if node.FullPath == fullPath {
		return node
	}
	for _, child := range node.Children {
		if strings.HasPrefix(fullPath, child.FullPath) {
			if found := findNode(child, fullPath); found != nil {
				return found
			}
		}
	}
	return nil
}

func getIcon(name string, isDir bool) string {
	if isDir {
		return ""
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".go":
		return ""
	case ".js", ".ts", ".tsx", ".jsx":
		return ""
	case ".css", ".scss", ".sass":
		return ""
	case ".html", ".htm":
		return ""
	case ".json", ".yaml", ".yml", ".toml":
		return ""
	case ".md", ".mdx":
		return ""
	case ".png", ".jpg", ".jpeg", ".svg", ".gif", ".webp":
		return ""
	case ".gitignore", ".gitmodules", ".gitattributes":
		return ""
	case ".rs":
		return ""
	case ".py":
		return ""
	case ".sh", ".bash", ".zsh":
		return ""
	case ".lock":
		return ""
	default:
		return ""
	}
}
