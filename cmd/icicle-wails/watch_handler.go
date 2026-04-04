//go:build windows && wails

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ── § Watch ────────────────────────────────────────────────────────

func (a *App) StartWatch(path string, dryRun bool) error {
	a.mu.Lock()
	if a.watchOn {
		a.mu.Unlock()
		return fmt.Errorf("watch is already running")
	}
	a.mu.Unlock()

	path = a.normalizePath(path, a.folders.Downloads)
	args := []string{"watch"}
	if dryRun {
		args = append(args, "--dry-run")
	}
	args = append(args, path)
	cmd := exec.Command(a.appPath, args...)
	cmd.Dir = filepath.Dir(a.appPath)
	cmd.Env = append(os.Environ(), "ICICLE_ALLOW_MULTI=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	a.mu.Lock()
	a.watchCmd = cmd
	a.watchOn = true
	a.mu.Unlock()
	a.appendLog("> icicle " + strings.Join(args, " "))
	go a.pipe(stdout)
	go a.pipe(stderr)
	go func() {
		err := cmd.Wait()
		if err != nil {
			a.appendLog("[watch stopped] " + err.Error())
		} else {
			a.appendLog("[watch stopped]")
		}
		a.mu.Lock()
		a.watchCmd = nil
		a.watchOn = false
		a.mu.Unlock()
	}()
	return nil
}

func (a *App) StopWatch() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.watchCmd != nil && a.watchCmd.Process != nil {
		_ = a.watchCmd.Process.Kill()
	}
}

func (a *App) WatchDiagnostics(path string, limit int) ([]WatchHealthItem, error) {
	path = a.normalizePath(path, a.folders.Downloads)
	if limit <= 0 {
		limit = 30
	}
	out := make([]WatchHealthItem, 0, limit+1)
	check := func(p string) WatchHealthItem {
		it := WatchHealthItem{Path: p, Status: "ok"}
		entries, err := os.ReadDir(p)
		if err != nil {
			if isDeniedError(err) {
				it.Status = "denied"
				it.Reason = "access denied"
				return it
			}
			it.Status = "error"
			it.Reason = err.Error()
			return it
		}
		it.Entries = len(entries)
		if len(entries) == 0 {
			it.Status = "empty"
		}
		return it
	}
	out = append(out, check(path))
	entries, err := os.ReadDir(path)
	if err != nil {
		return out, nil
	}
	for _, e := range entries {
		if len(out) >= limit+1 {
			break
		}
		if !e.IsDir() {
			continue
		}
		if shouldSkipDirName(strings.ToLower(e.Name())) {
			out = append(out, WatchHealthItem{
				Path:   filepath.Join(path, e.Name()),
				Status: "skipped",
				Reason: "system/protected",
			})
			continue
		}
		out = append(out, check(filepath.Join(path, e.Name())))
	}
	return out, nil
}

func (a *App) pipe(r io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			a.appendLog(string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}
