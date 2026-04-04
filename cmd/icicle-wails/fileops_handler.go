//go:build windows && wails

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"icicle/internal/organize"
)

// ── § File Operations ──────────────────────────────────────────────

func (a *App) MoveFile(src string, dstDir string) (string, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", fmt.Errorf("path is required")
	}
	dstDir = strings.TrimSpace(dstDir)
	if dstDir == "" {
		auto, ok := a.resolveAutoDestination(src)
		if !ok {
			return "", fmt.Errorf("no auto destination for extension")
		}
		dstDir = auto
	}
	dst := filepath.Join(dstDir, filepath.Base(src))
	uniqueDst, err := organize.EnsureUniquePath(dst)
	if err != nil {
		return "", err
	}
	if err := organize.MoveFile(src, uniqueDst); err != nil {
		return "", err
	}
	a.pushMove(src, uniqueDst)
	a.appendLog("[move] " + src + " -> " + uniqueDst)
	return uniqueDst, nil
}

func (a *App) DeleteFile(path string, safe bool) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("refusing to delete directory")
	}
	if safe {
		if err := deleteToRecycleBin(path); err != nil {
			return err
		}
		a.appendLog("[recycle] " + path)
		return nil
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	a.appendLog("[delete] " + path)
	return nil
}

func (a *App) BatchMove(paths []string, dstDir string, auto bool) BatchResult {
	res := BatchResult{Processed: len(paths)}
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		dest := dstDir
		if auto {
			dest = ""
		}
		if _, err := a.MoveFile(p, dest); err != nil {
			res.Failed++
			if len(res.Errors) < 20 {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", p, err))
			}
			continue
		}
		res.Succeeded++
	}
	return res
}

func (a *App) BatchDelete(paths []string, safe bool) BatchResult {
	res := BatchResult{Processed: len(paths)}
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if err := a.DeleteFile(p, safe); err != nil {
			res.Failed++
			if len(res.Errors) < 20 {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", p, err))
			}
			continue
		}
		res.Succeeded++
	}
	return res
}

func (a *App) UndoMove() (string, error) {
	rec, ok := a.peekMove()
	if !ok {
		return "", fmt.Errorf("no move history")
	}
	if _, err := os.Stat(rec.To); err != nil {
		return "", fmt.Errorf("moved file no longer exists: %s", rec.To)
	}
	target, err := organize.EnsureUniquePath(rec.From)
	if err != nil {
		return "", err
	}
	if err := organize.MoveFile(rec.To, target); err != nil {
		return "", err
	}
	a.dropLastMove()
	a.appendLog("[undo-move] " + rec.To + " -> " + target)
	return target, nil
}

// ── § Empty Dirs ───────────────────────────────────────────────────

func (a *App) CleanEmpty(path string) (int, error) {
	path = a.normalizePath(path, a.folders.Home)
	removed := 0
	for pass := 0; pass < 3; pass++ {
		dirs := []string{}
		err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				dirs = append(dirs, p)
			}
			return nil
		})
		if err != nil {
			return removed, err
		}
		sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
		passRemoved := 0
		for _, dir := range dirs {
			if strings.EqualFold(filepath.Clean(dir), filepath.Clean(path)) {
				continue
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 0 {
				continue
			}
			if err := os.Remove(dir); err == nil {
				removed++
				passRemoved++
			}
		}
		if passRemoved == 0 {
			break
		}
	}
	a.appendLog(fmt.Sprintf("[clean-empty] %s removed=%d", path, removed))
	return removed, nil
}

func (a *App) FindEmptyDirs(path string, limit int) ([]string, error) {
	path = a.normalizePath(path, a.folders.Home)
	if limit <= 0 {
		limit = 5000
	}
	dirs := make([]string, 0, 128)
	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			if isDeniedError(err) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Clean(p), filepath.Clean(path)) {
			return nil
		}
		if shouldSkipDirName(d.Name()) {
			return filepath.SkipDir
		}
		entries, rerr := os.ReadDir(p)
		if rerr != nil {
			if isDeniedError(rerr) {
				return filepath.SkipDir
			}
			return nil
		}
		if len(entries) == 0 {
			dirs = append(dirs, p)
			if len(dirs) >= limit {
				return fmt.Errorf("limit reached")
			}
		}
		return nil
	})
	if err != nil && err.Error() != "limit reached" {
		return nil, err
	}
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	return dirs, nil
}

func (a *App) DeleteEmptyDirsToRecycle(paths []string) BatchResult {
	res := BatchResult{Processed: len(paths)}
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			res.Failed++
			if len(res.Errors) < 20 {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", p, err))
			}
			continue
		}
		if !info.IsDir() {
			res.Failed++
			if len(res.Errors) < 20 {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: not a directory", p))
			}
			continue
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			res.Failed++
			if len(res.Errors) < 20 {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", p, err))
			}
			continue
		}
		if len(entries) != 0 {
			res.Failed++
			if len(res.Errors) < 20 {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: no longer empty", p))
			}
			continue
		}
		if err := deleteDirToRecycleBin(p); err != nil {
			res.Failed++
			if len(res.Errors) < 20 {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", p, err))
			}
			continue
		}
		res.Succeeded++
	}
	a.appendLog(fmt.Sprintf("[empty-dirs] moved to recycle: %d/%d", res.Succeeded, res.Processed))
	return res
}
