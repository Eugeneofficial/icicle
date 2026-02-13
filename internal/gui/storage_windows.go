//go:build windows

package gui

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows"
)

type DriveUsage struct {
	Drive string `json:"drive"`
	Total int64  `json:"total"`
	Free  int64  `json:"free"`
	Used  int64  `json:"used"`
}

func SystemStorage() ([]DriveUsage, error) {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, err
	}
	out := make([]DriveUsage, 0, 8)
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		drive := fmt.Sprintf("%c:\\", 'A'+i)
		if !isFixedOrRemovableDrive(drive) {
			continue
		}
		var freeBytes, totalBytes, _totalFree uint64
		p, _ := windows.UTF16PtrFromString(drive)
		err := windows.GetDiskFreeSpaceEx(p, &freeBytes, &totalBytes, &_totalFree)
		if err != nil || totalBytes == 0 {
			continue
		}
		used := int64(totalBytes - freeBytes)
		out = append(out, DriveUsage{Drive: strings.TrimSuffix(drive, "\\"), Total: int64(totalBytes), Free: int64(freeBytes), Used: used})
	}
	return out, nil
}

func isFixedOrRemovableDrive(path string) bool {
	ptr, _ := windows.UTF16PtrFromString(path)
	t := windows.GetDriveType(ptr)
	return t == windows.DRIVE_FIXED || t == windows.DRIVE_REMOVABLE
}
