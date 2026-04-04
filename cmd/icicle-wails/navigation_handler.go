//go:build windows && wails

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ── § Navigation ───────────────────────────────────────────────────

func (a *App) OpenPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("path not found: %w", err)
	}
	cmd := exec.Command("explorer.exe", abs)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open path: %w", err)
	}
	return nil
}

func (a *App) RevealPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("path not found: %w", err)
	}
	cmd := exec.Command("explorer.exe", "/select,"+abs)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to reveal path: %w", err)
	}
	return nil
}

func (a *App) PickFolder() (string, error) {
	path, err := pickFolderDialog()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(path), nil
}

func (a *App) FolderHint(path string) string {
	path = a.normalizePath(path, a.folders.Downloads)
	return detectFolderKind(path)
}

// ── § Saved Folders ────────────────────────────────────────────────

func (a *App) ListSavedFolders() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, len(a.saved))
	copy(out, a.saved)
	return out
}

func (a *App) SaveFolder(path string) error {
	abs, err := filepath.Abs(filepath.Clean(strings.TrimSpace(path)))
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.saved = dedupePaths(append([]string{abs}, a.saved...))
	return a.saveSavedLocked()
}

func (a *App) RemoveSavedFolder(path string) error {
	abs, err := filepath.Abs(filepath.Clean(strings.TrimSpace(path)))
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	next := make([]string, 0, len(a.saved))
	for _, p := range a.saved {
		if !strings.EqualFold(p, abs) {
			next = append(next, p)
		}
	}
	a.saved = next
	return a.saveSavedLocked()
}
