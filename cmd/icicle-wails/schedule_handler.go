//go:build windows && wails

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ── § Scheduled Scan ───────────────────────────────────────────────

func (a *App) StartScheduledScan(path string, intervalSec int, n int, maxFiles int, workers int) error {
	path = a.normalizePath(path, a.folders.Downloads)
	if intervalSec < 30 {
		intervalSec = 30
	}
	if n <= 0 {
		n = 20
	}
	if maxFiles < 0 {
		maxFiles = 0
	}

	a.mu.Lock()
	if a.schedule.Running {
		a.mu.Unlock()
		return fmt.Errorf("scheduled scan is already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.schedule = scheduledScanState{
		Running:     true,
		Path:        path,
		IntervalSec: intervalSec,
		TopN:        n,
		MaxFiles:    maxFiles,
		Workers:     workers,
		LastStatus:  "started",
		Cancel:      cancel,
	}
	a.mu.Unlock()

	go a.scheduledLoop(ctx)
	a.appendLog(fmt.Sprintf("[schedule] started: every %ds path=%s", intervalSec, path))
	return nil
}

func (a *App) RunScheduledScanOnce(path string, n int, maxFiles int, workers int) (string, error) {
	path = a.normalizePath(path, a.folders.Downloads)
	if n <= 0 {
		n = 20
	}
	res, err := a.RunHeavyFast(path, n, maxFiles, workers)
	if err != nil {
		return "", err
	}
	filePath, err := a.saveSnapshot(path, n, maxFiles, res)
	if err != nil {
		return "", err
	}
	a.appendLog("[schedule-once] " + filePath)
	return filePath, nil
}

func (a *App) scheduledLoop(ctx context.Context) {
	run := func() {
		a.mu.Lock()
		st := a.schedule
		a.mu.Unlock()
		path := st.Path
		n := st.TopN
		maxFiles := st.MaxFiles
		workers := st.Workers
		started := time.Now()
		res, err := a.RunHeavyFast(path, n, maxFiles, workers)
		status := "ok"
		if err != nil {
			status = "error: " + err.Error()
		} else {
			filePath, serr := a.saveSnapshot(path, n, maxFiles, res)
			if serr != nil {
				status = "snapshot error: " + serr.Error()
			} else {
				status = "ok: " + filepath.Base(filePath)
			}
		}
		a.mu.Lock()
		a.schedule.LastRunUnix = time.Now().Unix()
		a.schedule.LastStatus = status
		a.mu.Unlock()
		a.appendLog(fmt.Sprintf("[schedule] run finished in %d ms (%s)", time.Since(started).Milliseconds(), status))
	}

	run()
	a.mu.Lock()
	interval := a.schedule.IntervalSec
	a.mu.Unlock()
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func (a *App) StopScheduledScan() {
	a.mu.Lock()
	cancel := a.schedule.Cancel
	running := a.schedule.Running
	a.schedule.Running = false
	a.schedule.Cancel = nil
	a.mu.Unlock()
	if running {
		a.appendLog("[schedule] stopped")
	}
	if cancel != nil {
		cancel()
	}
}

func (a *App) ScheduledScanStatus() ScheduleStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return ScheduleStatus{
		Running:     a.schedule.Running,
		Path:        a.schedule.Path,
		IntervalSec: a.schedule.IntervalSec,
		TopN:        a.schedule.TopN,
		MaxFiles:    a.schedule.MaxFiles,
		Workers:     a.schedule.Workers,
		LastRunUnix: a.schedule.LastRunUnix,
		LastStatus:  a.schedule.LastStatus,
	}
}

func (a *App) saveSnapshot(path string, n int, maxFiles int, res HeavyResult) (string, error) {
	reportDir, err := a.reportDir()
	if err != nil {
		return "", err
	}
	type payload struct {
		AtUnix     int64       `json:"atUnix"`
		Path       string      `json:"path"`
		TopN       int         `json:"topN"`
		MaxFiles   int         `json:"maxFiles"`
		Seen       int         `json:"seen"`
		Limited    bool        `json:"limited"`
		DurationMS int64       `json:"durationMs"`
		Items      []HeavyItem `json:"items"`
	}
	data, err := json.MarshalIndent(payload{
		AtUnix:     time.Now().Unix(),
		Path:       path,
		TopN:       n,
		MaxFiles:   maxFiles,
		Seen:       res.Seen,
		Limited:    res.Limited,
		DurationMS: res.DurationMS,
		Items:      res.Items,
	}, "", "  ")
	if err != nil {
		return "", err
	}
	fileName := "heavy-snapshot-" + time.Now().Format("20060102-150405") + ".json"
	target := filepath.Join(reportDir, fileName)
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return "", err
	}
	return target, nil
}

func (a *App) reportDir() (string, error) {
	cfgDir, _ := os.UserConfigDir()
	if strings.TrimSpace(cfgDir) == "" {
		cfgDir = a.folders.Home
	}
	dir := filepath.Join(cfgDir, "icicle", "reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}
