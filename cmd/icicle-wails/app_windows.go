//go:build windows && wails

package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"icicle/internal/meta"
	"icicle/internal/organize"
	"icicle/internal/scan"
	"icicle/internal/ui"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type HeavyItem struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Human string `json:"human"`
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

	driveHistory map[string][]DriveHistoryPoint
	schedule     scheduledScanState
	cleanup      scheduledCleanupState
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
	Name  string `json:"name"`
	Path  string `json:"path"`
	Kind  string `json:"kind"`
	Size  int64  `json:"size"`
	Human string `json:"human"`
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
	return &App{appPath: appPath}
}

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

func (a *App) appendLog(line string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.logBuf.WriteString(line)
	if !strings.HasSuffix(line, "\n") {
		a.logBuf.WriteString("\n")
	}
	if a.logBuf.Len() > 2*1024*1024 {
		b := a.logBuf.Bytes()
		a.logBuf.Reset()
		a.logBuf.Write(b[len(b)-1024*1024:])
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

func (a *App) RunTree(path string, topN int, width int) (string, error) {
	path = a.normalizePath(path, a.folders.Home)
	if topN <= 0 {
		topN = 5
	}
	if width <= 0 {
		width = 22
	}
	stats, err := scan.ScanTree(path, topN)
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
	prevWorkers, hadWorkers := os.LookupEnv("ICICLE_SCAN_WORKERS")
	if workers > 0 {
		_ = os.Setenv("ICICLE_SCAN_WORKERS", strconv.Itoa(workers))
	}
	stats, seen, limited, err := scan.ScanTreeLimited(path, topN, maxFiles)
	if workers > 0 {
		if hadWorkers {
			_ = os.Setenv("ICICLE_SCAN_WORKERS", prevWorkers)
		} else {
			_ = os.Unsetenv("ICICLE_SCAN_WORKERS")
		}
	}
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

func (a *App) RunHeavy(path string, n int) ([]HeavyItem, error) {
	path = a.normalizePath(path, a.folders.Home)
	if n <= 0 {
		n = 20
	}
	stats, err := scan.ScanTopFiles(path, n)
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
	prevWorkers, hadWorkers := os.LookupEnv("ICICLE_SCAN_WORKERS")
	if workers > 0 {
		_ = os.Setenv("ICICLE_SCAN_WORKERS", strconv.Itoa(workers))
	}
	stats, seen, limited, err := scan.ScanTopFilesLimited(path, n, maxFiles)
	if workers > 0 {
		if hadWorkers {
			_ = os.Setenv("ICICLE_SCAN_WORKERS", prevWorkers)
		} else {
			_ = os.Unsetenv("ICICLE_SCAN_WORKERS")
		}
	}
	a.scanMu.Unlock()
	if err != nil {
		return HeavyResult{}, err
	}
	items := make([]HeavyItem, 0, len(stats.TopFiles))
	for _, f := range stats.TopFiles {
		items = append(items, HeavyItem{Path: f.Path, Size: f.Size, Human: ui.HumanBytes(f.Size)})
	}
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

func (a *App) ExportHeavy(path string, n int, format string) (string, error) {
	path = a.normalizePath(path, a.folders.Home)
	if n <= 0 {
		n = 20
	}
	items, err := a.RunHeavy(path, n)
	if err != nil {
		return "", err
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "csv"
	}
	var filter runtime.FileFilter
	switch format {
	case "json":
		filter = runtime.FileFilter{DisplayName: "JSON", Pattern: "*.json"}
	case "md":
		filter = runtime.FileFilter{DisplayName: "Markdown", Pattern: "*.md"}
	default:
		format = "csv"
		filter = runtime.FileFilter{DisplayName: "CSV", Pattern: "*.csv"}
	}
	filename := "icicle-heavy-" + time.Now().Format("20060102-150405") + "." + format
	target, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export heavy files",
		DefaultFilename: filename,
		Filters:         []runtime.FileFilter{filter},
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(target) == "" {
		return "", nil
	}

	var body string
	switch format {
	case "json":
		b, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return "", err
		}
		body = string(b) + "\n"
	case "md":
		var b strings.Builder
		b.WriteString("| Size | Path |\n|---:|---|\n")
		for _, it := range items {
			b.WriteString("| " + it.Human + " | `" + strings.ReplaceAll(it.Path, "`", "'") + "` |\n")
		}
		body = b.String()
	default:
		var b strings.Builder
		b.WriteString("size_bytes,size_human,path\n")
		for _, it := range items {
			b.WriteString(strconv.FormatInt(it.Size, 10))
			b.WriteString(",")
			b.WriteString(csvEscape(it.Human))
			b.WriteString(",")
			b.WriteString(csvEscape(it.Path))
			b.WriteString("\n")
		}
		body = b.String()
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		return "", err
	}
	a.appendLog("[export] heavy -> " + target)
	return target, nil
}

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
	return exec.Command("explorer.exe", drive+`\`).Start()
}

func (a *App) OpenPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err != nil {
		return err
	}
	return exec.Command("explorer.exe", abs).Start()
}

func (a *App) RevealPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err != nil {
		return err
	}
	return exec.Command("explorer.exe", "/select,"+abs).Start()
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
	prevWorkers, hadWorkers := os.LookupEnv("ICICLE_SCAN_WORKERS")
	if workers > 0 {
		_ = os.Setenv("ICICLE_SCAN_WORKERS", strconv.Itoa(workers))
	}
	items, seen, limited, err := scan.ScanExtStatsLimited(path, maxFiles)
	if workers > 0 {
		if hadWorkers {
			_ = os.Setenv("ICICLE_SCAN_WORKERS", prevWorkers)
		} else {
			_ = os.Unsetenv("ICICLE_SCAN_WORKERS")
		}
	}
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
	prevWorkers, hadWorkers := os.LookupEnv("ICICLE_SCAN_WORKERS")
	if workers > 0 {
		_ = os.Setenv("ICICLE_SCAN_WORKERS", strconv.Itoa(workers))
	}
	stats, err := scan.ScanOverviewLimited(path, maxFiles, topFiles, topExt)
	if workers > 0 {
		if hadWorkers {
			_ = os.Setenv("ICICLE_SCAN_WORKERS", prevWorkers)
		} else {
			_ = os.Unsetenv("ICICLE_SCAN_WORKERS")
		}
	}
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

func (a *App) DuplicateNames(path string, maxFiles int, top int) ([]DupStat, error) {
	path = a.normalizePath(path, a.folders.Home)
	files := 0
	byName := map[string][]string{}
	err := scan.WalkAll(path, func(p string, _ int64) {
		files++
		if maxFiles > 0 && files > maxFiles {
			return
		}
		name := strings.ToLower(filepath.Base(p))
		byName[name] = append(byName[name], p)
	})
	if err != nil {
		return nil, err
	}
	out := []DupStat{}
	for name, paths := range byName {
		if len(paths) <= 1 {
			continue
		}
		out = append(out, DupStat{Name: name, Count: len(paths), Paths: paths})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if top > 0 && len(out) > top {
		out = out[:top]
	}
	return out, nil
}

func (a *App) DuplicateFinderV2(path string, mode string, maxFiles int, top int) ([]DupV2Group, error) {
	path = a.normalizePath(path, a.folders.Home)
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "quick-name"
	}
	if maxFiles <= 0 {
		maxFiles = 70000
	}
	if top <= 0 {
		top = 20
	}

	type entry struct {
		Path string
		Size int64
		Name string
	}
	files := make([]entry, 0, 4096)
	count := 0
	err := scan.WalkAll(path, func(p string, size int64) {
		count++
		if count > maxFiles {
			return
		}
		files = append(files, entry{
			Path: p,
			Size: size,
			Name: strings.ToLower(filepath.Base(p)),
		})
	})
	if err != nil {
		return nil, err
	}

	groups := map[string][]entry{}
	switch mode {
	case "hash":
		bySize := map[int64][]entry{}
		for _, f := range files {
			bySize[f.Size] = append(bySize[f.Size], f)
		}
		for size, bucket := range bySize {
			if len(bucket) < 2 {
				continue
			}
			for _, f := range bucket {
				h, herr := hashFileQuick(f.Path)
				if herr != nil {
					continue
				}
				key := fmt.Sprintf("hash:%d:%s", size, h)
				groups[key] = append(groups[key], f)
			}
		}
	default:
		for _, f := range files {
			key := "name:" + f.Name
			groups[key] = append(groups[key], f)
		}
	}

	out := make([]DupV2Group, 0, len(groups))
	for key, bucket := range groups {
		if len(bucket) < 2 {
			continue
		}
		total := int64(0)
		paths := make([]string, 0, len(bucket))
		for _, e := range bucket {
			total += e.Size
			paths = append(paths, e.Path)
		}
		out = append(out, DupV2Group{
			Key:   key,
			Count: len(bucket),
			Total: total,
			Human: ui.HumanBytes(total),
			Paths: paths,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Total > out[j].Total
		}
		return out[i].Count > out[j].Count
	})
	if len(out) > top {
		out = out[:top]
	}
	return out, nil
}

func (a *App) DuplicateKeep(paths []string, rule string, safe bool) (DuplicateActionResult, error) {
	if len(paths) < 2 {
		return DuplicateActionResult{}, fmt.Errorf("need at least 2 files in duplicate group")
	}
	type fileMeta struct {
		Path string
		Time time.Time
	}
	meta := make([]fileMeta, 0, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		meta = append(meta, fileMeta{Path: p, Time: st.ModTime()})
	}
	if len(meta) < 2 {
		return DuplicateActionResult{}, fmt.Errorf("not enough valid files")
	}
	rule = strings.ToLower(strings.TrimSpace(rule))
	if rule == "" {
		rule = "newest"
	}
	sort.Slice(meta, func(i, j int) bool { return meta[i].Time.Before(meta[j].Time) })
	keep := meta[0].Path
	if rule == "newest" {
		keep = meta[len(meta)-1].Path
	}
	toDelete := make([]string, 0, len(meta)-1)
	for _, m := range meta {
		if strings.EqualFold(m.Path, keep) {
			continue
		}
		toDelete = append(toDelete, m.Path)
	}
	br := a.BatchDelete(toDelete, safe)
	a.appendLog(fmt.Sprintf("[dupe-keep] rule=%s keep=%s deleted=%d/%d", rule, keep, br.Succeeded, br.Processed))
	return DuplicateActionResult{
		Rule:      rule,
		KeptPath:  keep,
		Deleted:   br,
		Skipped:   len(paths) - len(meta),
		GroupSize: len(paths),
	}, nil
}

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

func (a *App) ListReportSnapshots(limit int) ([]SnapshotInfo, error) {
	if limit <= 0 {
		limit = 20
	}
	dir, err := a.reportDir()
	if err != nil {
		return nil, err
	}
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]SnapshotInfo, 0, len(items))
	for _, it := range items {
		if it.IsDir() || !strings.HasSuffix(strings.ToLower(it.Name()), ".json") {
			continue
		}
		full := filepath.Join(dir, it.Name())
		info, ierr := it.Info()
		if ierr != nil {
			continue
		}
		out = append(out, SnapshotInfo{
			File: full,
			At:   info.ModTime().Unix(),
			Path: full,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At > out[j].At })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type snapshotPayload struct {
	AtUnix   int64       `json:"atUnix"`
	Path     string      `json:"path"`
	TopN     int         `json:"topN"`
	MaxFiles int         `json:"maxFiles"`
	Seen     int         `json:"seen"`
	Limited  bool        `json:"limited"`
	Items    []HeavyItem `json:"items"`
}

func (a *App) SnapshotDiff(leftFile string, rightFile string, top int) (SnapshotDiffResult, error) {
	if top <= 0 {
		top = 30
	}
	left, err := readSnapshotFile(leftFile)
	if err != nil {
		return SnapshotDiffResult{}, err
	}
	right, err := readSnapshotFile(rightFile)
	if err != nil {
		return SnapshotDiffResult{}, err
	}
	leftMap := make(map[string]int64, len(left.Items))
	rightMap := make(map[string]int64, len(right.Items))
	for _, it := range left.Items {
		leftMap[strings.ToLower(it.Path)] = it.Size
	}
	for _, it := range right.Items {
		rightMap[strings.ToLower(it.Path)] = it.Size
	}

	out := SnapshotDiffResult{
		Left:      leftFile,
		Right:     rightFile,
		CreatedAt: time.Now().Unix(),
	}
	items := make([]SnapshotDiffItem, 0, len(leftMap)+len(rightMap))
	for p, rv := range rightMap {
		lv, ok := leftMap[p]
		if !ok {
			out.Added++
			items = append(items, SnapshotDiffItem{Path: p, Delta: rv, Human: ui.HumanBytes(rv), Status: "added"})
			continue
		}
		if lv != rv {
			out.Changed++
			delta := rv - lv
			h := ui.HumanBytes(delta)
			if delta > 0 {
				h = "+" + h
			}
			items = append(items, SnapshotDiffItem{Path: p, Delta: delta, Human: h, Status: "changed"})
		}
	}
	for p, lv := range leftMap {
		if _, ok := rightMap[p]; ok {
			continue
		}
		out.Removed++
		items = append(items, SnapshotDiffItem{Path: p, Delta: -lv, Human: "-" + ui.HumanBytes(lv), Status: "removed"})
	}
	sort.Slice(items, func(i, j int) bool {
		ai := items[i].Delta
		if ai < 0 {
			ai = -ai
		}
		aj := items[j].Delta
		if aj < 0 {
			aj = -aj
		}
		return ai > aj
	})
	if len(items) > top {
		items = items[:top]
	}
	out.Top = items
	return out, nil
}

func readSnapshotFile(path string) (snapshotPayload, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return snapshotPayload{}, fmt.Errorf("snapshot path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return snapshotPayload{}, err
	}
	var p snapshotPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return snapshotPayload{}, err
	}
	return p, nil
}

func (a *App) normalizePath(path string, fallback string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		path = fallback
	}
	return filepath.Clean(path)
}

func (a *App) pushMove(from, to string) {
	if abs, err := filepath.Abs(from); err == nil {
		from = abs
	}
	if abs, err := filepath.Abs(to); err == nil {
		to = abs
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.moves = append(a.moves, moveRecord{From: from, To: to})
	if len(a.moves) > 200 {
		a.moves = a.moves[len(a.moves)-200:]
	}
}

func (a *App) peekMove() (moveRecord, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.moves) == 0 {
		return moveRecord{}, false
	}
	return a.moves[len(a.moves)-1], true
}

func (a *App) dropLastMove() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.moves) == 0 {
		return
	}
	a.moves = a.moves[:len(a.moves)-1]
}

func (a *App) loadSaved() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	data, err := os.ReadFile(a.cfgPath)
	if os.IsNotExist(err) {
		a.saved = []string{}
		return nil
	}
	if err != nil {
		return err
	}
	var in []string
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	a.saved = dedupePaths(in)
	return nil
}

func (a *App) saveSavedLocked() error {
	data, err := json.MarshalIndent(a.saved, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(a.cfgPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(a.cfgPath, data, 0o644)
}

func dedupePaths(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, p := range in {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		k := strings.ToLower(filepath.Clean(p))
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, filepath.Clean(p))
	}
	return out
}

func csvEscape(s string) string {
	s = strings.ReplaceAll(s, `"`, `""`)
	return `"` + s + `"`
}

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
		v = os.ExpandEnv(v)
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
