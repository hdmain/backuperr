package host

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"backuperr/pkg/types"

	"github.com/google/uuid"
)

const (
	metaFilename      = "meta.json"
	manifestFilename  = "manifest.json"
	archiveFilename   = "bundle.tar.gz"
)

// ErrNotFound is returned when a backup does not exist for a client.
var ErrNotFound = errors.New("backup not found")

func clientDir(dataRoot, clientIP string) string {
	safe := strings.ReplaceAll(clientIP, ".", "_")
	safe = strings.ReplaceAll(safe, ":", "_")
	return filepath.Join(dataRoot, safe)
}

func backupDir(dataRoot, clientIP, id string) string {
	return filepath.Join(clientDir(dataRoot, clientIP), id)
}

// SaveBackup writes manifest, archive, and meta under dataRoot/<clientIP>/<id>/.
func SaveBackup(dataRoot, clientIP string, backupType string, parentID string, manifest []types.ManifestEntry, archiveReader io.Reader) (types.BackupMeta, error) {
	if backupType != "full" && backupType != "incremental" {
		return types.BackupMeta{}, fmt.Errorf("invalid backup type %q", backupType)
	}
	if backupType == "incremental" && parentID == "" {
		return types.BackupMeta{}, errors.New("incremental backup requires parent_id")
	}
	if backupType == "full" && parentID != "" {
		return types.BackupMeta{}, errors.New("full backup must not set parent_id")
	}
	if backupType == "incremental" {
		if _, err := GetMeta(dataRoot, clientIP, parentID); err != nil {
			return types.BackupMeta{}, fmt.Errorf("parent backup: %w", err)
		}
	}

	id := uuid.NewString()
	dir := backupDir(dataRoot, clientIP, id)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return types.BackupMeta{}, err
	}

	mf, err := os.OpenFile(filepath.Join(dir, manifestFilename), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return types.BackupMeta{}, err
	}
	if err := json.NewEncoder(mf).Encode(manifest); err != nil {
		mf.Close()
		return types.BackupMeta{}, err
	}
	if err := mf.Close(); err != nil {
		return types.BackupMeta{}, err
	}

	af, err := os.OpenFile(filepath.Join(dir, archiveFilename), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return types.BackupMeta{}, err
	}
	n, err := io.Copy(af, archiveReader)
	if err != nil {
		af.Close()
		return types.BackupMeta{}, err
	}
	if err := af.Close(); err != nil {
		return types.BackupMeta{}, err
	}

	var total int64
	for _, e := range manifest {
		total += e.Size
	}

	meta := types.BackupMeta{
		ID:        id,
		Type:      backupType,
		ParentID:  parentID,
		ClientIP:  clientIP,
		CreatedAt: time.Now().UTC(),
		FileCount: len(manifest),
		Bytes:     n,
	}
	mbf, err := os.OpenFile(filepath.Join(dir, metaFilename), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return types.BackupMeta{}, err
	}
	if err := json.NewEncoder(mbf).Encode(meta); err != nil {
		mbf.Close()
		return types.BackupMeta{}, err
	}
	if err := mbf.Close(); err != nil {
		return types.BackupMeta{}, err
	}

	return meta, nil
}

// ListBackups returns metadata for all backups for clientIP, newest first.
func ListBackups(dataRoot, clientIP string) ([]types.BackupMeta, error) {
	root := clientDir(dataRoot, clientIP)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []types.BackupMeta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		metaPath := filepath.Join(root, e.Name(), metaFilename)
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var m types.BackupMeta
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		out = append(out, m)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// GetMeta returns backup metadata if it belongs to clientIP.
func GetMeta(dataRoot, clientIP, id string) (types.BackupMeta, error) {
	dir := backupDir(dataRoot, clientIP, id)
	data, err := os.ReadFile(filepath.Join(dir, metaFilename))
	if err != nil {
		if os.IsNotExist(err) {
			return types.BackupMeta{}, ErrNotFound
		}
		return types.BackupMeta{}, err
	}
	var m types.BackupMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return types.BackupMeta{}, err
	}
	return m, nil
}

// OpenArchive returns a read closer for the backup bundle.
func OpenArchive(dataRoot, clientIP, id string) (*os.File, error) {
	path := filepath.Join(backupDir(dataRoot, clientIP, id), archiveFilename)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return f, nil
}

// ReadManifest loads manifest.json for a backup.
func ReadManifest(dataRoot, clientIP, id string) ([]types.ManifestEntry, error) {
	path := filepath.Join(backupDir(dataRoot, clientIP, id), manifestFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var m []types.ManifestEntry
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}
