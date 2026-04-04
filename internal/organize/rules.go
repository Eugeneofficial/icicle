package organize

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const copyBufSize = 1 << 20 // 1 MB — оптимально для больших файлов на SSD

var byExtension = map[string]string{
	".mp4":  "Videos",
	".mov":  "Videos",
	".mkv":  "Videos",
	".avi":  "Videos",
	".webm": "Videos",
	".zip":  "Archives",
	".rar":  "Archives",
	".7z":   "Archives",
	".tar":  "Archives",
	".gz":   "Archives",
	".bz2":  "Archives",
	".xz":   "Archives",
	".jpg":  "Pictures",
	".jpeg": "Pictures",
	".png":  "Pictures",
	".gif":  "Pictures",
	".webp": "Pictures",
	".bmp":  "Pictures",
	".heic": "Pictures",
	".pdf":  "Documents",
	".doc":  "Documents",
	".docx": "Documents",
	".txt":  "Documents",
	".md":   "Documents",
	".xls":  "Documents",
	".xlsx": "Documents",
	".ppt":  "Documents",
	".pptx": "Documents",
	".exe":  "Apps",
	".msi":  "Apps",
	".apk":  "Apps",
}

func DestinationDir(home string, srcPath string) (string, bool) {
	ext := strings.ToLower(filepath.Ext(srcPath))
	category, ok := byExtension[ext]
	if !ok {
		return "", false
	}
	return filepath.Join(home, category), true
}

func EnsureUniquePath(path string) (string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path, nil
	} else if err != nil {
		return "", err
	}

	dir := filepath.Dir(path)
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	for i := 1; i <= 9999; i++ {
		var candidate string
		if ext != "" {
			candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		} else {
			candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)", base, i))
		}
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("cannot resolve unique name for %s", path)
}

func MoveFile(srcPath, dstPath string) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	if err := os.Rename(srcPath, dstPath); err == nil {
		return nil
	} else if !isCrossDeviceRename(err) {
		return err
	}
	return copyThenDelete(srcPath, dstPath)
}

func isCrossDeviceRename(err error) bool {
	if errors.Is(err, syscall.EXDEV) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "cross-device link") ||
		strings.Contains(msg, "not same device") ||
		strings.Contains(msg, "different disk drive")
}

func copyThenDelete(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}

	srcInfo, err := src.Stat()
	if err != nil {
		_ = src.Close()
		return err
	}

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, srcInfo.Mode().Perm())
	if err != nil {
		_ = src.Close()
		return err
	}
	copyErr := func() error {
		buf := make([]byte, copyBufSize)
		if _, err := io.CopyBuffer(dst, src, buf); err != nil {
			return err
		}
		if err := dst.Sync(); err != nil {
			return err
		}
		return nil
	}()
	closeErr := dst.Close()
	if copyErr != nil {
		_ = src.Close()
		_ = os.Remove(dstPath)
		return copyErr
	}
	if closeErr != nil {
		_ = src.Close()
		_ = os.Remove(dstPath)
		return closeErr
	}
	if err := src.Close(); err != nil {
		_ = os.Remove(dstPath)
		return err
	}
	if err := os.Remove(srcPath); err != nil {
		_ = os.Remove(dstPath)
		return err
	}
	return nil
}
