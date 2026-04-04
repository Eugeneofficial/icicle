//go:build windows && wails

package main

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type driveUsage struct {
	Drive string
	Total int64
	Free  int64
	Used  int64
}

func systemStorage() ([]driveUsage, error) {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, err
	}
	out := make([]driveUsage, 0, 8)
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
		out = append(out, driveUsage{
			Drive: strings.TrimSuffix(drive, "\\"),
			Total: int64(totalBytes),
			Free:  int64(freeBytes),
			Used:  int64(totalBytes - freeBytes),
		})
	}
	return out, nil
}

func isFixedOrRemovableDrive(path string) bool {
	ptr, _ := windows.UTF16PtrFromString(path)
	t := windows.GetDriveType(ptr)
	return t == windows.DRIVE_FIXED || t == windows.DRIVE_REMOVABLE
}

func normalizeDrive(d string) string {
	d = strings.TrimSpace(d)
	d = strings.TrimSuffix(d, `\`)
	d = strings.TrimSuffix(d, `/`)
	if len(d) >= 2 && d[1] == ':' {
		letter := d[0]
		if (letter >= 'A' && letter <= 'Z') || (letter >= 'a' && letter <= 'z') {
			return strings.ToUpper(string(letter)) + ":"
		}
	}
	return ""
}

func detectUserFolders() userFolders {
	home, _ := os.UserHomeDir()
	if strings.TrimSpace(home) == "" {
		home = "."
	}
	f := userFolders{
		Home:      home,
		Downloads: filepath.Join(home, "Downloads"),
		Desktop:   filepath.Join(home, "Desktop"),
		Documents: filepath.Join(home, "Documents"),
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Explorer\User Shell Folders`, registry.QUERY_VALUE)
	if err != nil {
		return f
	}
	defer k.Close()
	read := func(name, fallback string) string {
		v, _, err := k.GetStringValue(name)
		if err != nil || strings.TrimSpace(v) == "" {
			return fallback
		}
		v = expandWindowsEnv(v)
		if strings.TrimSpace(v) == "" {
			return fallback
		}
		return filepath.Clean(v)
	}
	f.Desktop = read("Desktop", f.Desktop)
	f.Documents = read("Personal", f.Documents)
	f.Downloads = read("{374DE290-123F-4565-9164-39C4925E467B}", f.Downloads)
	return f
}

var winEnvPattern = regexp.MustCompile(`%([^%]+)%`)

func expandWindowsEnv(in string) string {
	out := os.ExpandEnv(in)
	out = winEnvPattern.ReplaceAllStringFunc(out, func(m string) string {
		key := strings.Trim(m, "%")
		if key == "" {
			return m
		}
		if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
			return v
		}
		return m
	})
	return out
}

func deleteToRecycleBin(path string) error {
	script := `Add-Type -AssemblyName Microsoft.VisualBasic; [Microsoft.VisualBasic.FileIO.FileSystem]::DeleteFile($args[0], [Microsoft.VisualBasic.FileIO.UIOption]::OnlyErrorDialogs, [Microsoft.VisualBasic.FileIO.RecycleOption]::SendToRecycleBin)`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script, path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, string(out))
	}
	return nil
}

func deleteDirToRecycleBin(path string) error {
	script := `Add-Type -AssemblyName Microsoft.VisualBasic; [Microsoft.VisualBasic.FileIO.FileSystem]::DeleteDirectory($args[0], [Microsoft.VisualBasic.FileIO.UIOption]::OnlyErrorDialogs, [Microsoft.VisualBasic.FileIO.RecycleOption]::SendToRecycleBin)`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script, path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, string(out))
	}
	return nil
}

func pickFolderDialog() (string, error) {
	script := `
Add-Type -AssemblyName System.Windows.Forms
$dlg = New-Object System.Windows.Forms.FolderBrowserDialog
$dlg.Description = 'Select folder'
$dlg.ShowNewFolderButton = $true
if ($dlg.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {
  Write-Output $dlg.SelectedPath
}
`
	cmd := exec.Command("powershell", "-NoProfile", "-STA", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func detectFolderKind(path string) string {
	base := strings.ToLower(filepath.Base(filepath.Clean(path)))
	switch base {
	case "downloads":
		return "Downloads"
	case "videos", "video":
		return "Videos"
	case "pictures", "images", "photos":
		return "Pictures"
	case "documents", "docs":
		return "Documents"
	case "desktop":
		return "Desktop"
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "Unknown"
	}
	extCount := map[string]int{}
	codeHits := 0
	dirHits := 0
	for i, e := range entries {
		if i >= 400 {
			break
		}
		if e.IsDir() {
			name := strings.ToLower(e.Name())
			if name == ".git" || name == "src" || name == "node_modules" || name == "vendor" {
				codeHits++
			}
			dirHits++
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		extCount[ext]++
	}
	if codeHits >= 2 {
		return "Code Project"
	}
	score := func(exts ...string) int {
		sum := 0
		for _, e := range exts {
			sum += extCount[e]
		}
		return sum
	}
	video := score(".mp4", ".mkv", ".mov", ".avi", ".webm")
	archive := score(".zip", ".rar", ".7z", ".tar", ".gz")
	pics := score(".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp")
	docs := score(".pdf", ".doc", ".docx", ".txt", ".md", ".xlsx", ".pptx")
	maxKind := "Mixed Folder"
	maxVal := video
	if archive > maxVal {
		maxKind, maxVal = "Archive Folder", archive
	}
	if pics > maxVal {
		maxKind, maxVal = "Pictures Folder", pics
	}
	if docs > maxVal {
		maxKind, maxVal = "Documents Folder", docs
	}
	if maxVal == 0 && dirHits > 0 {
		return "Workspace Folder"
	}
	return maxKind
}

func isDeniedError(err error) bool {
	if os.IsPermission(err) || errors.Is(err, fs.ErrPermission) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "access is denied") || strings.Contains(msg, "permission denied")
}

func shouldSkipDirName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "$recycle.bin", "system volume information", "windowsapps", "$winreagent", "$extend":
		return true
	default:
		return false
	}
}

func hashFileQuick(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	buf := make([]byte, 1024*1024)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", err
	}
	_, _ = h.Write(buf[:n])
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
