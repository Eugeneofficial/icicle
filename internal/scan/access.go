package scan

import (
	"errors"
	"io/fs"
	"os"
	"strings"
)

// IsAccessDenied reports whether err is likely caused by
// insufficient permissions (Windows "Access is denied" / POSIX EPERM).
func IsAccessDenied(err error) bool {
	if os.IsPermission(err) {
		return true
	}
	if errors.Is(err, fs.ErrPermission) {
		return true
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		if os.IsPermission(pe.Err) || errors.Is(pe.Err, fs.ErrPermission) {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "access is denied") || strings.Contains(msg, "permission denied")
}
