//go:build windows
// +build windows

package services

import (
	"golang.org/x/sys/windows"
)

// collectDiskUsage gets disk usage for the root filesystem (Windows implementation)
func collectDiskUsage() float64 {
	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64

	err := windows.GetDiskFreeSpaceEx(
		windows.StringToUTF16Ptr("C:\\"),
		&freeBytesAvailable,
		&totalNumberOfBytes,
		&totalNumberOfFreeBytes,
	)
	if err != nil {
		return 0.0
	}

	usedBytes := totalNumberOfBytes - totalNumberOfFreeBytes
	return float64(usedBytes) / (1024 * 1024)
}
