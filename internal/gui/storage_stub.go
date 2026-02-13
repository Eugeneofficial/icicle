//go:build !windows

package gui

type DriveUsage struct {
	Drive string `json:"drive"`
	Total int64  `json:"total"`
	Free  int64  `json:"free"`
	Used  int64  `json:"used"`
}

func SystemStorage() ([]DriveUsage, error) {
	return []DriveUsage{}, nil
}
