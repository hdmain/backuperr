package client

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"backuperr/pkg/types"
)

// PrunePathsNotInManifest removes regular files under root whose relative path is not in manifest.
// Used after restore so incremental deletions match the tip backup state.
func PrunePathsNotInManifest(root string, manifest []types.ManifestEntry) error {
	root = filepath.Clean(root)
	allowed := make(map[string]struct{}, len(manifest))
	for _, e := range manifest {
		allowed[e.Path] = struct{}{}
	}

	var toDel []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if _, ok := allowed[rel]; ok {
			return nil
		}
		toDel = append(toDel, path)
		return nil
	})
	if err != nil {
		return err
	}
	for _, p := range toDel {
		if err := os.Remove(p); err != nil {
			fmt.Fprintf(os.Stderr, "restore: prune could not remove %q: %v\n", p, err)
		}
	}
	return removeEmptyDirs(root)
}

func removeEmptyDirs(root string) error {
	for i := 0; i < 256; i++ {
		var empty []string
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || path == root {
				return nil
			}
			if !d.IsDir() {
				return nil
			}
			ents, err := os.ReadDir(path)
			if err != nil {
				return nil
			}
			if len(ents) == 0 {
				empty = append(empty, path)
			}
			return nil
		})
		if len(empty) == 0 {
			return nil
		}
		for _, d := range empty {
			_ = os.Remove(d)
		}
	}
	return nil
}
