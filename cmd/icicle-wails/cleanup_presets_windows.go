//go:build windows && wails

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
