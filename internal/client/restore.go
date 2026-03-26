package client

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"backuperr/pkg/types"

	"github.com/schollz/progressbar/v3"
)

// BackupChain returns ids from root full backup through tip, in apply order (full first).
func BackupChain(list []types.BackupMeta, tipID string) ([]string, error) {
	byID := make(map[string]types.BackupMeta, len(list))
	for _, b := range list {
		byID[b.ID] = b
	}
	var chain []string
	id := tipID
	for id != "" {
		b, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("backup %q not in list (wrong client IP or deleted)", id)
		}
		chain = append(chain, id)
		id = b.ParentID
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

func safeJoin(dest, name string) (string, error) {
	name = strings.TrimSuffix(filepath.ToSlash(name), "/")
	if name == "" || name == "." {
		return "", fmt.Errorf("illegal empty path in archive")
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") {
		return "", fmt.Errorf("illegal path in archive: %q", name)
	}
	for _, p := range strings.Split(name, "/") {
		if p == ".." {
			return "", fmt.Errorf("illegal path in archive: %q", name)
		}
	}
	return filepath.Join(dest, filepath.FromSlash(name)), nil
}

func symlinkTargetOK(target string) error {
	target = filepath.ToSlash(strings.TrimSpace(target))
	if target == "" {
		return fmt.Errorf("empty symlink target")
	}
	if filepath.IsAbs(target) || strings.HasPrefix(target, "/") {
		return fmt.Errorf("absolute symlink target not allowed: %q", target)
	}
	if strings.Contains(target, "..") {
		return fmt.Errorf("symlink target must not contain '..': %q", target)
	}
	return nil
}

func headerPerm(h *tar.Header) os.FileMode {
	mode := os.FileMode(h.Mode)
	if mode == 0 {
		return 0o644
	}
	return mode & os.ModePerm
}

func extractTarStream(tr *tar.Reader, destDir string) error {
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := hdr.Name
		switch hdr.Typeflag {
		case tar.TypeDir:
			target, err := safeJoin(destDir, name)
			if err != nil {
				return err
			}
			perm := headerPerm(hdr)
			if perm == 0 {
				perm = 0o755
			}
			if err := os.MkdirAll(target, perm); err != nil {
				return err
			}
			if err := chtimesIfSet(target, hdr.ModTime); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := symlinkTargetOK(hdr.Linkname); err != nil {
				return fmt.Errorf("%s: %w", hdr.Name, err)
			}
			target, err := safeJoin(destDir, name)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			target, err := safeJoin(destDir, name)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			perm := headerPerm(hdr)
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
			if err != nil {
				if isTextFileBusy(err) {
					fmt.Fprintf(os.Stderr, "restore: skipping %q: text file busy (cannot overwrite a running executable; use restore_to outside this tree or stop the client first)\n", target)
					if _, err := io.CopyN(io.Discard, tr, hdr.Size); err != nil {
						return fmt.Errorf("%s: %w", hdr.Name, err)
					}
					continue
				}
				return fmt.Errorf("%s: %w", hdr.Name, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
			if err := chtimesIfSet(target, hdr.ModTime); err != nil {
				return err
			}
		default:
			if _, err := io.CopyN(io.Discard, tr, hdr.Size); err != nil {
				return err
			}
		}
	}
	return nil
}

// ExtractTarGzStream reads a gzip-compressed tar from r without buffering the whole archive in memory.
// compressedSize is HTTP Content-Length when known (bytes on the wire), for the progress bar.
func ExtractTarGzStream(desc string, r io.Reader, compressedSize int64, destDir string) error {
	destDir = filepath.Clean(destDir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	var bar *progressbar.ProgressBar
	if compressedSize > 0 {
		bar = progressbar.DefaultBytes(compressedSize, desc)
	} else {
		bar = progressbar.Default(-1, desc)
	}
	defer bar.Close()
	pr := progressbar.NewReader(r, bar)
	gr, err := gzip.NewReader(&pr)
	if err != nil {
		return err
	}
	defer gr.Close()
	return extractTarStream(tar.NewReader(gr), destDir)
}

func chtimesIfSet(path string, mtime time.Time) error {
	if mtime.IsZero() {
		return nil
	}
	return os.Chtimes(path, mtime, mtime)
}

func isTextFileBusy(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.ETXTBSY) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "text file busy")
}

// RunRestore downloads the backup chain for tipID and writes files into cfg.RestoreTo
// (directories, regular files, safe relative symlinks, modes and mtimes when present).
// After applying bundles, removes files under restore root that are not in the tip manifest
// so incremental deletions are reflected on disk.
func RunRestore(cfg *Config, api *API, tipID string) error {
	if cfg.RestoreTo == "" {
		return fmt.Errorf("set restore_to or backup_root in config (restore target directory)")
	}
	list, err := api.ListBackups()
	if err != nil {
		return err
	}
	chain, err := BackupChain(list, tipID)
	if err != nil {
		return err
	}
	dest := filepath.Clean(cfg.RestoreTo)
	for i, id := range chain {
		fmt.Printf("restore: applying %d/%d (%s) -> %s\n", i+1, len(chain), id, dest)
		resp, err := api.GetArchiveResponse(id)
		if err != nil {
			return fmt.Errorf("download %s: %w", id, err)
		}
		desc := fmt.Sprintf("extract bundle %d/%d", i+1, len(chain))
		err = ExtractTarGzStream(desc, resp.Body, resp.ContentLength, dest)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("extract %s: %w", id, err)
		}
	}

	final, err := api.GetManifest(tipID)
	if err != nil {
		return fmt.Errorf("tip manifest for prune: %w", err)
	}
	if err := PrunePathsNotInManifest(dest, final); err != nil {
		return fmt.Errorf("prune: %w", err)
	}
	fmt.Printf("restore: finished (%d bundle(s)); pruned paths not in tip manifest (%d files kept)\n", len(chain), len(final))
	return nil
}
