package types

// WebhookInfoPayload is sent as JSON (application/json) to optional webhook URLs
// on the client and host. Consumers can use source, status, time, and disk fields.
type WebhookInfoPayload struct {
	Source   string `json:"source"` // "client" or "host"
	Status   string `json:"status"`
	Time     string `json:"time"` // RFC3339 UTC
	Message  string `json:"message,omitempty"`
	Event    string `json:"event,omitempty"` // e.g. startup, backup_complete, backup_failed, restore_complete
	BytesFree  uint64 `json:"bytes_free"`
	BytesTotal uint64 `json:"bytes_total"`
	VolumePath string `json:"volume_path,omitempty"`
	VolumeOK   bool   `json:"volume_ok"`
	// Filled by the client when the host reports storage (GET /v1/storage).
	HostBytesFree  uint64 `json:"host_bytes_free,omitempty"`
	HostBytesTotal uint64 `json:"host_bytes_total,omitempty"`
	HostDataDir    string `json:"host_data_dir,omitempty"`
	HostStorageOK  bool   `json:"host_storage_ok"`
	// Set on host webhooks when the event relates to a connecting client.
	ClientIP string `json:"client_ip,omitempty"`
}
