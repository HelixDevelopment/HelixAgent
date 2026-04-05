//go:build !windows
// +build !windows

package services

import "syscall"

// collectDiskUsage gets disk usage for the root filesystem (Unix implementation)
func collectDiskUsage() float64 {
	var stat syscall.Statfs_t
	err := syscall.Statfs("/", &stat)
	if err != nil {
		return 0.0
	}

	// Calculate used space in MB - block size is positive and fits in uint64
	totalBytes := stat.Blocks * uint64(stat.Bsize) // #nosec G115
	freeBytes := stat.Bfree * uint64(stat.Bsize)   // #nosec G115
	usedBytes := totalBytes - freeBytes

	return float64(usedBytes) / (1024 * 1024)
}
