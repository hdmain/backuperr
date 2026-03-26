package types

// HostStorageInfo is returned by GET /v1/storage (disk backing the host data directory).
type HostStorageInfo struct {
	BytesFree  uint64 `json:"bytes_free"`
	BytesTotal uint64 `json:"bytes_total"`
	DataDir    string `json:"data_dir"`
	Supported  bool   `json:"supported"` // false when the host OS cannot report usage (e.g. non-Linux)
}
