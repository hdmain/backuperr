//go:build linux

package client

import "syscall"

// nodeVolumeSpace returns total and free bytes on the filesystem containing path.
func nodeVolumeSpace(path string) (total, free uint64, ok bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, false
	}
	bs := uint64(st.Bsize)
	return uint64(st.Blocks) * bs, uint64(st.Bavail) * bs, true
}
