package client

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"backuperr/pkg/types"

	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/errgroup"
)

func excluded(rel string, rules []string) bool {
	rel = filepath.ToSlash(rel)
	for _, r := range rules {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if strings.Contains(rel, r) {
			return true
		}
	}
	return false
}

func scanWorkerCount() int {
	n := runtime.NumCPU()
	if n < 1 {
		return 1
	}
	return n
}

// collectRelPaths lists relative paths of regular files under backupRoot (excluded rules applied).
func collectRelPaths(backupRoot string, exclude []string) ([]string, error) {
	var rels []string
	err := filepath.WalkDir(backupRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(backupRoot, path)
		if err != nil {
			return err
		}
		if excluded(rel, exclude) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rels = append(rels, filepath.ToSlash(rel))
		return nil
	})
	return rels, err
}

func hashRelEntry(backupRoot, rel string) (types.ManifestEntry, error) {
	full := filepath.Join(backupRoot, filepath.FromSlash(rel))
	info, err := os.Stat(full)
	if err != nil {
		return types.ManifestEntry{}, err
	}
	h, err := hashFile(full)
	if err != nil {
		return types.ManifestEntry{}, err
	}
	return types.ManifestEntry{
		Path:    rel,
		Hash:    h,
		Size:    info.Size(),
		ModTime: info.ModTime().UTC(),
	}, nil
}

// ScanTree walks backupRoot and returns manifest entries for regular files.
// File hashing runs in parallel across all CPUs (see runtime.NumCPU).
func ScanTree(backupRoot string, exclude []string) ([]types.ManifestEntry, error) {
	backupRoot = filepath.Clean(backupRoot)
	rels, err := collectRelPaths(backupRoot, exclude)
	if err != nil {
		return nil, err
	}
	if len(rels) == 0 {
		return nil, nil
	}
	sort.Strings(rels)

	workers := scanWorkerCount()
	out := make([]types.ManifestEntry, len(rels))
	jobs := make(chan int, workers*4)
	go func() {
		for i := range rels {
			jobs <- i
		}
		close(jobs)
	}()

	g, _ := errgroup.WithContext(context.Background())
	for w := 0; w < workers; w++ {
		g.Go(func() error {
			for i := range jobs {
				e, err := hashRelEntry(backupRoot, rels[i])
				if err != nil {
					return err
				}
				out[i] = e
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// PrintBackupDatasetSummary prints total size of files in the manifest and node volume space (Linux).
func PrintBackupDatasetSummary(backupRoot string, entries []types.ManifestEntry) {
	var sum int64
	for _, e := range entries {
		sum += e.Size
	}
	giB := float64(sum) / (1 << 30)
	fmt.Printf("backup dataset: %.2f GiB (%d files)\n", giB, len(entries))
	if total, free, ok := nodeVolumeSpace(backupRoot); ok {
		fmt.Printf("node storage (volume of %s): %.2f GiB free / %.2f GiB total\n",
			backupRoot, float64(free)/(1<<30), float64(total)/(1<<30))
	}
}

// BuildTarGzToTempFile writes a gzip tar of relPaths to a temp file and returns its path.
// The caller must remove the file when done.
func BuildTarGzToTempFile(tempDir string, backupRoot string, relPaths []string) (name string, err error) {
	f, err := os.CreateTemp(tempDir, "backuperr-bundle-*.tar.gz")
	if err != nil {
		return "", err
	}
	path := f.Name()
	defer func() {
		if err != nil {
			f.Close()
			_ = os.Remove(path)
		}
	}()
	if err = writeTarGzToWriter(f, backupRoot, relPaths); err != nil {
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	return path, nil
}

func writeTarGzToWriter(out io.Writer, backupRoot string, relPaths []string) error {
	backupRoot = filepath.Clean(backupRoot)
	seen := make(map[string]struct{})
	var uniq []string
	for _, rel := range relPaths {
		rel = filepath.ToSlash(rel)
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		uniq = append(uniq, rel)
	}

	var totalBytes int64
	for _, rel := range uniq {
		full := filepath.Join(backupRoot, filepath.FromSlash(rel))
		info, err := os.Stat(full)
		if err != nil {
			return fmt.Errorf("stat %s: %w", rel, err)
		}
		totalBytes += info.Size()
	}

	var bar *progressbar.ProgressBar
	if totalBytes > 0 {
		bar = progressbar.DefaultBytes(totalBytes, "pack backup")
	} else {
		bar = progressbar.Default(-1, "pack backup")
	}
	defer bar.Close()

	gw := gzip.NewWriter(out)
	tw := tar.NewWriter(gw)

	for _, rel := range uniq {
		full := filepath.Join(backupRoot, filepath.FromSlash(rel))
		info, err := os.Stat(full)
		if err != nil {
			_ = tw.Close()
			_ = gw.Close()
			return fmt.Errorf("stat %s: %w", rel, err)
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			_ = tw.Close()
			_ = gw.Close()
			return err
		}
		hdr.Name = rel
		hdr.ModTime = info.ModTime()
		if err := tw.WriteHeader(hdr); err != nil {
			_ = tw.Close()
			_ = gw.Close()
			return err
		}
		sf, err := os.Open(full)
		if err != nil {
			_ = tw.Close()
			_ = gw.Close()
			return err
		}
		pr := progressbar.NewReader(sf, bar)
		_, err = io.Copy(tw, &pr)
		sf.Close()
		if err != nil {
			_ = tw.Close()
			_ = gw.Close()
			return err
		}
	}
	if err := tw.Close(); err != nil {
		_ = gw.Close()
		return err
	}
	return gw.Close()
}

func manifestMap(entries []types.ManifestEntry) map[string]types.ManifestEntry {
	m := make(map[string]types.ManifestEntry, len(entries))
	for _, e := range entries {
		m[e.Path] = e
	}
	return m
}

// PlanIncremental compares previous remote manifest with current local scan.
// Returns merged manifest (full tree state) and relative paths to include in the archive.
func PlanIncremental(prev, curr []types.ManifestEntry) (merged []types.ManifestEntry, toPack []string) {
	pm := manifestMap(prev)
	currByPath := manifestMap(curr)
	merged = curr

	for path, c := range currByPath {
		p, ok := pm[path]
		if !ok || p.Hash != c.Hash || p.Size != c.Size {
			toPack = append(toPack, path)
		}
	}
	return merged, toPack
}

// RunBackup performs full or incremental upload using cfg and api.
func RunBackup(cfg *Config, api *API, forceFull bool) (types.BackupMeta, error) {
	if cfg.BackupRoot == "" {
		return types.BackupMeta{}, fmt.Errorf("backup_root is required")
	}
	if cfg.StatePath == "" {
		return types.BackupMeta{}, fmt.Errorf("state_path is required (or set backup_root)")
	}

	curr, err := ScanTree(cfg.BackupRoot, cfg.Exclude)
	if err != nil {
		return types.BackupMeta{}, err
	}
	PrintBackupDatasetSummary(cfg.BackupRoot, curr)

	st, err := loadState(cfg.StatePath)
	if err != nil {
		return types.BackupMeta{}, err
	}

	var backupType string
	var parentID string
	var rels []string

	if forceFull || st.LastBackupID == "" {
		backupType = "full"
		rels = pathsFromManifest(curr)
	} else {
		prev, err := api.GetManifest(st.LastBackupID)
		if err != nil {
			return types.BackupMeta{}, fmt.Errorf("fetch previous manifest (try full backup): %w", err)
		}
		merged, pack := PlanIncremental(prev, curr)
		if len(pack) == 0 && !hasDeletions(prev, curr) {
			return types.BackupMeta{}, fmt.Errorf("nothing to upload (no changes since last backup)")
		}
		backupType = "incremental"
		parentID = st.LastBackupID
		curr = merged
		rels = pack
	}

	tf, err := BuildTarGzToTempFile(cfg.TempDir, cfg.BackupRoot, rels)
	if err != nil {
		return types.BackupMeta{}, err
	}
	defer os.Remove(tf)

	meta, err := api.UploadBackupStream(backupType, parentID, curr, tf, cfg.TempDir)
	if err != nil {
		return types.BackupMeta{}, err
	}
	if err := saveState(cfg.StatePath, State{LastBackupID: meta.ID}); err != nil {
		return meta, fmt.Errorf("backup saved but state not written: %w", err)
	}
	return meta, nil
}

func hasDeletions(prev, curr []types.ManifestEntry) bool {
	pm := manifestMap(prev)
	cm := manifestMap(curr)
	for p := range pm {
		if _, ok := cm[p]; !ok {
			return true
		}
	}
	return false
}

func pathsFromManifest(m []types.ManifestEntry) []string {
	out := make([]string, len(m))
	for i := range m {
		out[i] = m[i].Path
	}
	return out
}
