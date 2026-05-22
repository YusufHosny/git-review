package review

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func Load(gitDir, branch string) (*State, error) {
	path := statePath(gitDir, branch)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{
				Version:   1,
				Branch:    branch,
				Files:     make(map[string]*FileState),
				Comments:  []*Comment{},
				UpdatedAt: time.Now(),
			}, nil
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Files == nil {
		s.Files = make(map[string]*FileState)
	}
	if s.Comments == nil {
		s.Comments = []*Comment{}
	}
	return &s, nil
}

func Save(gitDir string, s *State) error {
	dir := filepath.Join(gitDir, "review")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	s.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(gitDir, s.Branch), data, 0644)
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

func statePath(gitDir, branch string) string {
	slug := branchSlug(branch)
	return filepath.Join(gitDir, "review", slug+".json")
}

var slugRe = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func branchSlug(branch string) string {
	slug := slugRe.ReplaceAllString(branch, "-")
	return strings.Trim(slug, "-")
}

func newID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
