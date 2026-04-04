//go:build windows && wails

package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"icicle/internal/scan"
	"icicle/internal/ui"
)

// ── § Cleanup Presets ──────────────────────────────────────────────

func (a *App) ScanCleanupPreset(path string, preset string, limit int, maxFiles int) (CleanupPresetResult, error) {
	path = a.normalizePath(path, a.folders.Home)
	preset = strings.ToLower(strings.TrimSpace(preset))
	if preset == "" {
		preset = "dev-cache"
	}
	if limit <= 0 {
		limit = 120
	}
	if maxFiles < 0 {
		maxFiles = 0
	}
	out := CleanupPresetResult{Preset: preset}
	candidates := make([]CleanupCandidate, 0, limit+64)
	seen, err := scan.WalkAllLimit(path, maxFiles, func(p string, size int64) {
		ok, reason := matchCleanupPreset(preset, p)
		if !ok {
			return
		}
		risk := cleanupRiskLevel(p)
		candidates = append(candidates, CleanupCandidate{
			Path:   p,
			Size:   size,
			Human:  ui.HumanBytes(size),
			Reason: reason,
			Risk:   risk,
		})
		switch risk {
		case "high":
			out.RiskHigh++
		case "medium":
			out.RiskMedium++
		default:
			out.RiskLow++
		}
		out.TotalBytes += size
	})
	if err != nil {
		return CleanupPresetResult{}, err
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Size > candidates[j].Size })
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	out.Candidates = candidates
	out.Count = len(candidates)
	out.TotalHuman = ui.HumanBytes(out.TotalBytes)
	a.appendLog(fmt.Sprintf("[preset-scan] %s seen=%d candidates=%d", preset, seen, out.Count))
	return out, nil
}

func (a *App) ApplyPresetCleanup(paths []string, safe bool) BatchResult {
	return a.BatchDelete(paths, safe)
}

func matchCleanupPreset(preset string, path string) (bool, string) {
	lp := strings.ToLower(path)
	ext := strings.ToLower(filepath.Ext(lp))
	switch preset {
	case "games":
		if strings.Contains(lp, `\\shadercache\\`) || strings.Contains(lp, `\\crashdumps\\`) {
			return true, "game cache"
		}
		switch ext {
		case ".msi", ".iso", ".tmp", ".bak", ".dmp", ".log", ".crdownload", ".part":
			return true, "game installer/cache file"
		}
		return false, ""
	case "media":
		switch ext {
		case ".tmp", ".part", ".crdownload", ".download", ".m3u8", ".ts", ".srt.tmp":
			return true, "media temp file"
		}
		if strings.Contains(lp, `\\cache\\`) && (strings.Contains(lp, `video`) || strings.Contains(lp, `media`)) {
			return true, "media cache"
		}
		return false, ""
	case "dev-cache":
		devCacheMarks := []string{
			`\\node_modules\\.cache\\`,
			`\\appdata\\local\\npm-cache\\`,
			`\\appdata\\local\\pnpm\\store\\`,
			`\\.nuget\\packages\\`,
			`\\appdata\\local\\pip\\cache\\`,
			`\\appdata\\local\\go-build\\`,
			`\\appdata\\roaming\\code\\cache\\`,
		}
		for _, m := range devCacheMarks {
			if strings.Contains(lp, m) {
				return true, "dev cache"
			}
		}
		if ext == ".tmp" || ext == ".log" {
			return true, "dev temp/log"
		}
		return false, ""
	default:
		return false, ""
	}
}

func cleanupRiskLevel(path string) string {
	p := strings.ToLower(path)
	highMarks := []string{`\\desktop\\`, `\\documents\\`, `\\pictures\\`, `\\videos\\`}
	for _, m := range highMarks {
		if strings.Contains(p, m) {
			return "high"
		}
	}
	mediumMarks := []string{`\\downloads\\`, `\\music\\`}
	for _, m := range mediumMarks {
		if strings.Contains(p, m) {
			return "medium"
		}
	}
	return "low"
}

// ── § Extension Stats ──────────────────────────────────────────────

func (a *App) ExtensionStats(path string, limit int) ([]ExtStat, error) {
	path = a.normalizePath(path, a.folders.Home)
	byExt := map[string]ExtStat{}
	err := scan.WalkAll(path, func(p string, size int64) {
		ext := strings.ToLower(filepath.Ext(p))
		if ext == "" {
			ext = "(no_ext)"
		}
		cur := byExt[ext]
		cur.Ext = ext
		cur.Count++
		cur.Size += size
		byExt[ext] = cur
	})
	if err != nil {
		return nil, err
	}
	out := make([]ExtStat, 0, len(byExt))
	for _, v := range byExt {
		v.Human = ui.HumanBytes(v.Size)
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (a *App) ExtensionStatsFast(path string, limit int, maxFiles int, workers int) (ExtStatsResult, error) {
	path = a.normalizePath(path, a.folders.Home)
	if limit <= 0 {
		limit = 20
	}
	if maxFiles < 0 {
		maxFiles = 0
	}
	started := time.Now()
	a.scanMu.Lock()
	items, seen, limited, err := scan.ScanExtStatsLimited(path, maxFiles, workers)
	a.scanMu.Unlock()
	if err != nil {
		return ExtStatsResult{}, err
	}

	out := make([]ExtStat, 0, len(items))
	for _, it := range items {
		out = append(out, ExtStat{
			Ext:   it.Ext,
			Count: it.Count,
			Size:  it.Size,
			Human: ui.HumanBytes(it.Size),
		})
	}
	if len(out) > limit {
		out = out[:limit]
	}
	res := ExtStatsResult{
		Items:      out,
		Seen:       seen,
		Limited:    limited,
		DurationMS: time.Since(started).Milliseconds(),
	}
	a.appendLog(fmt.Sprintf("[extensions-fast] %s seen=%d limited=%v ms=%d", path, res.Seen, res.Limited, res.DurationMS))
	return res, nil
}
