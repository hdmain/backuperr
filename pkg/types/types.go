package types

import "time"

// ManifestEntry describes one file in a backup manifest.
type ManifestEntry struct {
	Path    string    `json:"path"`
	Hash    string    `json:"hash"` // SHA256 hex
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// BackupMeta is stored on the host per backup and returned in list API.
type BackupMeta struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // "full" | "incremental"
	ParentID  string    `json:"parent_id,omitempty"`
	ClientIP  string    `json:"client_ip"`
	CreatedAt time.Time `json:"created_at"`
	FileCount int       `json:"file_count"`
	Bytes     int64     `json:"bytes"`
}
