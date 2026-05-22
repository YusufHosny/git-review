package review

import "time"

type FileStatus string

const (
	StatusUnreviewed FileStatus = "unreviewed"
	StatusViewed     FileStatus = "viewed"
	StatusApproved   FileStatus = "approved"
	StatusChanged    FileStatus = "changed"
)

type FileState struct {
	Status           FileStatus `json:"status"`
	ApprovedAtCommit string     `json:"approved_at_commit,omitempty"`
	ApprovedBlobHash string     `json:"approved_blob_hash,omitempty"`
}

type Comment struct {
	ID              string    `json:"id"`
	File            string    `json:"file"`
	DiffLineContent string    `json:"diff_line_content"`
	DiffLineIndex   int       `json:"diff_line_index"`
	Body            string    `json:"body"`
	CreatedAt       time.Time `json:"created_at"`
}

type State struct {
	Version         int                   `json:"version"`
	Branch          string                `json:"branch"`
	BaseRef         string                `json:"base_ref,omitempty"`
	MergeBaseCommit string                `json:"merge_base_commit,omitempty"`
	RangeFrom       string                `json:"range_from"`
	RangeTo         string                `json:"range_to"`
	UpdatedAt       time.Time             `json:"updated_at"`
	Files           map[string]*FileState `json:"files"`
	Comments        []*Comment            `json:"comments"`
}
