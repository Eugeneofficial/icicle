//go:build windows && wails

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type DiskCleanupPreset struct {
	Drive       string `json:"drive"`
	Preset      string `json:"preset"`
	Mode        string `json:"mode"`
	IntervalSec int    `json:"intervalSec"`
	Hour        int    `json:"hour"`
	Minute      int    `json:"minute"`
	Weekday     int    `json:"weekday"`
	Safe        bool   `json:"safe"`
	DryRun      bool   `json:"dryRun"`
	MaxDelete   int    `json:"maxDelete"`
}

func (a *App) cleanupPresetsPath() string {
	cfgDir, _ := os.UserConfigDir()
	if strings.TrimSpace(cfgDir) == "" {
		cfgDir = a.folders.Home
	}
	return filepath.Join(cfgDir, "icicle", "cleanup_drive_presets.json")
}

func (a *App) readCleanupPresets() (map[string]DiskCleanupPreset, error) {
	path := a.cleanupPresetsPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]DiskCleanupPreset{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]DiskCleanupPreset
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]DiskCleanupPreset{}
	}
	return m, nil
}

func (a *App) writeCleanupPresets(m map[string]DiskCleanupPreset) error {
	path := a.cleanupPresetsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

func normalizeDiskCleanupPreset(in DiskCleanupPreset) DiskCleanupPreset {
	in.Drive = normalizeDrive(in.Drive)
	if in.Preset == "" {
		in.Preset = "dev-cache"
	}
	in.Preset = strings.ToLower(strings.TrimSpace(in.Preset))
	in.Mode = strings.ToLower(strings.TrimSpace(in.Mode))
	if in.Mode == "" {
		in.Mode = "interval"
	}
	if in.Mode != "interval" && in.Mode != "daily" && in.Mode != "weekly" {
		in.Mode = "interval"
	}
	if in.IntervalSec < 60 {
		in.IntervalSec = 900
	}
	if in.Hour < 0 || in.Hour > 23 {
		in.Hour = 2
	}
	if in.Minute < 0 || in.Minute > 59 {
		in.Minute = 30
	}
	if in.Weekday < 0 || in.Weekday > 6 {
		in.Weekday = 1
	}
	if in.MaxDelete <= 0 {
		in.MaxDelete = 150
	}
	return in
}

func (a *App) SaveCleanupPresetForDrive(p DiskCleanupPreset) error {
	p = normalizeDiskCleanupPreset(p)
	if p.Drive == "" {
		return fmt.Errorf("invalid drive")
	}
	m, err := a.readCleanupPresets()
	if err != nil {
		return err
	}
	m[p.Drive] = p
	if err := a.writeCleanupPresets(m); err != nil {
		return err
	}
	a.appendLog(fmt.Sprintf("[cleanup-preset] saved for %s", p.Drive))
	return nil
}

func (a *App) ListCleanupPresetsByDrive() ([]DiskCleanupPreset, error) {
	m, err := a.readCleanupPresets()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]DiskCleanupPreset, 0, len(keys))
	for _, k := range keys {
		p := normalizeDiskCleanupPreset(m[k])
		p.Drive = k
		out = append(out, p)
	}
	return out, nil
}

func (a *App) LoadCleanupPresetForDrive(drive string) (DiskCleanupPreset, error) {
	drive = normalizeDrive(drive)
	if drive == "" {
		return DiskCleanupPreset{}, fmt.Errorf("invalid drive")
	}
	m, err := a.readCleanupPresets()
	if err != nil {
		return DiskCleanupPreset{}, err
	}
	p, ok := m[drive]
	if !ok {
		return DiskCleanupPreset{}, fmt.Errorf("preset not found for %s", drive)
	}
	p = normalizeDiskCleanupPreset(p)
	p.Drive = drive
	return p, nil
}

type TeamPresetPack struct {
	Version        int                          `json:"version"`
	ExportedAt     int64                        `json:"exportedAt"`
	Name           string                       `json:"name"`
	CleanupByDrive map[string]DiskCleanupPreset `json:"cleanupByDrive"`
	RouteRules     []RouteRule                  `json:"routeRules"`
	SavedFolders   []string                     `json:"savedFolders"`
}

func (a *App) ExportTeamPresetPack(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "team-pack"
	}
	cleanup, err := a.readCleanupPresets()
	if err != nil {
		return "", err
	}
	rules, err := a.ListRoutingRules()
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	saved := make([]string, len(a.saved))
	copy(saved, a.saved)
	a.mu.Unlock()
	pack := TeamPresetPack{
		Version:        1,
		ExportedAt:     time.Now().Unix(),
		Name:           name,
		CleanupByDrive: cleanup,
		RouteRules:     rules,
		SavedFolders:   saved,
	}
	body, err := json.MarshalIndent(pack, "", "  ")
	if err != nil {
		return "", err
	}
	target, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Export team preset pack",
		DefaultFilename: "icicle-" + strings.ReplaceAll(name, " ", "-") + ".pack.json",
		Filters:         []wailsruntime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil || strings.TrimSpace(target) == "" {
		return "", err
	}
	if err := os.WriteFile(target, body, 0o644); err != nil {
		return "", err
	}
	a.appendLog("[team-pack] exported: " + target)
	return target, nil
}

func (a *App) ImportTeamPresetPack(mode string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "merge"
	}
	source, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:   "Import team preset pack",
		Filters: []wailsruntime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil || strings.TrimSpace(source) == "" {
		return "", err
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return "", err
	}
	var pack TeamPresetPack
	if err := json.Unmarshal(data, &pack); err != nil {
		return "", err
	}
	existing, _ := a.readCleanupPresets()
	if mode == "overwrite" {
		existing = map[string]DiskCleanupPreset{}
	}
	for k, v := range pack.CleanupByDrive {
		d := normalizeDrive(k)
		if d == "" {
			continue
		}
		v.Drive = d
		existing[d] = normalizeDiskCleanupPreset(v)
	}
	if err := a.writeCleanupPresets(existing); err != nil {
		return "", err
	}
	currentRules, _ := a.ListRoutingRules()
	rules := normalizeRouteRules(pack.RouteRules)
	if mode != "overwrite" {
		rules = mergeRouteRules(currentRules, rules)
	}
	if err := a.SaveRoutingRules(rules); err != nil {
		return "", err
	}
	a.mu.Lock()
	if mode == "overwrite" {
		a.saved = dedupePaths(pack.SavedFolders)
	} else {
		a.saved = dedupePaths(append(a.saved, pack.SavedFolders...))
	}
	err = a.saveSavedLocked()
	a.mu.Unlock()
	if err != nil {
		return "", err
	}
	a.appendLog(fmt.Sprintf("[team-pack] imported (%s): %s", mode, source))
	return source, nil
}
