package restic

import (
	"encoding/json"
	"time"
)

// BackupStatus is a streaming progress line emitted during `restic backup --json`
// with message_type "status".
type BackupStatus struct {
	PercentDone  float64  `json:"percent_done"`
	TotalFiles   int      `json:"total_files"`
	FilesDone    int      `json:"files_done"`
	TotalBytes   uint64   `json:"total_bytes"`
	BytesDone    uint64   `json:"bytes_done"`
	CurrentFiles []string `json:"current_files"`
}

// BackupSummary is the final line of `restic backup --json` (message_type "summary").
type BackupSummary struct {
	MessageType         string  `json:"message_type"`
	FilesNew            int     `json:"files_new"`
	FilesChanged        int     `json:"files_changed"`
	FilesUnmodified     int     `json:"files_unmodified"`
	DirsNew             int     `json:"dirs_new"`
	DataAdded           uint64  `json:"data_added"`
	DataAddedPacked     uint64  `json:"data_added_packed"`
	TotalFilesProcessed int     `json:"total_files_processed"`
	TotalBytesProcessed uint64  `json:"total_bytes_processed"`
	TotalDuration       float64 `json:"total_duration"`
	SnapshotID          string  `json:"snapshot_id"`
}

// Snapshot is one entry from `restic snapshots --json`.
type Snapshot struct {
	ID       string    `json:"id"`
	ShortID  string    `json:"short_id"`
	Time     time.Time `json:"time"`
	Hostname string    `json:"hostname"`
	Username string    `json:"username"`
	Paths    []string  `json:"paths"`
	Tags     []string  `json:"tags"`
}

// Node is one file/dir entry from `restic ls <snap> --json`.
type Node struct {
	Name  string    `json:"name"`
	Type  string    `json:"type"`
	Path  string    `json:"path"`
	Size  uint64    `json:"size"`
	Mtime time.Time `json:"mtime"`
}

// RestoreStatus is a streaming progress line during `restic restore --json`.
type RestoreStatus struct {
	MessageType   string  `json:"message_type"`
	PercentDone   float64 `json:"percent_done"`
	TotalFiles    int     `json:"total_files"`
	FilesRestored int     `json:"files_restored"`
	TotalBytes    uint64  `json:"total_bytes"`
	BytesRestored uint64  `json:"bytes_restored"`
}

// peekType reads only the message_type field so we can route a JSON line without
// committing to a full struct. Unknown types are ignored by callers.
func peekType(line []byte) string {
	var e struct {
		MessageType string `json:"message_type"`
	}
	_ = json.Unmarshal(line, &e)
	return e.MessageType
}
