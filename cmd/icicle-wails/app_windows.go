//go:build windows && wails

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	return out, nil
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
		auto, ok := organize.DestinationDir(a.folders.Home, src)
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
