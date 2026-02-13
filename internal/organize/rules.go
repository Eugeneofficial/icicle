package organize

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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

	ext := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), ext)
	dir := filepath.Dir(path)
	for i := 1; i <= 9999; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
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
	return os.Rename(srcPath, dstPath)
}
