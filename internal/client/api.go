package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"

	"backuperr/pkg/types"

	"github.com/schollz/progressbar/v3"
)

// API is a thin HTTP client for the backup host.
type API struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

func (a *API) client() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	// No timeout: large backups/uploads can run for a long time.
	return &http.Client{Timeout: 0}
}

func shortBackupID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func (a *API) req(method, path string, body io.Reader, contentType string) (*http.Response, error) {
	url := a.BaseURL + path
	req, err := http.NewRequestWithContext(context.Background(), method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", a.APIKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return a.client().Do(req)
}

// ListBackups returns backups for the authenticated client (by server-side IP).
func (a *API) ListBackups() ([]types.BackupMeta, error) {
	resp, err := a.req(http.MethodGet, "/v1/backups", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list backups: %s: %s", resp.Status, string(b))
	}
	var list []types.BackupMeta
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}
	return list, nil
}

// GetHostStorage reports free/total space on the host's backup data volume (GET /v1/storage).
func (a *API) GetHostStorage() (types.HostStorageInfo, error) {
	resp, err := a.req(http.MethodGet, "/v1/storage", nil, "")
	if err != nil {
		return types.HostStorageInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return types.HostStorageInfo{}, fmt.Errorf("storage: %s: %s", resp.Status, string(b))
	}
	var info types.HostStorageInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return types.HostStorageInfo{}, err
	}
	return info, nil
}

// UploadBackupStream builds a multipart request body on disk (streams archive into it, no full-RAM buffer),
// then uploads with Content-Length. This avoids io.Pipe races where net/http closes the reader while the
// multipart writer is still flushing, which caused "io: read/write on closed pipe" on large uploads.
func (a *API) UploadBackupStream(backupType, parentID string, manifest []types.ManifestEntry, archivePath string, tempDir string) (types.BackupMeta, error) {
	af, err := os.Open(archivePath)
	if err != nil {
		return types.BackupMeta{}, err
	}
	fi, err := af.Stat()
	if err != nil {
		af.Close()
		return types.BackupMeta{}, err
	}
	archiveSize := fi.Size()

	bodyF, err := os.CreateTemp(tempDir, "backuperr-multipart-*.dat")
	if err != nil {
		af.Close()
		return types.BackupMeta{}, err
	}
	bodyPath := bodyF.Name()
	defer func() {
		_ = bodyF.Close()
		_ = os.Remove(bodyPath)
	}()

	mw := multipart.NewWriter(bodyF)
	contentType := mw.FormDataContentType()

	if err := mw.WriteField("type", backupType); err != nil {
		af.Close()
		return types.BackupMeta{}, err
	}
	if parentID != "" {
		if err := mw.WriteField("parent_id", parentID); err != nil {
			af.Close()
			return types.BackupMeta{}, err
		}
	}
	mb, err := json.Marshal(manifest)
	if err != nil {
		af.Close()
		return types.BackupMeta{}, err
	}
	if err := mw.WriteField("manifest", string(mb)); err != nil {
		af.Close()
		return types.BackupMeta{}, err
	}
	part, err := mw.CreateFormFile("archive", "bundle.tar.gz")
	if err != nil {
		af.Close()
		return types.BackupMeta{}, err
	}
	packBar := progressbar.DefaultBytes(archiveSize, fmt.Sprintf("stage upload (%s)", backupType))
	prPack := progressbar.NewReader(af, packBar)
	_, err = io.Copy(part, &prPack)
	_ = af.Close()
	_ = packBar.Close()
	if err != nil {
		return types.BackupMeta{}, err
	}
	if err := mw.Close(); err != nil {
		return types.BackupMeta{}, err
	}

	size, err := bodyF.Seek(0, io.SeekEnd)
	if err != nil {
		return types.BackupMeta{}, err
	}
	if _, err := bodyF.Seek(0, io.SeekStart); err != nil {
		return types.BackupMeta{}, err
	}

	upBar := progressbar.DefaultBytes(size, fmt.Sprintf("upload backup (%s)", backupType))
	prUp := progressbar.NewReader(bodyF, upBar)

	url := a.BaseURL + "/v1/backups"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, io.NopCloser(&prUp))
	if err != nil {
		_ = upBar.Close()
		return types.BackupMeta{}, err
	}
	req.Header.Set("X-API-Key", a.APIKey)
	req.Header.Set("Content-Type", contentType)

	resp, err := a.client().Do(req)
	_ = upBar.Close()
	if err != nil {
		return types.BackupMeta{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return types.BackupMeta{}, fmt.Errorf("upload: %s: %s", resp.Status, string(b))
	}
	var meta types.BackupMeta
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return types.BackupMeta{}, err
	}
	return meta, nil
}

// GetManifest fetches stored manifest for a backup id.
func (a *API) GetManifest(id string) ([]types.ManifestEntry, error) {
	resp, err := a.req(http.MethodGet, "/v1/backups/"+id+"/manifest", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("manifest: %s: %s", resp.Status, string(b))
	}
	var m []types.ManifestEntry
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// GetArchiveResponse opens a download stream. The caller must close resp.Body.
func (a *API) GetArchiveResponse(id string) (*http.Response, error) {
	resp, err := a.req(http.MethodGet, "/v1/backups/"+id+"/download", nil, "")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("download: %s: %s", resp.Status, string(b))
	}
	return resp, nil
}
