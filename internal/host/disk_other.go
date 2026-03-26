//go:build !linux

package host

import "fmt"

func dataDirDiskUsage(path string) (uint64, uint64, error) {
	_ = path
	return 0, 0, fmt.Errorf("disk usage not implemented on this platform")
}
