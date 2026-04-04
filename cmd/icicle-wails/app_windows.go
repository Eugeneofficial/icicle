//go:build windows && wails

// Package main is the Wails desktop application entry point.
//
// Code organization (2700+ lines):
//
//	§ Lifecycle:        startup, shutdown, Version, Defaults     (~30 lines)
//	§ Logging:          appendLog, ClearLog, WatchLog, pipe      (~50 lines)
//	§ Scan - Tree:      RunTree, RunTreeFast                     (~80 lines)
//	§ Scan - Heavy:     RunHeavy, RunHeavyFast, FullScan+Cancel  (~150 lines)
//	§ Export:           ExportHeavy                              (~70 lines)
//	§ Watch:            StartWatch, StopWatch, Diagnostics       (~90 lines)
//	§ Drives:           ListDrives, DriveHistory, OpenDrive      (~80 lines)
//	§ Navigation:       OpenPath, RevealPath, PickFolder         (~50 lines)
//	§ Saved Folders:    List/Save/Remove, load/save              (~70 lines)
//	§ File Ops:         Move, Delete, Batch, Undo                (~130 lines)
//	§ Empty Dirs:       Clean, Find, Delete                      (~140 lines)
//	§ Cleanup Presets:  Scan, Apply, match helpers               (~120 lines)
//	§ Extension Stats:  ExtensionStats, ExtensionStatsFast       (~60 lines)
//	§ WizMap:           WizMap, WizMapTurbo, Delta               (~160 lines)
//	§ Duplicates:       Names, FinderV2, Keep                    (~160 lines)
//	§ Scheduled Scan:   Start/Stop, Loop, Snapshots              (~180 lines)
//	§ Snapshots:        List, Diff, TreemapDiff, Export          (~330 lines)
//	§ Utilities:        normalizePath, markNewHeavy, csv helpers (~120 lines)
//
// Other files in this package:
//
//	tray_windows.go           - System tray integration
//	update_windows.go         - Auto-update logic
//	routing_windows.go        - Routing rules engine
//	filters_windows.go        - Filtered scan wrappers
//	cleanup_presets_windows.go - Cleanup preset management
//	schedule_cleanup_profile_windows.go - Scheduled cleanup + profiles
//
// Refactoring plan:
// Each § section above is a candidate for extraction into its own file
// (e.g. scan_handler.go, snapshot_handler.go, fileops_handler.go).
// See cmd/icicle-wails/handlers/ for the foundation package.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"icicle/internal/meta"
	"icicle/internal/scan"
	"icicle/internal/ui"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type HeavyItem struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Human string `json:"human"`
	New   bool   `json:"new,omitempty"`
}

type HeavyResult struct {
	Items      []HeavyItem `json:"items"`
	Seen       int         `json:"seen"`
	Limited    bool        `json:"limited"`
	DurationMS int64       `json:"durationMs"`
}

type TreeResult struct {
	Output     string `json:"output"`
	Seen       int    `json:"seen"`
	Limited    bool   `json:"limited"`
	DurationMS int64  `json:"durationMs"`
}

type ExtStat struct {
	Ext   string `json:"ext"`
	Count int    `json:"count"`
	Size  int64  `json:"size"`
	Human string `json:"human"`
}

type DupStat struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Paths []string `json:"paths"`
}

type DupV2Group struct {
	Key   string   `json:"key"`
	Count int      `json:"count"`
	Total int64    `json:"total"`
	Human string   `json:"human"`
	Paths []string `json:"paths"`
}

type DriveInfo struct {
	Drive      string  `json:"drive"`
	Total      int64   `json:"total"`
	Free       int64   `json:"free"`
	Used       int64   `json:"used"`
	UsedHuman  string  `json:"usedHuman"`
	TotalHuman string  `json:"totalHuman"`
	UsedRatio  float64 `json:"usedRatio"`
}

type Defaults struct {
	Home      string `json:"home"`
	Downloads string `json:"downloads"`
	Desktop   string `json:"desktop"`
	Documents string `json:"documents"`
	Version   string `json:"version"`
}

type moveRecord struct {
	From string
	To   string
}

type userFolders struct {
	Home      string
	Downloads string
	Desktop   string
	Documents string
}

type App struct {
	ctx      context.Context
	appPath  string
	mu       sync.Mutex
	scanMu   sync.Mutex
	logBuf   bytes.Buffer
	watchCmd *exec.Cmd
	watchOn  bool

	folders userFolders
	cfgPath string
	saved   []string
	moves   []moveRecord
	tray    *trayBridge

	fullScan struct {
		Running    bool
		Done       bool
		Path       string
		Seen       int
		DurationMS int64
		Items      []HeavyItem
		Err        string
	}
	fullCancel context.CancelFunc

	driveHistory    map[string][]DriveHistoryPoint
	schedule        scheduledScanState
	cleanup         scheduledCleanupState
	heavySeenByRoot map[string]map[string]struct{}
}

type DriveHistoryPoint struct {
	AtUnix int64 `json:"atUnix"`
	Used   int64 `json:"used"`
	Total  int64 `json:"total"`
}

type DriveHistory struct {
	Drive  string              `json:"drive"`
	Points []DriveHistoryPoint `json:"points"`
}

type ExtStatsResult struct {
	Items      []ExtStat `json:"items"`
	Seen       int       `json:"seen"`
	Limited    bool      `json:"limited"`
	DurationMS int64     `json:"durationMs"`
}

type VizRect struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Size       int64  `json:"size"`
	Human      string `json:"human"`
	Delta      int64  `json:"delta,omitempty"`
	DeltaHuman string `json:"deltaHuman,omitempty"`
	IsNew      bool   `json:"isNew,omitempty"`
}

type WizMapResult struct {
	Path       string    `json:"path"`
	Total      int64     `json:"total"`
	TotalHuman string    `json:"totalHuman"`
	Seen       int       `json:"seen"`
	Limited    bool      `json:"limited"`
	DurationMS int64     `json:"durationMs"`
	Rects      []VizRect `json:"rects"`
	Ext        []ExtStat `json:"ext"`
}

type SnapshotInfo struct {
	File string `json:"file"`
	At   int64  `json:"at"`
	Path string `json:"path"`
}

type ScheduleStatus struct {
	Running     bool   `json:"running"`
	Path        string `json:"path"`
	IntervalSec int    `json:"intervalSec"`
	TopN        int    `json:"topN"`
	MaxFiles    int    `json:"maxFiles"`
	Workers     int    `json:"workers"`
	LastRunUnix int64  `json:"lastRunUnix"`
	LastStatus  string `json:"lastStatus"`
}

type scheduledScanState struct {
	Running     bool
	Path        string
	IntervalSec int
	TopN        int
	MaxFiles    int
	Workers     int
	LastRunUnix int64
	LastStatus  string
	Cancel      context.CancelFunc
}

type CleanupCandidate struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Human  string `json:"human"`
	Reason string `json:"reason"`
	Risk   string `json:"risk"`
}

type CleanupPresetResult struct {
	Preset     string             `json:"preset"`
	Count      int                `json:"count"`
	TotalBytes int64              `json:"totalBytes"`
	TotalHuman string             `json:"totalHuman"`
	Candidates []CleanupCandidate `json:"candidates"`
	RiskLow    int                `json:"riskLow"`
	RiskMedium int                `json:"riskMedium"`
	RiskHigh   int                `json:"riskHigh"`
}

type SnapshotDiffItem struct {
	Path   string `json:"path"`
	Delta  int64  `json:"delta"`
	Human  string `json:"human"`
	Status string `json:"status"`
}

type SnapshotDiffResult struct {
	Left      string             `json:"left"`
	Right     string             `json:"right"`
	Added     int                `json:"added"`
	Removed   int                `json:"removed"`
	Changed   int                `json:"changed"`
	Top       []SnapshotDiffItem `json:"top"`
	CreatedAt int64              `json:"createdAt"`
}

type SnapshotTreemapItem struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Delta   int64  `json:"delta"`
	Human   string `json:"human"`
	Status  string `json:"status"`
	AbsSize int64  `json:"absSize"`
}

type SnapshotTreemapCompare struct {
	Left       string                `json:"left"`
	Right      string                `json:"right"`
	TotalAbs   int64                 `json:"totalAbs"`
	TotalHuman string                `json:"totalHuman"`
	Items      []SnapshotTreemapItem `json:"items"`
}

type DuplicateActionResult struct {
	Rule      string      `json:"rule"`
	KeptPath  string      `json:"keptPath"`
	Deleted   BatchResult `json:"deleted"`
	Skipped   int         `json:"skipped"`
	GroupSize int         `json:"groupSize"`
}

type WatchHealthItem struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Entries int    `json:"entries"`
}

type HeavyFullProgress struct {
	Running    bool        `json:"running"`
	Done       bool        `json:"done"`
	Path       string      `json:"path"`
	Seen       int         `json:"seen"`
	DurationMS int64       `json:"durationMs"`
	Items      []HeavyItem `json:"items"`
	Error      string      `json:"error"`
}

type BatchResult struct {
	Processed int      `json:"processed"`
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors"`
}

func NewApp(appPath string) *App {
	return &App{
		appPath:         appPath,
		heavySeenByRoot: map[string]map[string]struct{}{},
	}
}

// ── § Lifecycle ────────────────────────────────────────────────────

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.folders = detectUserFolders()
	cfgDir, _ := os.UserConfigDir()
	if strings.TrimSpace(cfgDir) == "" {
		cfgDir = a.folders.Home
	}
	a.cfgPath = filepath.Join(cfgDir, "icicle", "saved_folders.json")
	_ = a.loadSaved()
	a.tray = startTray(func() {
		runtime.Show(a.ctx)
		runtime.WindowUnminimise(a.ctx)
	})
}

func (a *App) shutdown(context.Context) {
	a.StopWatch()
	a.StopScheduledScan()
	a.StopScheduledCleanup()
	if a.tray != nil {
		a.tray.Close()
	}
}

func (a *App) Version() string {
	return meta.Version
}

func (a *App) Defaults() Defaults {
	return Defaults{
		Home:      a.folders.Home,
		Downloads: a.folders.Downloads,
		Desktop:   a.folders.Desktop,
		Documents: a.folders.Documents,
		Version:   meta.Version,
	}
}

// ── § Logging ──────────────────────────────────────────────────────

const (
	maxLogBytes  = 2 * 1024 * 1024 // 2MB max log size
	keepLogBytes = 1 * 1024 * 1024 // keep last 1MB when trimming
)

func (a *App) appendLog(line string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Ensure line ends with newline
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	a.logBuf.WriteString(line)

	// Trim from front if too large (avoid reallocation by keeping reasonable chunk)
	if a.logBuf.Len() > maxLogBytes {
		full := a.logBuf.Bytes()
		cutoff := len(full) - keepLogBytes
		if cutoff > 0 {
			// Find next newline to avoid splitting lines
			for cutoff < len(full) && full[cutoff] != '\n' {
				cutoff++
			}
			if cutoff < len(full) {
				cutoff++ // skip the newline
			}
			remaining := full[cutoff:]
			a.logBuf.Reset()
			a.logBuf.Write(remaining)
		}
	}
}

func (a *App) ClearLog() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.logBuf.Reset()
}

func (a *App) WatchLog() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.logBuf.String()
}

func (a *App) WatchRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.watchOn
}

// ── § Scan: Tree ───────────────────────────────────────────────────

func (a *App) RunTree(path string, topN int, width int) (string, error) {
	path = a.normalizePath(path, a.folders.Home)
	if topN <= 0 {
		topN = 5
	}
	if width <= 0 {
		width = 22
	}
	stats, err := scan.ScanTree(path, topN, 0)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	theme := ui.Theme{NoColor: true, NoEmoji: true}
	b.WriteString(fmt.Sprintf("%s  (total: %s)\n", path, ui.HumanBytes(stats.Total)))
	limit := 20
	if len(stats.ChildNames) < limit {
		limit = len(stats.ChildNames)
	}
	for i := 0; i < limit; i++ {
		name := stats.ChildNames[i]
		size := stats.ByChild[name]
		ratio := 0.0
		if stats.Total > 0 {
			ratio = float64(size) / float64(stats.Total)
		}
		prefix := "|-"
		if i == limit-1 && stats.RootFiles == 0 {
			prefix = "`-"
		}
		b.WriteString(fmt.Sprintf("%s [DIR] %-20s %8s  %s\n", prefix, name, ui.HumanBytes(size), theme.Bar(ratio, width)))
	}
	if stats.RootFiles > 0 {
		ratio := 0.0
		if stats.Total > 0 {
			ratio = float64(stats.RootFiles) / float64(stats.Total)
		}
		b.WriteString(fmt.Sprintf("`- [FILES] %-18s %8s  %s\n", "(root)", ui.HumanBytes(stats.RootFiles), theme.Bar(ratio, width)))
	}
	b.WriteString("\nTOP FILES:\n")
	for _, file := range stats.TopFiles {
		rel, relErr := filepath.Rel(path, file.Path)
		if relErr != nil {
			rel = file.Path
		}
		b.WriteString(fmt.Sprintf("%8s  %s\n", ui.HumanBytes(file.Size), rel))
	}
	out := b.String()
	a.appendLog("> tree " + path + "\n" + out)
	return out, nil
}

func (a *App) RunTreeFast(path string, topN int, width int, maxFiles int, workers int) (TreeResult, error) {
	path = a.normalizePath(path, a.folders.Home)
	if topN <= 0 {
		topN = 5
	}
	if width <= 0 {
		width = 22
	}
	if maxFiles < 0 {
		maxFiles = 0
	}
	started := time.Now()
	a.scanMu.Lock()
	stats, seen, limited, err := scan.ScanTreeLimited(path, topN, maxFiles, workers)
	a.scanMu.Unlock()
	if err != nil {
		return TreeResult{}, err
	}
	var b strings.Builder
	theme := ui.Theme{NoColor: true, NoEmoji: true}
	b.WriteString(fmt.Sprintf("%s  (total: %s)\n", path, ui.HumanBytes(stats.Total)))
	limit := 20
	if len(stats.ChildNames) < limit {
		limit = len(stats.ChildNames)
	}
	for i := 0; i < limit; i++ {
		name := stats.ChildNames[i]
		size := stats.ByChild[name]
		ratio := 0.0
		if stats.Total > 0 {
			ratio = float64(size) / float64(stats.Total)
		}
		prefix := "|-"
		if i == limit-1 && stats.RootFiles == 0 {
			prefix = "`-"
		}
		b.WriteString(fmt.Sprintf("%s [DIR] %-20s %8s  %s\n", prefix, name, ui.HumanBytes(size), theme.Bar(ratio, width)))
	}
	if stats.RootFiles > 0 {
		ratio := 0.0
		if stats.Total > 0 {
			ratio = float64(stats.RootFiles) / float64(stats.Total)
		}
		b.WriteString(fmt.Sprintf("`- [FILES] %-18s %8s  %s\n", "(root)", ui.HumanBytes(stats.RootFiles), theme.Bar(ratio, width)))
	}
	b.WriteString("\nTOP FILES:\n")
	for _, file := range stats.TopFiles {
		rel, relErr := filepath.Rel(path, file.Path)
		if relErr != nil {
			rel = file.Path
		}
		b.WriteString(fmt.Sprintf("%8s  %s\n", ui.HumanBytes(file.Size), rel))
	}
	out := b.String()
	res := TreeResult{
		Output:     out,
		Seen:       seen,
		Limited:    limited,
		DurationMS: time.Since(started).Milliseconds(),
	}
	a.appendLog(fmt.Sprintf("> tree %s [seen=%d limited=%v ms=%d]\n%s", path, seen, limited, res.DurationMS, out))
	return res, nil
}

// ── § Scan: Heavy ──────────────────────────────────────────────────

func (a *App) RunHeavy(path string, n int) ([]HeavyItem, error) {
	path = a.normalizePath(path, a.folders.Home)
	if n <= 0 {
		n = 20
	}
	stats, err := scan.ScanTopFiles(path, n, 0)
	if err != nil {
		return nil, err
	}
	items := make([]HeavyItem, 0, len(stats.TopFiles))
	var b strings.Builder
	b.WriteString("> heavy --n " + strconv.Itoa(n) + " " + path + "\n")
	for _, f := range stats.TopFiles {
		items = append(items, HeavyItem{Path: f.Path, Size: f.Size, Human: ui.HumanBytes(f.Size)})
		rel, relErr := filepath.Rel(path, f.Path)
		if relErr != nil {
			rel = f.Path
		}
		b.WriteString(fmt.Sprintf("%8s  %s\n", ui.HumanBytes(f.Size), rel))
	}
	items = a.markNewHeavy(path, items)
	a.appendLog(b.String())
	return items, nil
}

func (a *App) RunHeavyFast(path string, n int, maxFiles int, workers int) (HeavyResult, error) {
	path = a.normalizePath(path, a.folders.Home)
	if n <= 0 {
		n = 20
	}
	if maxFiles < 0 {
		maxFiles = 0
	}
	started := time.Now()

	a.scanMu.Lock()
	stats, seen, limited, err := scan.ScanTopFilesLimited(path, n, maxFiles, workers)
	a.scanMu.Unlock()
	if err != nil {
		return HeavyResult{}, err
	}
	items := make([]HeavyItem, 0, len(stats.TopFiles))
	for _, f := range stats.TopFiles {
		items = append(items, HeavyItem{Path: f.Path, Size: f.Size, Human: ui.HumanBytes(f.Size)})
	}
	items = a.markNewHeavy(path, items)
	out := HeavyResult{
		Items:      items,
		Seen:       seen,
		Limited:    limited,
		DurationMS: time.Since(started).Milliseconds(),
	}
	a.appendLog(fmt.Sprintf("> heavy --n %d %s [seen=%d limited=%v ms=%d]", n, path, out.Seen, out.Limited, out.DurationMS))
	return out, nil
}

func (a *App) StartHeavyFullScan(path string, n int) error {
	path = a.normalizePath(path, a.folders.Home)
	if n <= 0 {
		n = 20
	}

	a.mu.Lock()
	if a.fullScan.Running {
		a.mu.Unlock()
		return fmt.Errorf("full scan is already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.fullCancel = cancel
	a.fullScan = struct {
		Running    bool
		Done       bool
		Path       string
		Seen       int
		DurationMS int64
		Items      []HeavyItem
		Err        string
	}{
		Running: true,
		Done:    false,
		Path:    path,
	}
	a.mu.Unlock()

	started := time.Now()
	go func() {
		top := scan.NewTopFiles(n)
		seen := 0
		lastPush := time.Now()
		push := func(done bool, errText string) {
			list := top.ListDesc()
			items := make([]HeavyItem, 0, len(list))
			for _, f := range list {
				items = append(items, HeavyItem{Path: f.Path, Size: f.Size, Human: ui.HumanBytes(f.Size)})
			}
			a.mu.Lock()
			a.fullScan.Running = !done
			a.fullScan.Done = done
			a.fullScan.Path = path
			a.fullScan.Seen = seen
			a.fullScan.DurationMS = time.Since(started).Milliseconds()
			a.fullScan.Items = items
			a.fullScan.Err = errText
			a.mu.Unlock()
		}

		stopErr := fmt.Errorf("full scan cancelled")
		err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			select {
			case <-ctx.Done():
				return stopErr
			default:
			}
			if err != nil {
				if isDeniedError(err) {
					if d != nil && d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				return nil
			}
			if d != nil {
				if d.IsDir() && shouldSkipDirName(strings.ToLower(d.Name())) {
					return filepath.SkipDir
				}
				if d.Type()&os.ModeSymlink != 0 {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if d.IsDir() {
					return nil
				}
				info, ierr := d.Info()
				if ierr != nil {
					return nil
				}
				top.Push(scan.FileInfo{Path: p, Size: info.Size()})
				seen++
				if seen%400 == 0 || time.Since(lastPush) > 300*time.Millisecond {
					push(false, "")
					lastPush = time.Now()
				}
			}
			return nil
		})
		if err != nil && err != stopErr {
			push(true, err.Error())
			a.appendLog("[full-heavy] failed: " + err.Error())
			return
		}
		if err == stopErr {
			push(true, "cancelled")
			a.appendLog("[full-heavy] cancelled")
			return
		}
		push(true, "")
		a.appendLog(fmt.Sprintf("[full-heavy] done: seen=%d ms=%d", seen, time.Since(started).Milliseconds()))
	}()
	return nil
}

func (a *App) CancelHeavyFullScan() {
	a.mu.Lock()
	cancel := a.fullCancel
	a.fullCancel = nil
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) GetHeavyFullProgress() HeavyFullProgress {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := HeavyFullProgress{
		Running:    a.fullScan.Running,
		Done:       a.fullScan.Done,
		Path:       a.fullScan.Path,
		Seen:       a.fullScan.Seen,
		DurationMS: a.fullScan.DurationMS,
		Error:      a.fullScan.Err,
	}
	if len(a.fullScan.Items) > 0 {
		out.Items = make([]HeavyItem, len(a.fullScan.Items))
		copy(out.Items, a.fullScan.Items)
	}
	return out
}

// ── § Drives ───────────────────────────────────────────────────────

func (a *App) ListDrives() ([]DriveInfo, error) {
	volumes, err := systemStorage()
	if err != nil {
		return nil, err
	}
	out := make([]DriveInfo, 0, len(volumes))
	for _, v := range volumes {
		ratio := 0.0
		if v.Total > 0 {
			ratio = float64(v.Used) / float64(v.Total)
		}
		out = append(out, DriveInfo{
			Drive:      v.Drive,
			Total:      v.Total,
			Free:       v.Free,
			Used:       v.Used,
			UsedHuman:  ui.HumanBytes(v.Used),
			TotalHuman: ui.HumanBytes(v.Total),
			UsedRatio:  ratio,
		})
	}
	a.recordDriveHistory(out)
	return out, nil
}

func (a *App) recordDriveHistory(items []DriveInfo) {
	const maxPoints = 120
	now := time.Now().Unix()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.driveHistory == nil {
		a.driveHistory = map[string][]DriveHistoryPoint{}
	}
	for _, d := range items {
		key := strings.ToUpper(strings.TrimSpace(d.Drive))
		if key == "" {
			continue
		}
		points := a.driveHistory[key]
		p := DriveHistoryPoint{AtUnix: now, Used: d.Used, Total: d.Total}
		if len(points) > 0 {
			last := points[len(points)-1]
			if last.Used == p.Used && last.Total == p.Total && now-last.AtUnix < 20 {
				continue
			}
		}
		points = append(points, p)
		if len(points) > maxPoints {
			points = points[len(points)-maxPoints:]
		}
		a.driveHistory[key] = points
	}
}

func (a *App) DriveHistory() []DriveHistory {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.driveHistory) == 0 {
		return []DriveHistory{}
	}
	keys := make([]string, 0, len(a.driveHistory))
	for k := range a.driveHistory {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]DriveHistory, 0, len(keys))
	for _, k := range keys {
		src := a.driveHistory[k]
		dst := make([]DriveHistoryPoint, len(src))
		copy(dst, src)
		out = append(out, DriveHistory{Drive: k, Points: dst})
	}
	return out
}

func (a *App) OpenDrive(drive string) error {
	drive = normalizeDrive(drive)
	if drive == "" {
		return fmt.Errorf("invalid drive")
	}
	cmd := exec.Command("explorer.exe", drive+`\`)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open drive %s: %w", drive, err)
	}
	return nil
}

// ── § WizMap ───────────────────────────────────────────────────────

func (a *App) WizMap(path string, maxFiles int, workers int, topDirs int, topFiles int, topExt int) (WizMapResult, error) {
	path = a.normalizePath(path, a.folders.Home)
	if maxFiles < 0 {
		maxFiles = 0
	}
	if topDirs <= 0 {
		topDirs = 24
	}
	if topFiles <= 0 {
		topFiles = 80
	}
	if topExt <= 0 {
		topExt = 30
	}
	started := time.Now()
	a.scanMu.Lock()
	stats, err := scan.ScanOverviewLimited(path, maxFiles, topFiles, topExt, workers)
	a.scanMu.Unlock()
	if err != nil {
		return WizMapResult{}, err
	}

	type kv struct {
		Name string
		Size int64
	}
	children := make([]kv, 0, len(stats.ByChild))
	for name, size := range stats.ByChild {
		children = append(children, kv{Name: name, Size: size})
	}
	sort.Slice(children, func(i, j int) bool { return children[i].Size > children[j].Size })
	if len(children) > topDirs {
		children = children[:topDirs]
	}

	rects := make([]VizRect, 0, len(children)+len(stats.TopFiles))
	for _, c := range children {
		full := filepath.Join(path, c.Name)
		if c.Name == "(root)" {
			full = path
		}
		rects = append(rects, VizRect{
			Name:  c.Name,
			Path:  full,
			Kind:  "dir",
			Size:  c.Size,
			Human: ui.HumanBytes(c.Size),
		})
	}
	for _, f := range stats.TopFiles {
		rects = append(rects, VizRect{
			Name:  filepath.Base(f.Path),
			Path:  f.Path,
			Kind:  "file",
			Size:  f.Size,
			Human: ui.HumanBytes(f.Size),
		})
	}

	ext := make([]ExtStat, 0, len(stats.ExtStats))
	for _, e := range stats.ExtStats {
		ext = append(ext, ExtStat{
			Ext:   e.Ext,
			Count: e.Count,
			Size:  e.Size,
			Human: ui.HumanBytes(e.Size),
		})
	}
	res := WizMapResult{
		Path:       path,
		Total:      stats.Total,
		TotalHuman: ui.HumanBytes(stats.Total),
		Seen:       stats.Seen,
		Limited:    stats.Limited,
		DurationMS: time.Since(started).Milliseconds(),
		Rects:      rects,
		Ext:        ext,
	}
	a.appendLog(fmt.Sprintf("[wizmap] %s seen=%d limited=%v ms=%d", path, res.Seen, res.Limited, res.DurationMS))
	return res, nil
}

func (a *App) WizMapTurbo(path string, maxFiles int, topDirs int, topFiles int, topExt int) (WizMapResult, error) {
	if topDirs <= 0 {
		topDirs = 32
	}
	if topFiles <= 0 {
		topFiles = 120
	}
	if topExt <= 0 {
		topExt = 40
	}
	workers := goruntime.NumCPU() * 4
	if workers < 24 {
		workers = 24
	}
	if workers > 128 {
		workers = 128
	}
	res, err := a.WizMap(path, maxFiles, workers, topDirs, topFiles, topExt)
	if err == nil {
		a.appendLog(fmt.Sprintf("[wizmap-turbo] workers=%d seen=%d limited=%v", workers, res.Seen, res.Limited))
	}
	return res, err
}

func (a *App) WizMapWithDelta(path string, snapshotFile string, maxFiles int, workers int, topDirs int, topFiles int, topExt int) (WizMapResult, error) {
	res, err := a.WizMap(path, maxFiles, workers, topDirs, topFiles, topExt)
	if err != nil {
		return WizMapResult{}, err
	}
	snapshotFile = strings.TrimSpace(snapshotFile)
	if snapshotFile == "" {
		return res, nil
	}
	snap, err := readSnapshotFile(snapshotFile)
	if err != nil {
		return WizMapResult{}, err
	}
	prev := make(map[string]int64, len(snap.Items))
	for _, it := range snap.Items {
		prev[normalizePathKey(it.Path)] = it.Size
	}
	a.applySnapshotDeltaToWiz(&res, prev)
	a.appendLog(fmt.Sprintf("[wizmap-delta] snapshot=%s rects=%d", snapshotFile, len(res.Rects)))
	return res, nil
}

func (a *App) applySnapshotDeltaToWiz(res *WizMapResult, prev map[string]int64) {
	if res == nil || len(res.Rects) == 0 || len(prev) == 0 {
		return
	}
	dirCache := map[string]int64{}
	for i := range res.Rects {
		r := &res.Rects[i]
		key := normalizePathKey(r.Path)
		if key == "" {
			continue
		}
		prevSize, ok := prev[key]
		if r.Kind == "dir" {
			if cached, found := dirCache[key]; found {
				prevSize = cached
				ok = true
			} else {
				sum := int64(0)
				prefix := key
				if !strings.HasSuffix(prefix, "\\") {
					prefix += "\\"
				}
				for p, s := range prev {
					if p == key || strings.HasPrefix(p, prefix) {
						sum += s
					}
				}
				if sum > 0 {
					prevSize = sum
					ok = true
					dirCache[key] = sum
				}
			}
		}
		if !ok {
			if r.Kind == "file" {
				r.IsNew = true
				r.Delta = r.Size
				r.DeltaHuman = "+" + ui.HumanBytes(r.Size)
			}
			continue
		}
		delta := r.Size - prevSize
		r.Delta = delta
		if delta > 0 {
			r.DeltaHuman = "+" + ui.HumanBytes(delta)
		} else {
			r.DeltaHuman = ui.HumanBytes(delta)
		}
		if prevSize == 0 && r.Size > 0 {
			r.IsNew = true
		}
	}
}
