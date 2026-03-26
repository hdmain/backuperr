package host

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"strings"
	"time"

	"backuperr/internal/webhook"
	"backuperr/pkg/types"
)

// Server handles backup HTTP API.
type Server struct {
	DataDir    string
	MainKey    string
	Log        *log.Logger
	WebhookURL string
}

func (s *Server) hostDiskPayloadBase() (types.WebhookInfoPayload, bool) {
	p := types.WebhookInfoPayload{
		Source:     "host",
		VolumePath: s.DataDir,
	}
	free, total, err := dataDirDiskUsage(s.DataDir)
	if err != nil {
		return p, false
	}
	p.BytesFree, p.BytesTotal, p.VolumeOK = free, total, true
	return p, true
}

func (s *Server) makeHostWebhookPayload(status, message, event, clientIP string) types.WebhookInfoPayload {
	p, _ := s.hostDiskPayloadBase()
	p.Status = status
	p.Message = message
	p.Event = event
	p.ClientIP = clientIP
	p.Time = time.Now().UTC().Format(time.RFC3339)
	return p
}

func (s *Server) sendWebhookAsync(p types.WebhookInfoPayload) {
	url := strings.TrimSpace(s.WebhookURL)
	if url == "" {
		return
	}
	go func() {
		if err := webhook.PostInfo(url, p); err != nil {
			s.Log.Printf("webhook: %v", err)
		}
	}()
}

// NotifyStartupWebhook posts host status, time, and data-dir free space (if webhook_url is set).
func (s *Server) NotifyStartupWebhook() {
	p := s.makeHostWebhookPayload("ok", "listener starting", "startup", "")
	s.sendWebhookAsync(p)
}

func (s *Server) clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) auth(r *http.Request) bool {
	key := r.Header.Get("X-API-Key")
	if key == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(key), []byte(s.MainKey)) == 1
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.auth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ip := s.clientIP(r)
	list, err := ListBackups(s.DataDir, ip)
	if err != nil {
		s.Log.Printf("list backups: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []types.BackupMeta{}
	}
	s.writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.auth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ip := s.clientIP(r)
	if r.ContentLength > 0 {
		s.Log.Printf("receive backup: client=%s HTTP_Content-Length=%d (reading multipart)", ip, r.ContentLength)
	} else {
		s.Log.Printf("receive backup: client=%s (reading multipart, size unknown)", ip)
	}

	// Stream multipart upload directly from the request body.
	// This avoids writing large request parts into os.TempDir() (typically /tmp),
	// which can fail when /tmp is full.
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		s.Log.Printf("receive backup: invalid Content-Type: %q err=%v (client=%s)", r.Header.Get("Content-Type"), err, ip)
		http.Error(w, "bad multipart content-type", http.StatusBadRequest)
		return
	}
	boundary := params["boundary"]
	if boundary == "" {
		http.Error(w, "missing multipart boundary", http.StatusBadRequest)
		return
	}

	mr := multipart.NewReader(r.Body, boundary)
	var (
		backupType string
		parentID   string
		manifest   []types.ManifestEntry
		meta       types.BackupMeta
		metaSet    bool
	)

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.Log.Printf("receive backup: multipart NextPart failed: %v (client=%s)", err, ip)
			http.Error(w, "bad multipart form", http.StatusBadRequest)
			return
		}

		switch part.FormName() {
		case "type":
			b, _ := io.ReadAll(part)
			backupType = strings.ToLower(strings.TrimSpace(string(b)))
		case "parent_id":
			b, _ := io.ReadAll(part)
			parentID = strings.TrimSpace(string(b))
		case "manifest":
			b, _ := io.ReadAll(part)
			if err := json.Unmarshal(b, &manifest); err != nil {
				http.Error(w, "invalid manifest json", http.StatusBadRequest)
				return
			}
		case "archive":
			if backupType == "" {
				http.Error(w, "missing type field", http.StatusBadRequest)
				return
			}
			if manifest == nil {
				http.Error(w, "missing manifest field", http.StatusBadRequest)
				return
			}
			s.Log.Printf("receive backup: client=%s type=%s parent=%q manifest_files=%d (writing bundle)", ip, backupType, parentID, len(manifest))
			bundleReader := io.TeeReader(part, newReceiveProgressLog(s.Log, ip, backupType, parentID))
			meta, err = SaveBackup(s.DataDir, ip, backupType, parentID, manifest, bundleReader)
			if err != nil {
				s.Log.Printf("save backup: %v", err)
				p := s.makeHostWebhookPayload("error", err.Error(), "backup_failed", ip)
				s.sendWebhookAsync(p)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			metaSet = true
			s.Log.Printf("receive backup done: client=%s new_id=%s type=%s bundle_stored_bytes=%d files=%d", ip, meta.ID, meta.Type, meta.Bytes, meta.FileCount)
			wh := s.makeHostWebhookPayload("ok", fmt.Sprintf("id=%s type=%s files=%d bytes=%d", meta.ID, meta.Type, meta.FileCount, meta.Bytes), "backup_received", ip)
			s.sendWebhookAsync(wh)
			s.writeJSON(w, http.StatusCreated, meta)
			return
		default:
			// Drain unknown parts.
			_, _ = io.Copy(io.Discard, part)
		}
	}

	if !metaSet {
		http.Error(w, "missing archive part", http.StatusBadRequest)
		return
	}
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.auth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/backups/")
	id = strings.TrimSuffix(id, "/download")
	if id == "" || strings.Contains(id, "/") {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	ip := s.clientIP(r)
	f, err := OpenArchive(s.DataDir, ip, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		s.Log.Printf("open archive: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		s.Log.Printf("stat archive: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.Log.Printf("send backup: client=%s backup_id=%s size_bytes=%d", ip, id, fi.Size())

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", "attachment; filename="+id+".tar.gz")
	http.ServeContent(w, r, id+".tar.gz", fi.ModTime(), f)
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.auth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/backups/")
	id = strings.TrimSuffix(id, "/meta")
	if id == "" || strings.Contains(id, "/") {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	ip := s.clientIP(r)
	meta, err := GetMeta(s.DataDir, ip, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.writeJSON(w, http.StatusOK, meta)
}

func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.auth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/backups/")
	id = strings.TrimSuffix(id, "/manifest")
	if id == "" || strings.Contains(id, "/") {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	ip := s.clientIP(r)
	m, err := ReadManifest(s.DataDir, ip, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.Log.Printf("send manifest: client=%s backup_id=%s entries=%d", ip, id, len(m))
	s.writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleStorage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.auth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	info := types.HostStorageInfo{DataDir: s.DataDir}
	free, total, err := dataDirDiskUsage(s.DataDir)
	if err != nil {
		s.writeJSON(w, http.StatusOK, info)
		return
	}
	info.BytesFree = free
	info.BytesTotal = total
	info.Supported = true
	s.writeJSON(w, http.StatusOK, info)
}

// Handler returns the root mux for the API.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/storage", s.handleStorage)
	mux.HandleFunc("/v1/backups", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/backups":
			if r.Method == http.MethodGet {
				s.handleList(w, r)
				return
			}
			if r.Method == http.MethodPost {
				s.handleCreateBackup(w, r)
				return
			}
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/v1/backups/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/download"):
			s.handleDownload(w, r)
		case strings.HasSuffix(path, "/meta"):
			s.handleMeta(w, r)
		case strings.HasSuffix(path, "/manifest"):
			s.handleManifest(w, r)
		default:
			http.NotFound(w, r)
		}
	})
	return mux
}
