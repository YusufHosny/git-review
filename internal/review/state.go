package review

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func Load(gitDir, branch string) (*State, error) {
	return loadForKind(gitDir, branch, ReviewKindBranch)
}

func LoadCustom(gitDir, branch string) (*State, error) {
	return loadForKind(gitDir, branch, ReviewKindCustom)
}

func loadForKind(gitDir, branch string, kind ReviewKind) (*State, error) {
	path := statePath(gitDir, branch, kind)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return newEmptyState(branch, kind), nil
		}
		return nil, fmt.Errorf("read review state: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse review state: %w", err)
	}
	if s.Files == nil {
		s.Files = make(map[string]*FileState)
	}
	if s.Comments == nil {
		s.Comments = []*Comment{}
	}
	if s.Kind == "" {
		s.Kind = kind
	}
	return &s, nil
}

func newEmptyState(branch string, kind ReviewKind) *State {
	return &State{
		Version:   1,
		Branch:    branch,
		Kind:      kind,
		Files:     make(map[string]*FileState),
		Comments:  []*Comment{},
		UpdatedAt: time.Now(),
	}
}

func Save(gitDir string, s *State) error {
	dir := filepath.Join(gitDir, "review")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create review directory: %w", err)
	}
	s.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal review state: %w", err)
	}
	kind := s.Kind
	if kind == "" {
		kind = ReviewKindBranch
	}
	if err := os.WriteFile(statePath(gitDir, s.Branch, kind), data, 0644); err != nil {
		return fmt.Errorf("write review state: %w", err)
	}
	return nil
}

func (s *State) HasProgress() bool {
	for _, fs := range s.Files {
		if fs.Status != StatusUnreviewed {
			return true
		}
	}
	return len(s.Comments) > 0
}

func (s *State) SetFileStatus(file string, status FileStatus, headCommit string, blobHash ...string) {
	if _, ok := s.Files[file]; !ok {
		s.Files[file] = &FileState{}
	}
	s.Files[file].Status = status
	if status == StatusApproved {
		s.Files[file].ApprovedAtCommit = headCommit
		if len(blobHash) > 0 {
			s.Files[file].ApprovedBlobHash = blobHash[0]
		}
	}
}

func (s *State) GetFileStatus(file string) FileStatus {
	if fs, ok := s.Files[file]; ok {
		return fs.Status
	}
	return StatusUnreviewed
}

func (s *State) GetApprovedAtCommit(file string) string {
	if fs, ok := s.Files[file]; ok {
		return fs.ApprovedAtCommit
	}
	return ""
}

func (s *State) AddComment(file, lineContent string, lineIndex int, body string) *Comment {
	c := &Comment{
		ID:              newID(),
		File:            file,
		DiffLineContent: lineContent,
		DiffLineIndex:   lineIndex,
		Body:            body,
		CreatedAt:       time.Now(),
	}
	s.Comments = append(s.Comments, c)
	return c
}

func (s *State) DeleteComment(id string) {
	for i, c := range s.Comments {
		if c.ID == id {
			s.Comments = append(s.Comments[:i], s.Comments[i+1:]...)
			return
		}
	}
}

func (s *State) CommentsForFile(file string) []*Comment {
	var out []*Comment
	for _, c := range s.Comments {
		if c.File == file {
			out = append(out, c)
		}
	}
	return out
}

func (s *State) Reset() {
	s.Files = make(map[string]*FileState)
	s.Comments = []*Comment{}
}

func statePath(gitDir, branch string, kind ReviewKind) string {
	slug := branchSlug(branch)
	if kind == ReviewKindCustom {
		return filepath.Join(gitDir, "review", slug+".custom.json")
	}
	return filepath.Join(gitDir, "review", slug+".json")
}

var branchSlugInvalidCharRe = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func branchSlug(branch string) string {
	slug := branchSlugInvalidCharRe.ReplaceAllString(branch, "-")
	return strings.Trim(slug, "-")
}

func newID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
