//go:build !linux

package client

func nodeVolumeSpace(path string) (total, free uint64, ok bool) {
	_ = path
	return 0, 0, false
}
