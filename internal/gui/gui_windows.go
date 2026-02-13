//go:build windows

package gui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/getlantern/systray"
	"icicle/internal/meta"
	"icicle/internal/organize"
	"icicle/internal/scan"
	"icicle/internal/ui"
)

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

type heavyItem struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Human string `json:"human"`
}

type guiState struct {
	mu            sync.Mutex
	watch         *exec.Cmd
	watchOn       bool
	log           bytes.Buffer
	saved         []string
	cfgPath       string
	launcherCfg   string
	gitAutoUpdate bool
	moves         []moveRecord
}

type moveRecord struct {
	From string
	To   string
}

type extStat struct {
	Ext   string `json:"ext"`
	Count int    `json:"count"`
	Size  int64  `json:"size"`
	Human string `json:"human"`
}

type dupStat struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Paths []string `json:"paths"`
}

func (s *guiState) appendLog(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log.WriteString(line)
	if !strings.HasSuffix(line, "\n") {
		s.log.WriteString("\n")
	}
	if s.log.Len() > 2*1024*1024 {
		b := s.log.Bytes()
		s.log.Reset()
		s.log.Write(b[len(b)-1024*1024:])
	}
}

func (s *guiState) getLog() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.log.String(), s.watchOn
}

func (s *guiState) clearLog() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log.Reset()
}

func (s *guiState) pushMove(from, to string) {
	if abs, err := filepath.Abs(from); err == nil {
		from = abs
	}
	if abs, err := filepath.Abs(to); err == nil {
		to = abs
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.moves = append(s.moves, moveRecord{From: from, To: to})
	if len(s.moves) > 200 {
		s.moves = s.moves[len(s.moves)-200:]
	}
}

func (s *guiState) peekMove() (moveRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.moves) == 0 {
		return moveRecord{}, false
	}
	return s.moves[len(s.moves)-1], true
}

func (s *guiState) dropLastMove() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.moves) == 0 {
		return
	}
	s.moves = s.moves[:len(s.moves)-1]
}

func (s *guiState) loadSaved() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.cfgPath)
	if os.IsNotExist(err) {
		s.saved = []string{}
		return nil
	}
	if err != nil {
		return err
	}
	var in []string
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	s.saved = dedupePaths(in)
	return nil
}

func (s *guiState) loadLauncherSettings() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gitAutoUpdate = true
	data, err := os.ReadFile(s.launcherCfg)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if strings.EqualFold(key, "ICICLE_GIT_AUTO_UPDATE") {
			s.gitAutoUpdate = val != "0"
			return nil
		}
	}
	return nil
}

func (s *guiState) saveLauncherSettings() error {
	value := "1"
	if !s.gitAutoUpdate {
		value = "0"
	}
	body := strings.Join([]string{
		"# icicle launcher settings",
		"ICICLE_GIT_AUTO_UPDATE=" + value,
		"",
	}, "\n")
	if err := os.MkdirAll(filepath.Dir(s.launcherCfg), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.launcherCfg, []byte(body), 0o644)
}

func (s *guiState) getGitAutoUpdate() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gitAutoUpdate
}

func (s *guiState) setGitAutoUpdate(v bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gitAutoUpdate = v
	return s.saveLauncherSettings()
}

func (s *guiState) saveSaved() error {
	data, err := json.MarshalIndent(s.saved, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.cfgPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.cfgPath, data, 0o644)
}

func (s *guiState) listSaved() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.saved))
	copy(out, s.saved)
	return out
}

func (s *guiState) addSaved(path string) error {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saved = dedupePaths(append([]string{abs}, s.saved...))
	return s.saveSaved()
}

func (s *guiState) removeSaved(path string) error {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make([]string, 0, len(s.saved))
	for _, p := range s.saved {
		if !strings.EqualFold(p, abs) {
			next = append(next, p)
		}
	}
	s.saved = next
	return s.saveSaved()
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

func cleanEmptyDirs(root string) (int, error) {
	root = filepath.Clean(root)
	removed := 0
	// Passes from deepest to root until stable.
	for pass := 0; pass < 3; pass++ {
		dirs := []string{}
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				dirs = append(dirs, path)
			}
			return nil
		})
		if err != nil {
			return removed, err
		}
		sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
		passRemoved := 0
		for _, dir := range dirs {
			if strings.EqualFold(filepath.Clean(dir), root) {
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
	return removed, nil
}

func extensionStats(path string, limit int) ([]extStat, error) {
	byExt := map[string]extStat{}
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
	out := make([]extStat, 0, len(byExt))
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

func duplicateNames(path string, maxFiles int, top int) ([]dupStat, error) {
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
	out := []dupStat{}
	for name, paths := range byName {
		if len(paths) <= 1 {
			continue
		}
		out = append(out, dupStat{Name: name, Count: len(paths), Paths: paths})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if top > 0 && len(out) > top {
		out = out[:top]
	}
	return out, nil
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

func (s *guiState) startWatch(appPath, path string, dry bool) error {
	s.mu.Lock()
	if s.watchOn {
		s.mu.Unlock()
		return fmt.Errorf("watch is already running")
	}
	args := []string{"watch"}
	if dry {
		args = append(args, "--dry-run")
	}
	args = append(args, path)
	cmd := exec.Command(appPath, args...)
	cmd.Dir = filepath.Dir(appPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.mu.Unlock()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.mu.Unlock()
		return err
	}
	if err := cmd.Start(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.watch = cmd
	s.watchOn = true
	s.mu.Unlock()

	s.appendLog("> icicle " + strings.Join(args, " "))
	go s.pipe(stdout)
	go s.pipe(stderr)
	go func() {
		err := cmd.Wait()
		if err != nil {
			s.appendLog("[watch stopped] " + err.Error())
		} else {
			s.appendLog("[watch stopped]")
		}
		s.mu.Lock()
		s.watch = nil
		s.watchOn = false
		s.mu.Unlock()
	}()
	return nil
}

func (s *guiState) stopWatch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.watch != nil && s.watch.Process != nil {
		_ = s.watch.Process.Kill()
	}
}

func (s *guiState) pipe(r io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			s.appendLog(string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}

func Run(appPath string) error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	folders := detectUserFolders()
	home := folders.Home
	downloads := folders.Downloads
	desktop := folders.Desktop
	documents := folders.Documents
	cfgDir, _ := os.UserConfigDir()
	if cfgDir == "" {
		cfgDir = home
	}
	cfgPath := filepath.Join(cfgDir, "icicle", "saved_folders.json")
	launcherCfg := filepath.Join(cfgDir, "icicle", "launcher.env")

	state := &guiState{cfgPath: cfgPath, launcherCfg: launcherCfg, gitAutoUpdate: true}
	_ = state.loadSaved()
	_ = state.loadLauncherSettings()
	quit := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(quit) }) }

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, pageHTML)
	})
	mux.HandleFunc("/api/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Command string `json:"command"`
			Path    string `json:"path"`
			TopN    string `json:"topN"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		path := strings.TrimSpace(req.Path)
		if path == "" {
			path = "."
		}
		args := []string{}
		switch req.Command {
		case "tree":
			args = []string{"tree", path}
		case "heavy":
			n := strings.TrimSpace(req.TopN)
			if n == "" {
				n = "20"
			}
			args = []string{"heavy", "--n", n, path}
		case "help":
			args = []string{"help"}
		default:
			http.Error(w, "unknown command", http.StatusBadRequest)
			return
		}
		cmd := exec.Command(appPath, args...)
		cmd.Dir = filepath.Dir(appPath)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		text := "> icicle " + strings.Join(args, " ") + "\n" + out.String()
		state.appendLog(text)
		resp := map[string]string{"status": "ok"}
		if err != nil {
			resp["error"] = err.Error()
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/api/watch/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Path string `json:"path"`
			Dry  bool   `json:"dry"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		path := strings.TrimSpace(req.Path)
		if path == "" {
			path = downloads
		}
		if err := state.startWatch(appPath, path, req.Dry); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/watch/stop", func(w http.ResponseWriter, r *http.Request) {
		state.stopWatch()
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/watch/log", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		logText, running := state.getLog()
		_ = json.NewEncoder(w).Encode(map[string]any{"log": logText, "running": running})
	})
	mux.HandleFunc("/api/log/clear", func(w http.ResponseWriter, r *http.Request) {
		state.clearLog()
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/exit", func(w http.ResponseWriter, r *http.Request) {
		state.stopWatch()
		stop()
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/defaults", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"home":          home,
			"downloads":     downloads,
			"desktop":       desktop,
			"documents":     documents,
			"build":         meta.Version,
			"repo":          updateRepo(),
			"gitAutoUpdate": state.getGitAutoUpdate(),
		})
	})
	mux.HandleFunc("/api/settings/git-auto-update", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"enabled": state.getGitAutoUpdate()})
		case http.MethodPost:
			var req struct {
				Enabled bool `json:"enabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := state.setGitAutoUpdate(req.Enabled); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			state.appendLog(fmt.Sprintf("[settings] git auto-update = %v", req.Enabled))
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "enabled": req.Enabled})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/update/check", func(w http.ResponseWriter, r *http.Request) {
		info, err := checkForUpdate(meta.Version)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(info)
	})
	mux.HandleFunc("/api/update/apply", func(w http.ResponseWriter, r *http.Request) {
		info, newExe, err := prepareUpdateBinary(appPath, meta.Version)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if !info.HasUpdate {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":  "uptodate",
				"message": "already on latest version",
				"current": info.Current,
				"latest":  info.Latest,
			})
			return
		}
		if err := launchSwapScript(appPath, newExe, os.Getpid()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		state.appendLog("[update] downloaded " + info.Latest + " -> restarting")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "restarting",
			"message": "update downloaded, app will restart",
			"latest":  info.Latest,
		})
		go func() {
			time.Sleep(300 * time.Millisecond)
			state.stopWatch()
			stop()
		}()
	})
	mux.HandleFunc("/api/system/storage", func(w http.ResponseWriter, r *http.Request) {
		volumes, err := SystemStorage()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		type volumeOut struct {
			Drive      string  `json:"drive"`
			Total      int64   `json:"total"`
			Free       int64   `json:"free"`
			Used       int64   `json:"used"`
			UsedHuman  string  `json:"usedHuman"`
			TotalHuman string  `json:"totalHuman"`
			UsedRatio  float64 `json:"usedRatio"`
		}
		out := make([]volumeOut, 0, len(volumes))
		for _, v := range volumes {
			ratio := 0.0
			if v.Total > 0 {
				ratio = float64(v.Used) / float64(v.Total)
			}
			out = append(out, volumeOut{
				Drive:      v.Drive,
				Total:      v.Total,
				Free:       v.Free,
				Used:       v.Used,
				UsedHuman:  ui.HumanBytes(v.Used),
				TotalHuman: ui.HumanBytes(v.Total),
				UsedRatio:  ratio,
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": out})
	})
	mux.HandleFunc("/api/system/open-drive", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Drive string `json:"drive"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		drive := normalizeDrive(req.Drive)
		if drive == "" {
			http.Error(w, "invalid drive", http.StatusBadRequest)
			return
		}
		if err := exec.Command("explorer.exe", drive+`\`).Start(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/path/open", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		path := strings.TrimSpace(req.Path)
		if path == "" {
			http.Error(w, "path is required", http.StatusBadRequest)
			return
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := os.Stat(abs); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := exec.Command("explorer.exe", abs).Start(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/path/reveal", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		path := strings.TrimSpace(req.Path)
		if path == "" {
			http.Error(w, "path is required", http.StatusBadRequest)
			return
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := os.Stat(abs); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cmd := exec.Command("explorer.exe", "/select,"+abs)
		if err := cmd.Start(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/path/clean-empty", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		path := strings.TrimSpace(req.Path)
		if path == "" {
			path = home
		}
		removed, err := cleanEmptyDirs(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		state.appendLog(fmt.Sprintf("[clean-empty] %s removed=%d", path, removed))
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "removed": removed})
	})
	mux.HandleFunc("/api/path/extensions", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		path := strings.TrimSpace(req.Path)
		if path == "" {
			path = home
		}
		stats, err := extensionStats(path, 20)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": stats})
	})
	mux.HandleFunc("/api/path/duplicates", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		path := strings.TrimSpace(req.Path)
		if path == "" {
			path = home
		}
		stats, err := duplicateNames(path, 70000, 20)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": stats})
	})
	mux.HandleFunc("/api/folders/list", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"items": state.listSaved()})
	})
	mux.HandleFunc("/api/folders/add", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		path := strings.TrimSpace(req.Path)
		if path == "" {
			http.Error(w, "path is required", http.StatusBadRequest)
			return
		}
		if err := state.addSaved(path); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/folders/remove", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := state.removeSaved(req.Path); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/folder/hint", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		path := strings.TrimSpace(req.Path)
		if path == "" {
			path = downloads
		}
		kind := detectFolderKind(path)
		_ = json.NewEncoder(w).Encode(map[string]string{"kind": kind})
	})
	mux.HandleFunc("/api/heavy", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
			TopN string `json:"topN"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		path := strings.TrimSpace(req.Path)
		if path == "" {
			path = home
		}
		n := 20
		if req.TopN != "" {
			if parsed, err := strconv.Atoi(req.TopN); err == nil && parsed > 0 && parsed <= 200 {
				n = parsed
			}
		}
		stats, err := scan.ScanTopFiles(path, n)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		items := make([]heavyItem, 0, len(stats.TopFiles))
		for _, f := range stats.TopFiles {
			items = append(items, heavyItem{Path: f.Path, Size: f.Size, Human: ui.HumanBytes(f.Size)})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Size > items[j].Size })
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
	})
	mux.HandleFunc("/api/file/delete", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		path := strings.TrimSpace(req.Path)
		info, err := os.Stat(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if info.IsDir() {
			http.Error(w, "refusing to delete directory", http.StatusBadRequest)
			return
		}
		if err := os.Remove(path); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		state.appendLog("[delete] " + path)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/file/move", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
			Dest string `json:"dest"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		src := strings.TrimSpace(req.Path)
		if src == "" {
			http.Error(w, "path is required", http.StatusBadRequest)
			return
		}
		destDir := strings.TrimSpace(req.Dest)
		if destDir == "" {
			auto, ok := organize.DestinationDir(home, src)
			if !ok {
				http.Error(w, "no auto destination for this extension", http.StatusBadRequest)
				return
			}
			destDir = auto
		}
		dst := filepath.Join(destDir, filepath.Base(src))
		uniqueDst, err := organize.EnsureUniquePath(dst)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := organize.MoveFile(src, uniqueDst); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		state.pushMove(src, uniqueDst)
		state.appendLog("[move] " + src + " -> " + uniqueDst)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "dest": uniqueDst})
	})
	mux.HandleFunc("/api/file/undo-move", func(w http.ResponseWriter, r *http.Request) {
		rec, ok := state.peekMove()
		if !ok {
			http.Error(w, "no move history", http.StatusBadRequest)
			return
		}
		if _, err := os.Stat(rec.To); err != nil {
			http.Error(w, "moved file no longer exists: "+rec.To, http.StatusBadRequest)
			return
		}
		target := rec.From
		uniqueTarget, err := organize.EnsureUniquePath(target)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := organize.MoveFile(rec.To, uniqueTarget); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		state.dropLastMove()
		state.appendLog("[undo-move] " + rec.To + " -> " + uniqueTarget)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "path": uniqueTarget})
	})

	srv := &http.Server{Handler: mux}
	go func() {
		_ = srv.Serve(ln)
	}()

	url := "http://" + ln.Addr().String()
	if err := openBrowser(url); err != nil {
		_ = srv.Shutdown(context.Background())
		return err
	}
	quitTray := make(chan struct{})
	go func() {
		systray.Run(func() {
			systray.SetIcon(trayIcon)
			systray.SetTooltip("icicle")
			systray.SetTitle("icicle")
			openItem := systray.AddMenuItem("Open GUI", "Open icicle interface")
			openItem.SetIcon(trayIcon)
			systray.AddSeparator()
			exitItem := systray.AddMenuItem("Exit", "Stop icicle")
			exitItem.SetIcon(trayIcon)
			go func() {
				for {
					select {
					case <-openItem.ClickedCh:
						_ = openBrowser(url)
					case <-exitItem.ClickedCh:
						state.stopWatch()
						stop()
						systray.Quit()
						return
					case <-quit:
						systray.Quit()
						return
					}
				}
			}()
		}, func() {
			close(quitTray)
		})
	}()

	<-quit
	select {
	case <-quitTray:
	case <-time.After(2 * time.Second):
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

func openBrowser(url string) error {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	return cmd.Start()
}

const pageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>icicle</title>
<style>
:root{
  --bg:#faf7ef;--ink:#1f2328;--muted:#6d707a;--card:#ffffff;--line:#d9d2c4;--surface:#f7f3e9;
  --cold:#a34d00;--hot:#bf1e2e;--accent:#0063a3;--shadow:0 12px 28px rgba(45,39,25,.12);
}
body[data-theme="dark"]{
  --bg:#12100d;--ink:#f7f2e8;--muted:#b5a896;--card:#1b1814;--line:#383026;--surface:#17140f;
  --cold:#ff9f4a;--hot:#ff6a7e;--accent:#69b6ff;--shadow:0 14px 34px rgba(0,0,0,.4);
}
*{box-sizing:border-box}
body{
  margin:0;color:var(--ink);font-family:"Segoe UI",Arial,sans-serif;
  background:
  repeating-linear-gradient(-32deg,rgba(0,0,0,.015) 0 16px,transparent 16px 34px),
  radial-gradient(1200px 700px at 75% -30%,rgba(163,77,0,.18),transparent 60%),
  radial-gradient(900px 500px at -10% 110%,rgba(0,99,163,.14),transparent 60%),
  var(--bg);
}
.wrap{max-width:1240px;margin:0 auto;padding:14px}
.shell{display:grid;grid-template-columns:280px 1fr;gap:12px;min-height:calc(100vh - 28px)}
.side{
  background:var(--card);border:1px solid var(--line);border-radius:18px;padding:16px;box-shadow:var(--shadow);
  display:flex;flex-direction:column;gap:10px;position:sticky;top:14px;height:fit-content
}
.brand{font-size:54px;line-height:1;font-weight:900;letter-spacing:-1px}
.sub{color:var(--muted);font-size:13px}
.controlsTop{display:flex;flex-wrap:wrap;gap:8px}
.pill{display:inline-block;padding:4px 10px;border-radius:999px;border:1px solid var(--line);background:var(--surface);font-size:11px;font-weight:800}
.main{display:flex;flex-direction:column;gap:12px}
.panel{
  background:var(--card);border:1px solid var(--line);border-radius:18px;padding:14px;box-shadow:var(--shadow)
}
.row{display:flex;gap:8px;flex-wrap:wrap;margin-bottom:8px;align-items:center}
.grid{display:grid;grid-template-columns:2fr 1fr;gap:12px}
input,button,select{border-radius:8px;border:1px solid var(--line);padding:10px 12px;font-size:14px;background:var(--surface);color:var(--ink)}
input{flex:1;min-width:200px}
button{cursor:pointer;white-space:nowrap;transition:all .14s ease}
button:hover{transform:translateY(-1px)}
button.primary{background:var(--cold);color:#fff;border-color:transparent}
button.warn{background:var(--hot);color:#fff;border-color:transparent}
button.ghost{background:transparent}
label{font-size:11px;letter-spacing:.7px;text-transform:uppercase;color:var(--muted);font-weight:700}
.folderTag{display:inline-block;padding:4px 10px;border:1px dashed var(--line);border-radius:999px;font-size:11px;font-weight:800}
.storageGrid{display:grid;grid-template-columns:repeat(auto-fit,minmax(210px,1fr));gap:10px}
.storageCard{background:var(--surface);border:1px solid var(--line);border-radius:10px;padding:10px}
.bar{height:7px;border-radius:999px;background:rgba(0,0,0,.12);overflow:hidden;margin-top:6px}
.barFill{height:100%;background:linear-gradient(90deg,var(--accent),var(--cold))}
.heavy{width:100%;border-collapse:collapse;font-size:13px}
.heavy td,.heavy th{padding:8px;border-bottom:1px solid var(--line);vertical-align:top}
.heavy th{text-align:left;color:var(--muted)}
.pathCell{max-width:560px;word-break:break-all}
.mono{font-family:Consolas,monospace}
pre{margin:0;background:#050404;color:#f9ecd2;padding:14px;border-radius:10px;min-height:260px;max-height:46vh;overflow:auto;font-family:Consolas,monospace}

.konami{position:fixed;inset:0;display:none;align-items:center;justify-content:center;z-index:9999;
background:radial-gradient(circle at center,rgba(255,159,74,.32),rgba(9,7,4,.93));backdrop-filter:blur(5px)}
.konami.show{display:flex;animation:fadeIn .2s ease}
.konamiCard{position:relative;overflow:hidden;min-width:360px;background:var(--card);border:1px solid var(--line);border-radius:20px;padding:28px 30px;text-align:center;box-shadow:0 24px 80px rgba(0,0,0,.4)}
.konamiBurst{position:absolute;inset:0;pointer-events:none}
.konamiParticle{position:absolute;width:10px;height:10px;border-radius:50%;background:linear-gradient(180deg,var(--accent),var(--cold));animation:burst 1.4s ease-out forwards}
.konamiTitle{font-size:34px;font-weight:900;margin-bottom:8px;animation:pulse 1s ease-in-out infinite}
.konamiSub{color:var(--muted);font-size:15px}
.konamiGlow{position:absolute;left:50%;top:50%;width:520px;height:520px;transform:translate(-50%,-50%);background:radial-gradient(circle,rgba(255,159,74,.34),transparent 60%);filter:blur(10px);animation:spin 2.4s linear infinite}
.shake{animation:shake .35s linear 3}
@keyframes fadeIn{from{opacity:0}to{opacity:1}}
@keyframes pulse{0%,100%{transform:scale(1)}50%{transform:scale(1.04)}}
@keyframes burst{0%{transform:translate(0,0) scale(1);opacity:1}100%{transform:translate(var(--dx),var(--dy)) scale(.2);opacity:0}}
@keyframes spin{from{transform:translate(-50%,-50%) rotate(0deg)}to{transform:translate(-50%,-50%) rotate(360deg)}}
@keyframes shake{0%,100%{transform:translateX(0)}20%{transform:translateX(-8px)}40%{transform:translateX(8px)}60%{transform:translateX(-6px)}80%{transform:translateX(6px)}}
@media (max-width:980px){.shell{grid-template-columns:1fr}.side{position:static}.grid{grid-template-columns:1fr}}
</style>
</head>
<body data-theme="light">
<div class="wrap">
  <div class="shell">
    <aside class="side">
      <div class="brand">icicle</div>
      <div class="sub">Windows release</div>
      <div class="controlsTop">
        <span class="pill">RELEASE</span>
        <button class="ghost" id="updateBtn" onclick="handleUpdateClick()" data-i18n="checkUpdate">Check Update</button>
        <button class="ghost" id="gitUpdateBtn" onclick="toggleGitAutoUpdate()" data-i18n="gitAutoOn">Git Auto: ON</button>
        <button class="ghost" id="themeBtn" onclick="toggleTheme()">Dark</button>
        <button class="ghost" id="langBtn" onclick="toggleLang()">RU</button>
      </div>
      <div class="panel" style="margin:0">
        <div class="row" style="justify-content:space-between">
          <label data-i18n="systemStorage">System Storage</label>
          <button onclick="loadStorage()" data-i18n="refreshStorage">Refresh Storage</button>
        </div>
        <div class="row"><span class="pill" id="driveSelected">-</span></div>
        <div id="storageGrid" class="storageGrid"></div>
      </div>
    </aside>

    <main class="main">
      <section class="panel">
        <div class="grid">
          <div>
            <div class="row"><label data-i18n="pathLabel">Path</label></div>
            <div class="row"><input id="path" placeholder="C:\\Users\\you\\Downloads"/><span id="folderKind" class="folderTag" data-i18n="unknown">Unknown</span></div>
            <div class="row">
              <button onclick="setPathQuick('home')" data-i18n="qHome">Home</button>
              <button onclick="setPathQuick('downloads')" data-i18n="qDownloads">Downloads</button>
              <button onclick="setPathQuick('desktop')" data-i18n="qDesktop">Desktop</button>
              <button onclick="setPathQuick('documents')" data-i18n="qDocuments">Documents</button>
              <button onclick="openCurrentPath()" data-i18n="openPath">Open Path</button>
              <button onclick="analyzeCurrentPath()" data-i18n="analyzePath">Analyze</button>
            </div>
          </div>
          <div>
            <div class="row"><label data-i18n="topNLabel">Top N</label></div>
            <div class="row"><input id="topN" value="20" style="max-width:120px"/></div>
          </div>
        </div>
        <div class="row">
          <select id="savedFolders" style="min-width:280px"></select>
          <button onclick="refreshFolders()" data-i18n="refreshFolders">Refresh Folders</button>
          <button onclick="saveCurrentFolder()" data-i18n="saveCurrent">Save Current</button>
          <button onclick="useSelectedFolder()" data-i18n="useSelected">Use Selected</button>
          <button class="warn" onclick="removeSelectedFolder()" data-i18n="removeSelected">Remove Selected</button>
        </div>
        <div class="row">
          <button class="primary" onclick="runCmd('tree')" data-i18n="runTree">Run Tree</button>
          <button class="primary" onclick="runCmd('heavy')" data-i18n="runHeavy">Run Heavy</button>
          <button id="heavyToggleBtn" onclick="toggleHeavyPanel()">Heavy Actions</button>
          <button onclick="undoMove()" data-i18n="undoMove">Undo Move</button>
          <button onclick="runCmd('help')" data-i18n="help">Help</button>
        </div>
        <div class="row">
          <button class="primary" onclick="startWatch()" data-i18n="startWatch">Start Watch</button>
          <button class="warn" onclick="stopWatch()" data-i18n="stopWatch">Stop Watch</button>
          <label><input id="dry" type="checkbox"/> <span data-i18n="dryRun">dry-run</span></label>
          <button onclick="clearLog()" data-i18n="clearLog">Clear Log</button>
          <button onclick="exitApp()" data-i18n="exitApp">Exit App</button>
        </div>
      </section>

      <section class="panel" id="heavyPanel" style="display:none">
        <div class="row">
          <label data-i18n="quickMove">Quick Move Destination</label>
          <input id="moveDest" placeholder="C:\\Users\\you\\Desktop\\temp (optional)"/>
        </div>
        <div class="row">
          <button id="autoRefreshBtn" onclick="toggleAutoRefresh()" data-i18n="autoRefreshOff">Auto Refresh: Off</button>
          <button onclick="saveHeavySnapshot()" data-i18n="saveSnapshot">Save Snapshot</button>
          <button onclick="compareHeavySnapshot()" data-i18n="compareSnapshot">Compare Snapshot</button>
          <button onclick="cleanEmptyFolders()" data-i18n="cleanEmpty">Clean Empty Folders</button>
          <button onclick="analyzeExtensions()" data-i18n="extBreakdown">Extension Breakdown</button>
          <button onclick="findDuplicates()" data-i18n="findDupes">Find Duplicates</button>
          <button onclick="exportReportMD()" data-i18n="exportReport">Export Report</button>
        </div>
        <div class="row">
          <input id="heavySearch" placeholder="filter by name/path"/>
          <input id="minSizeMB" type="number" min="0" step="1" value="0" style="max-width:120px" title="Min size MB"/>
          <select id="sortMode" style="max-width:200px">
            <option value="size_desc">size desc</option>
            <option value="size_asc">size asc</option>
            <option value="name_asc">name a-z</option>
            <option value="name_desc">name z-a</option>
          </select>
          <button onclick="applyHeavyFilter()" data-i18n="applyFilter">Apply Filter</button>
          <button onclick="clearHeavyFilter()" data-i18n="clearFilter">Clear Filter</button>
          <button onclick="refreshHeavy()" data-i18n="refreshHeavy">Refresh Heavy</button>
        </div>
        <div class="row">
          <button onclick="selectAllVisible()" data-i18n="selectAll">Select All</button>
          <button onclick="clearSelection()" data-i18n="clearSelection">Clear Selection</button>
          <button onclick="bulkAutoMove()" data-i18n="bulkAutoMove">Bulk Auto Move</button>
          <button onclick="bulkMoveCustom()" data-i18n="bulkMoveTo">Bulk Move To</button>
          <button class="warn" onclick="bulkDelete()" data-i18n="bulkDelete">Bulk Delete</button>
          <button onclick="exportHeavyCSV()" data-i18n="exportCSV">Export CSV</button>
          <button onclick="exportHeavyJSON()" data-i18n="exportJSON">Export JSON</button>
          <button onclick="copySelectedPaths()" data-i18n="copyPaths">Copy Paths</button>
          <span class="pill" id="selectionInfo">selected: 0</span>
        </div>
        <table class="heavy">
          <thead><tr><th>Sel</th><th data-i18n="size">Size</th><th data-i18n="file">File</th><th data-i18n="actions">Actions</th></tr></thead>
          <tbody id="heavyBody"><tr><td colspan="4" data-i18n="noHeavy">No heavy list loaded.</td></tr></tbody>
        </table>
      </section>

      <section class="panel"><pre id="log">icicle GUI ready
</pre></section>
    </main>
  </div>
</div>
<div id="konami" class="konami" onclick="hideKonami()">
  <div class="konamiCard">
    <div class="konamiGlow"></div>
    <div class="konamiBurst" id="konamiBurst"></div>
    <div class="konamiTitle" id="konamiTitle">Konami Unlocked</div>
    <div class="konamiSub" id="konamiSub">Congrats, you are old school.</div>
  </div>
</div>
<script>
const I18N = {
  en: {
    pathLabel: 'Path', topNLabel: 'Top N',
    refreshFolders: 'Refresh Folders', saveCurrent: 'Save Current', useSelected: 'Use Selected', removeSelected: 'Remove Selected',
    runTree: 'Run Tree', runHeavy: 'Run Heavy', heavyActions: 'Heavy Actions', help: 'Help',
    showHeavyActions: 'Show File Actions', hideHeavyActions: 'Hide File Actions',
    undoMove: 'Undo Move',
    startWatch: 'Start Watch', stopWatch: 'Stop Watch', dryRun: 'dry-run', clearLog: 'Clear Log', exitApp: 'Exit App',
    quickMove: 'Quick Move Destination', size: 'Size', file: 'File', actions: 'Actions',
    systemStorage: 'System Storage', refreshStorage: 'Refresh Storage',
    diskUsePath: 'Use Path', diskTree: 'Tree', diskHeavy: 'Heavy', diskOpen: 'Open',
    diskSelected: 'Selected Disk',
    qHome: 'Home', qDownloads: 'Downloads', qDesktop: 'Desktop', qDocuments: 'Documents', openPath: 'Open Path', analyzePath: 'Analyze',
    applyFilter: 'Apply Filter', clearFilter: 'Clear Filter', refreshHeavy: 'Refresh Heavy',
    selectAll: 'Select All', clearSelection: 'Clear Selection',
    bulkAutoMove: 'Bulk Auto Move', bulkMoveTo: 'Bulk Move To', bulkDelete: 'Bulk Delete',
    exportCSV: 'Export CSV', exportJSON: 'Export JSON', copyPaths: 'Copy Paths',
    autoRefreshOff: 'Auto Refresh: Off', autoRefreshOn: 'Auto Refresh: On',
    saveSnapshot: 'Save Snapshot', compareSnapshot: 'Compare Snapshot',
    cleanEmpty: 'Clean Empty Folders', extBreakdown: 'Extension Breakdown', findDupes: 'Find Duplicates', exportReport: 'Export Report',
    reveal: 'Reveal',
    autoMove: 'Auto Move', moveTo: 'Move To', delete: 'Delete',
    noHeavy: 'No heavy list loaded.', noSaved: '(no saved folders)',
    unknown: 'Unknown',
    errPathEmpty: '[error] path is empty\n', errMoveEmpty: '[error] move destination is empty\n',
    confirmDelete: 'Delete file permanently?\n',
    checkUpdate: 'Check Update', installUpdate: 'Install Update',
    updateAvailable: '[update] available version ', updateLatest: '[update] already latest\n',
    updateApplying: '[update] applying update and restarting...\n',
    gitAutoOn: 'Git Auto: ON', gitAutoOff: 'Git Auto: OFF',
    gitAutoChanged: '[settings] git auto-update: ',
    konamiBtn: 'Konami Test', konamiTitle: 'Konami Unlocked', konamiSub: 'Congrats, you are old school.'
  },
  ru: {
    pathLabel: 'Путь', topNLabel: 'Топ N',
    refreshFolders: 'Обновить папки', saveCurrent: 'Сохранить текущую', useSelected: 'Использовать', removeSelected: 'Удалить из списка',
    runTree: 'Построить дерево', runHeavy: 'Тяжёлые файлы', heavyActions: 'Действия по файлам', help: 'Справка',
    showHeavyActions: 'Показать действия', hideHeavyActions: 'Скрыть действия',
    undoMove: 'Отменить перенос',
    startWatch: 'Старт слежения', stopWatch: 'Стоп слежения', dryRun: 'тестовый режим', clearLog: 'Очистить лог', exitApp: 'Выйти',
    quickMove: 'Быстрый перенос в', size: 'Размер', file: 'Файл', actions: 'Действия',
    systemStorage: 'Место на дисках', refreshStorage: 'Обновить диски',
    diskUsePath: 'В путь', diskTree: 'Дерево', diskHeavy: 'Тяжёлые', diskOpen: 'Открыть',
    diskSelected: 'Выбранный диск',
    qHome: 'Домой', qDownloads: 'Загрузки', qDesktop: 'Рабочий стол', qDocuments: 'Документы', openPath: 'Открыть путь', analyzePath: 'Анализ',
    applyFilter: 'Применить фильтр', clearFilter: 'Сбросить фильтр', refreshHeavy: 'Обновить тяжёлые',
    selectAll: 'Выбрать все', clearSelection: 'Сброс выбора',
    bulkAutoMove: 'Массовый авто перенос', bulkMoveTo: 'Массовый перенос в', bulkDelete: 'Массовое удаление',
    exportCSV: 'Экспорт CSV', exportJSON: 'Экспорт JSON', copyPaths: 'Копировать пути',
    autoRefreshOff: 'Автообновление: Выкл', autoRefreshOn: 'Автообновление: Вкл',
    saveSnapshot: 'Сохранить снимок', compareSnapshot: 'Сравнить снимок',
    cleanEmpty: 'Очистить пустые папки', extBreakdown: 'Разбор расширений', findDupes: 'Найти дубли', exportReport: 'Экспорт отчёта',
    reveal: 'Показать',
    autoMove: 'Авто перенос', moveTo: 'Перенести в', delete: 'Удалить',
    noHeavy: 'Список ещё не загружен.', noSaved: '(нет сохранённых папок)',
    unknown: 'Неизвестно',
    errPathEmpty: '[ошибка] путь пустой\n', errMoveEmpty: '[ошибка] путь назначения пустой\n',
    confirmDelete: 'Удалить файл навсегда?\n',
    checkUpdate: 'Проверить обновление', installUpdate: 'Установить обновление',
    updateAvailable: '[обновление] доступна версия ', updateLatest: '[обновление] уже последняя версия\n',
    updateApplying: '[обновление] установка и перезапуск...\n',
    gitAutoOn: 'Git авто: ВКЛ', gitAutoOff: 'Git авто: ВЫКЛ',
    gitAutoChanged: '[настройки] git автообновление: ',
    konamiBtn: 'Тест Konami', konamiTitle: 'Konami активирован', konamiSub: 'Поздравляем, ты олд.'
  }
};
const logEl = document.getElementById('log');
const pathEl = document.getElementById('path');
const folderKindEl = document.getElementById('folderKind');
const savedEl = document.getElementById('savedFolders');
const heavyBody = document.getElementById('heavyBody');
const heavyPanel = document.getElementById('heavyPanel');
const heavyToggleBtn = document.getElementById('heavyToggleBtn');
const autoRefreshBtn = document.getElementById('autoRefreshBtn');
const heavySearchEl = document.getElementById('heavySearch');
const minSizeMBEl = document.getElementById('minSizeMB');
const sortModeEl = document.getElementById('sortMode');
const selectionInfoEl = document.getElementById('selectionInfo');
const storageGrid = document.getElementById('storageGrid');
const konamiEl = document.getElementById('konami');
const konamiBurst = document.getElementById('konamiBurst');
const konamiBtn = document.getElementById('konamiBtn');
const driveSelectedEl = document.getElementById('driveSelected');
const themeBtn = document.getElementById('themeBtn');
const langBtn = document.getElementById('langBtn');
const updateBtn = document.getElementById('updateBtn');
const gitUpdateBtn = document.getElementById('gitUpdateBtn');
let lang = localStorage.getItem('icicle_lang') || 'en';
let theme = localStorage.getItem('icicle_theme') || 'light';
let lastLogLen = logEl.textContent.length;
let hintTimer = null;
let lastHeavyItems = [];
let heavyViewItems = [];
let heavyOpen = false;
let selectedDrive = '';
const selectedPaths = new Set();
let autoRefreshHeavy = false;
let autoRefreshTimer = null;
let heavySnapshot = [];
let pendingUpdate = null;
let gitAutoUpdate = true;
let userDefaults = { home:'', downloads:'', desktop:'', documents:'' };
const konamiSeq = ['ArrowUp','ArrowUp','ArrowDown','ArrowDown','ArrowLeft','ArrowRight','ArrowLeft','ArrowRight','b','a'];
let konamiPos = 0;
function t(k){ return (I18N[lang] && I18N[lang][k]) || (I18N.en && I18N.en[k]) || k; }
function applyTheme(){ document.body.setAttribute('data-theme', theme); themeBtn.textContent = theme === 'light' ? 'Dark' : 'Light'; }
function toggleTheme(){ theme = theme === 'light' ? 'dark' : 'light'; localStorage.setItem('icicle_theme', theme); applyTheme(); }
function applyLang(){
  document.documentElement.lang = lang === 'ru' ? 'ru' : 'en';
  const nodes = document.querySelectorAll('[data-i18n]');
  for(const n of nodes){ n.textContent = t(n.getAttribute('data-i18n')); }
  if(konamiBtn){ konamiBtn.textContent = t('konamiBtn'); }
  updateHeavyToggleLabel();
  updateAutoRefreshLabel();
  updateSelectionInfo();
  updateDriveSelectedPill();
  updateUpdateButton();
  updateGitAutoButton();
  langBtn.textContent = lang === 'ru' ? 'EN' : 'RU';
}
function updateUpdateButton(){
  if(!updateBtn){ return; }
  if(pendingUpdate && pendingUpdate.latest){
    updateBtn.textContent = t('installUpdate') + ' ' + pendingUpdate.latest;
  } else {
    updateBtn.textContent = t('checkUpdate');
  }
}
function updateGitAutoButton(){
  if(!gitUpdateBtn){ return; }
  gitUpdateBtn.textContent = gitAutoUpdate ? t('gitAutoOn') : t('gitAutoOff');
}
async function checkUpdate(silent){
  const r = await fetch('/api/update/check');
  if(!r.ok){
    if(!silent){ append('[error] '+await r.text()+'\n'); }
    return;
  }
  const d = await r.json();
  if(d.hasUpdate){
    pendingUpdate = d;
    updateUpdateButton();
    if(!silent){ append(t('updateAvailable') + d.latest + '\n'); }
    return;
  }
  pendingUpdate = null;
  updateUpdateButton();
  if(!silent){ append(t('updateLatest')); }
}
async function toggleGitAutoUpdate(){
  gitAutoUpdate = !gitAutoUpdate;
  const r = await fetch('/api/settings/git-auto-update',{
    method:'POST',
    headers:{'content-type':'application/json'},
    body:JSON.stringify({enabled:gitAutoUpdate}),
  });
  if(!r.ok){
    append('[error] '+await r.text()+'\n');
    gitAutoUpdate = !gitAutoUpdate;
  } else {
    append(t('gitAutoChanged') + (gitAutoUpdate ? 'ON' : 'OFF') + '\n');
  }
  updateGitAutoButton();
}
async function applyUpdate(){
  const r = await fetch('/api/update/apply',{method:'POST'});
  if(!r.ok){
    append('[error] '+await r.text()+'\n');
    return;
  }
  const d = await r.json();
  if(d.status === 'uptodate'){
    pendingUpdate = null;
    updateUpdateButton();
    append(t('updateLatest'));
    return;
  }
  append(t('updateApplying'));
}
async function handleUpdateClick(){
  if(pendingUpdate && pendingUpdate.hasUpdate){
    await applyUpdate();
    return;
  }
  await checkUpdate(false);
}
async function toggleLang(){
  lang = lang === 'ru' ? 'en' : 'ru';
  localStorage.setItem('icicle_lang', lang);
  applyLang();
  renderHeavy(lastHeavyItems);
  try{
    await refreshFolders();
    await loadStorage();
    await updateFolderHint();
  }catch(e){
    append('[js error] '+(e && e.message ? e.message : String(e))+'\n');
  }
}
function append(t){logEl.textContent += t; logEl.scrollTop = logEl.scrollHeight;}
async function clearLog(){ await fetch('/api/log/clear',{method:'POST'}); logEl.textContent=''; lastLogLen = 0; }
async function setDefaults(){
  try{
    const r = await fetch('/api/defaults');
    const d = await r.json();
    userDefaults = {
      home: d.home || '',
      downloads: d.downloads || '',
      desktop: d.desktop || '',
      documents: d.documents || '',
    };
    if(!pathEl.value.trim()){ pathEl.value = d.downloads || d.home || ''; }
    if(typeof d.gitAutoUpdate === 'boolean'){ gitAutoUpdate = d.gitAutoUpdate; }
    selectedDrive = normDrive(pathEl.value);
    updateDriveSelectedPill();
    updateGitAutoButton();
  }catch(_){}
  await refreshFolders();
  await updateFolderHint();
  setHeavyOpen(false, false);
  await loadStorage();
  await checkUpdate(true);
}
async function runCmd(command){
  const path = pathEl.value.trim();
  const topN = document.getElementById('topN').value.trim();
  const r = await fetch('/api/run',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({command,path,topN})});
  const data = await r.json();
  if(data.error) append('\n[error] '+data.error+'\n');
  await poll();
}
async function startWatch(){
  const path = pathEl.value.trim();
  const dry = document.getElementById('dry').checked;
  const r = await fetch('/api/watch/start',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({path,dry})});
  if(!r.ok) append('[error] '+await r.text()+'\n');
}
async function stopWatch(){ await fetch('/api/watch/stop',{method:'POST'}); }
async function exitApp(){ await fetch('/api/exit',{method:'POST'}); window.close(); }
async function refreshFolders(){
  const r = await fetch('/api/folders/list');
  const d = await r.json();
  savedEl.innerHTML = '';
  const items = d.items || [];
  if(items.length === 0){
    const opt = document.createElement('option');
    opt.value = '';
    opt.textContent = t('noSaved');
    savedEl.appendChild(opt);
    return;
  }
  for(const p of items){
    const opt = document.createElement('option');
    opt.value = p; opt.textContent = p; savedEl.appendChild(opt);
  }
}
async function saveCurrentFolder(){
  const path = pathEl.value.trim();
  if(!path){ append(t('errPathEmpty')); return; }
  const r = await fetch('/api/folders/add',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({path})});
  if(!r.ok){ append('[error] '+await r.text()+'\n'); return; }
  await refreshFolders();
}
async function removeSelectedFolder(){
  const path = savedEl.value || '';
  if(!path){ return; }
  const r = await fetch('/api/folders/remove',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({path})});
  if(!r.ok){ append('[error] '+await r.text()+'\n'); return; }
  await refreshFolders();
}
function useSelectedFolder(){ const path = savedEl.value || ''; if(path){ pathEl.value = path; updateFolderHint(); } }
function escapeHtml(s){ return s.replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;'); }
function normDrive(d){
  if(!d){ return ''; }
  d = d.trim().replace(/[\\\/]+$/g,'');
  if(/^[a-zA-Z]:$/.test(d)){ return d.toUpperCase(); }
  if(/^[a-zA-Z]:/.test(d)){ return d.slice(0,2).toUpperCase(); }
  return '';
}
function updateDriveSelectedPill(){
  if(!driveSelectedEl){ return; }
  const show = selectedDrive || '-';
  driveSelectedEl.textContent = t('diskSelected') + ': ' + show;
}
function selectDrive(drive){
  const d = normDrive(drive);
  if(!d){ return; }
  selectedDrive = d;
  pathEl.value = d + '\\';
  updateDriveSelectedPill();
  updateFolderHint();
}
async function openDrive(drive){
  const d = normDrive(drive);
  if(!d){ return; }
  const r = await fetch('/api/system/open-drive',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({drive:d})});
  if(!r.ok){ append('[error] '+await r.text()+'\n'); }
}
function setPathQuick(kind){
  const home = userDefaults.home || pathEl.value.trim() || '';
  const downloads = userDefaults.downloads || (home ? home + '\\Downloads' : '');
  const desktop = userDefaults.desktop || (home ? home + '\\Desktop' : '');
  const documents = userDefaults.documents || (home ? home + '\\Documents' : '');
  if(kind === 'home'){ pathEl.value = home; }
  if(kind === 'downloads'){ pathEl.value = downloads; }
  if(kind === 'desktop'){ pathEl.value = desktop; }
  if(kind === 'documents'){ pathEl.value = documents; }
  selectedDrive = normDrive(pathEl.value);
  updateDriveSelectedPill();
  updateFolderHint();
}
async function openCurrentPath(){
  const path = pathEl.value.trim();
  const r = await fetch('/api/path/open',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({path})});
  if(!r.ok){ append('[error] '+await r.text()+'\n'); }
}
async function analyzeCurrentPath(){
  await runCmd('tree');
  setHeavyOpen(true, true);
}
function triggerKonami(source){
  const title = document.getElementById('konamiTitle');
  const sub = document.getElementById('konamiSub');
  if(title){ title.textContent = t('konamiTitle'); }
  if(sub){ sub.textContent = t('konamiSub') + (source === 'dev' ? ' [dev]' : ''); }
  spawnKonamiParticles();
  konamiEl.classList.add('show');
  document.body.classList.add('shake');
  setTimeout(() => document.body.classList.remove('shake'), 1150);
  setTimeout(() => hideKonami(), 2800);
}
function spawnKonamiParticles(){
  if(!konamiBurst){ return; }
  konamiBurst.innerHTML = '';
  const n = 84;
  for(let i=0;i<n;i++){
    const p = document.createElement('span');
    p.className = 'konamiParticle';
    p.style.left = (42 + Math.random()*16) + '%';
    p.style.top = (36 + Math.random()*26) + '%';
    p.style.setProperty('--dx', (Math.random()*640 - 320) + 'px');
    p.style.setProperty('--dy', (Math.random()*420 - 210) + 'px');
    p.style.animationDelay = (Math.random()*0.32) + 's';
    p.style.opacity = (0.5 + Math.random()*0.5).toFixed(2);
    p.style.width = (6 + Math.random()*10) + 'px';
    p.style.height = p.style.width;
    konamiBurst.appendChild(p);
  }
}
function hideKonami(){
  konamiEl.classList.remove('show');
  if(konamiBurst){ konamiBurst.innerHTML = ''; }
}
async function runDrive(mode, drive){
  selectDrive(drive);
  if(mode === 'tree'){ await runCmd('tree'); }
  else if(mode === 'heavy'){ await runCmd('heavy'); }
}
function renderStorage(items){
  if(!items || items.length===0){ storageGrid.innerHTML = '<div class="storageCard">No disk data</div>'; return; }
  let html = '';
  for(const d of items){
    const pct = Math.max(0, Math.min(100, Math.round((d.usedRatio || 0) * 100)));
    const drv = normDrive(d.drive || '');
    html += '<div class="storageCard">'
      + '<div><b>'+escapeHtml(drv)+'</b></div>'
      + '<div>'+escapeHtml(d.usedHuman)+' / '+escapeHtml(d.totalHuman)+'</div>'
      + '<div class="bar"><div class="barFill" style="width:'+pct+'%"></div></div>'
      + '<div style="color:var(--muted);font-size:12px;margin-top:4px">'+pct+'%</div>'
      + '<div class="row" style="margin-top:8px">'
      + '<button onclick="selectDrive(\''+drv+'\')">'+t('diskUsePath')+'</button>'
      + '<button onclick="runDrive(\'tree\',\''+drv+'\')">'+t('diskTree')+'</button>'
      + '<button onclick="runDrive(\'heavy\',\''+drv+'\')">'+t('diskHeavy')+'</button>'
      + '<button onclick="openDrive(\''+drv+'\')">'+t('diskOpen')+'</button>'
      + '</div>'
      + '</div>';
  }
  storageGrid.innerHTML = html;
}
async function loadStorage(){
  const r = await fetch('/api/system/storage');
  if(!r.ok){ return; }
  const d = await r.json();
  renderStorage(d.items || []);
}
function encPath(path){ return encodeURIComponent(path); }
function decPath(path){ return decodeURIComponent(path); }
function rowActionButtons(path){
  const p = encPath(path);
  return '<button data-action="move-auto" data-path="'+p+'">'+t('autoMove')+'</button>'
    + ' <button data-action="move-custom" data-path="'+p+'">'+t('moveTo')+'</button>'
    + ' <button data-action="reveal" data-path="'+p+'">'+t('reveal')+'</button>'
    + ' <button class="warn" data-action="delete" data-path="'+p+'">'+t('delete')+'</button>';
}
function updateAutoRefreshLabel(){
  if(!autoRefreshBtn){ return; }
  autoRefreshBtn.textContent = autoRefreshHeavy ? t('autoRefreshOn') : t('autoRefreshOff');
}
function toggleAutoRefresh(){
  autoRefreshHeavy = !autoRefreshHeavy;
  updateAutoRefreshLabel();
  if(autoRefreshTimer){
    clearInterval(autoRefreshTimer);
    autoRefreshTimer = null;
  }
  if(autoRefreshHeavy){
    autoRefreshTimer = setInterval(() => {
      if(heavyOpen){ loadHeavyActions(); }
    }, 6000);
  }
}
function updateSelectionInfo(){
  if(!selectionInfoEl){ return; }
  selectionInfoEl.textContent = 'selected: ' + selectedPaths.size;
}
function getFilteredHeavyItems(){
  let items = [...lastHeavyItems];
  const q = heavySearchEl && heavySearchEl.value ? heavySearchEl.value.trim().toLowerCase() : '';
  const minMB = Number(minSizeMBEl && minSizeMBEl.value ? minSizeMBEl.value : 0) || 0;
  const minBytes = minMB * 1024 * 1024;
  if(q){ items = items.filter(it => (it.path || '').toLowerCase().includes(q)); }
  if(minBytes > 0){ items = items.filter(it => (it.size || 0) >= minBytes); }
  const mode = sortModeEl ? sortModeEl.value : 'size_desc';
  if(mode === 'size_asc'){ items.sort((a,b)=>(a.size||0)-(b.size||0)); }
  else if(mode === 'name_asc'){ items.sort((a,b)=>(a.path||'').localeCompare(b.path||'')); }
  else if(mode === 'name_desc'){ items.sort((a,b)=>(b.path||'').localeCompare(a.path||'')); }
  else { items.sort((a,b)=>(b.size||0)-(a.size||0)); }
  return items;
}
function renderHeavy(items){
  if(!items || items.length===0){ heavyBody.innerHTML = '<tr><td colspan="4">'+t('noHeavy')+'</td></tr>'; updateSelectionInfo(); return; }
  let html = '';
  for(const it of items){
    const checked = selectedPaths.has(it.path) ? 'checked' : '';
    const p = encPath(it.path || '');
    html += '<tr>'
      + '<td><input type="checkbox" data-action="select" data-path="'+p+'" '+checked+'/></td>'
      + '<td class="mono">'+escapeHtml(it.human)+'</td>'
      + '<td class="pathCell">'+escapeHtml(it.path)+'</td>'
      + '<td>'+rowActionButtons(it.path)+'</td>'
      + '</tr>';
  }
  heavyBody.innerHTML = html;
  updateSelectionInfo();
}
async function loadHeavyActions(){
  const path = pathEl.value.trim();
  const topN = document.getElementById('topN').value.trim();
  const r = await fetch('/api/heavy',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({path,topN})});
  if(!r.ok){ append('[error] '+await r.text()+'\n'); return; }
  const d = await r.json();
  lastHeavyItems = d.items || [];
  heavyViewItems = getFilteredHeavyItems();
  selectedPaths.clear();
  renderHeavy(heavyViewItems);
}
function applyHeavyFilter(){
  heavyViewItems = getFilteredHeavyItems();
  selectedPaths.clear();
  renderHeavy(heavyViewItems);
}
function clearHeavyFilter(){
  if(heavySearchEl){ heavySearchEl.value = ''; }
  if(minSizeMBEl){ minSizeMBEl.value = '0'; }
  if(sortModeEl){ sortModeEl.value = 'size_desc'; }
  applyHeavyFilter();
}
function refreshHeavy(){ return loadHeavyActions(); }
function saveHeavySnapshot(){
  heavySnapshot = heavyViewItems.map(it => ({path: it.path, size: it.size}));
  append('[snapshot] saved ' + heavySnapshot.length + ' items\n');
}
function compareHeavySnapshot(){
  if(!heavySnapshot.length){
    append('[snapshot] empty\n');
    return;
  }
  const oldMap = new Map(heavySnapshot.map(x => [x.path, x.size]));
  const nowMap = new Map(heavyViewItems.map(x => [x.path, x.size]));
  let added = 0, removed = 0, changed = 0;
  for(const p of nowMap.keys()){
    if(!oldMap.has(p)){ added++; }
    else if(oldMap.get(p) !== nowMap.get(p)){ changed++; }
  }
  for(const p of oldMap.keys()){
    if(!nowMap.has(p)){ removed++; }
  }
  append('[snapshot diff] added='+added+' removed='+removed+' changed='+changed+'\n');
}
function selectAllVisible(){
  selectedPaths.clear();
  for(const it of heavyViewItems){ selectedPaths.add(it.path); }
  renderHeavy(heavyViewItems);
}
function clearSelection(){
  selectedPaths.clear();
  renderHeavy(heavyViewItems);
}
async function bulkAutoMove(){
  const items = [...selectedPaths];
  if(items.length === 0){ append('[info] nothing selected\n'); return; }
  let ok = 0;
  for(const p of items){
    const r = await fetch('/api/file/move',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({path:p,dest:''})});
    if(r.ok){ ok++; }
  }
  append('[bulk auto move] '+ok+'/'+items.length+'\n');
  await loadHeavyActions(); await poll();
}
async function bulkMoveCustom(){
  const dest = document.getElementById('moveDest').value.trim();
  if(!dest){ append(t('errMoveEmpty')); return; }
  const items = [...selectedPaths];
  if(items.length === 0){ append('[info] nothing selected\n'); return; }
  let ok = 0;
  for(const p of items){
    const r = await fetch('/api/file/move',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({path:p,dest})});
    if(r.ok){ ok++; }
  }
  append('[bulk move] '+ok+'/'+items.length+'\n');
  await loadHeavyActions(); await poll();
}
async function bulkDelete(){
  const items = [...selectedPaths];
  if(items.length === 0){ append('[info] nothing selected\n'); return; }
  if(!confirm(t('confirmDelete') + '('+items.length+' files)')){ return; }
  let ok = 0;
  for(const p of items){
    const r = await fetch('/api/file/delete',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({path:p})});
    if(r.ok){ ok++; }
  }
  append('[bulk delete] '+ok+'/'+items.length+'\n');
  await loadHeavyActions(); await poll();
}
function exportHeavyCSV(){
  const rows = [['size_bytes','size_human','path']];
  for(const it of heavyViewItems){ rows.push([String(it.size||0), String(it.human||''), String(it.path||'')]); }
  const csv = rows.map(r => r.map(v => '"'+v.replaceAll('"','""')+'"').join(',')).join('\n');
  const blob = new Blob([csv], {type:'text/csv;charset=utf-8'});
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = 'icicle-heavy.csv';
  a.click();
  URL.revokeObjectURL(a.href);
}
function exportHeavyJSON(){
  const blob = new Blob([JSON.stringify(heavyViewItems, null, 2)], {type:'application/json;charset=utf-8'});
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = 'icicle-heavy.json';
  a.click();
  URL.revokeObjectURL(a.href);
}
async function copySelectedPaths(){
  const items = selectedPaths.size ? [...selectedPaths] : heavyViewItems.map(it => it.path);
  if(items.length === 0){ return; }
  try{
    await navigator.clipboard.writeText(items.join('\n'));
    append('[copy] '+items.length+' path(s)\n');
  }catch(_){
    append('[error] clipboard unavailable\n');
  }
}
async function revealPath(path){
  const r = await fetch('/api/path/reveal',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({path})});
  if(!r.ok){ append('[error] '+await r.text()+'\n'); }
}
async function cleanEmptyFolders(){
  const path = pathEl.value.trim();
  const r = await fetch('/api/path/clean-empty',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({path})});
  if(!r.ok){ append('[error] '+await r.text()+'\n'); return; }
  const d = await r.json();
  append('[clean-empty] removed '+(d.removed||0)+' folders\n');
}
async function analyzeExtensions(){
  const path = pathEl.value.trim();
  const r = await fetch('/api/path/extensions',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({path})});
  if(!r.ok){ append('[error] '+await r.text()+'\n'); return; }
  const d = await r.json();
  const items = d.items || [];
  append('[ext breakdown]\n');
  for(const it of items.slice(0,10)){ append('  '+it.ext+'  '+it.human+'  ('+it.count+')\n'); }
}
async function findDuplicates(){
  const path = pathEl.value.trim();
  const r = await fetch('/api/path/duplicates',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({path})});
  if(!r.ok){ append('[error] '+await r.text()+'\n'); return; }
  const d = await r.json();
  const items = d.items || [];
  append('[duplicates]\n');
  for(const it of items.slice(0,10)){ append('  '+it.name+' x'+it.count+'\n'); }
}
function exportReportMD(){
  const lines = [];
  lines.push('# icicle report');
  lines.push('');
  lines.push('Path: '+(pathEl.value.trim()||'(empty)'));
  lines.push('Generated: '+new Date().toISOString());
  lines.push('');
  lines.push('## Heavy files');
  for(const it of heavyViewItems.slice(0,50)){ lines.push('- '+it.human+'  '+it.path); }
  const blob = new Blob([lines.join('\n')], {type:'text/markdown;charset=utf-8'});
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = 'icicle-report.md';
  a.click();
  URL.revokeObjectURL(a.href);
}
function updateHeavyToggleLabel(){
  if(!heavyToggleBtn){ return; }
  heavyToggleBtn.textContent = heavyOpen ? t('hideHeavyActions') : t('showHeavyActions');
}
function setHeavyOpen(open, refresh){
  heavyOpen = !!open;
  if(heavyPanel){ heavyPanel.style.display = heavyOpen ? 'block' : 'none'; }
  updateHeavyToggleLabel();
  if(heavyOpen && refresh !== false){
    loadHeavyActions();
  }
}
function toggleHeavyPanel(){
  setHeavyOpen(!heavyOpen, true);
}
async function moveAuto(path){ await moveFile(path,''); }
async function moveCustom(path){
  const dest = document.getElementById('moveDest').value.trim();
  if(!dest){ append(t('errMoveEmpty')); return; }
  await moveFile(path,dest);
}
async function moveFile(path,dest){
  const r = await fetch('/api/file/move',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({path,dest})});
  if(!r.ok){ append('[error] '+await r.text()+'\n'); return; }
  await loadHeavyActions(); await poll();
}
async function undoMove(){
  const r = await fetch('/api/file/undo-move',{method:'POST'});
  if(!r.ok){ append('[error] '+await r.text()+'\n'); return; }
  await loadHeavyActions(); await poll();
}
async function deleteFile(path){
  if(!confirm(t('confirmDelete')+path)){ return; }
  const r = await fetch('/api/file/delete',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({path})});
  if(!r.ok){ append('[error] '+await r.text()+'\n'); return; }
  await loadHeavyActions(); await poll();
}
async function updateFolderHint(){
  const path = pathEl.value.trim();
  const r = await fetch('/api/folder/hint',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({path})});
  if(!r.ok){ folderKindEl.textContent = t('unknown'); return; }
  const d = await r.json();
  folderKindEl.textContent = d.kind || t('unknown');
}
pathEl.addEventListener('input', () => {
  selectedDrive = normDrive(pathEl.value);
  updateDriveSelectedPill();
  if(hintTimer){ clearTimeout(hintTimer); }
  hintTimer = setTimeout(updateFolderHint, 320);
});
heavyBody.addEventListener('click', async (e) => {
  const actionEl = e.target.closest('[data-action]');
  if(!actionEl){ return; }
  const action = actionEl.getAttribute('data-action');
  const path = decPath(actionEl.getAttribute('data-path') || '');
  try{
    if(action === 'select'){
      if(actionEl.checked){ selectedPaths.add(path); } else { selectedPaths.delete(path); }
      updateSelectionInfo();
      return;
    }
    if(action === 'move-auto'){ await moveAuto(path); }
    else if(action === 'move-custom'){ await moveCustom(path); }
    else if(action === 'reveal'){ await revealPath(path); }
    else if(action === 'delete'){ await deleteFile(path); }
  }catch(err){
    append('[js error] '+(err && err.message ? err.message : String(err))+'\n');
  }
});
async function poll(){
  try{
    const r = await fetch('/api/watch/log');
    const d = await r.json();
    const serverLog = d.log || '';
    if(serverLog.length < lastLogLen){ logEl.textContent = serverLog; lastLogLen = serverLog.length; }
    else if(serverLog.length > lastLogLen){
      logEl.textContent += serverLog.slice(lastLogLen);
      lastLogLen = serverLog.length;
      logEl.scrollTop = logEl.scrollHeight;
    }
  }catch(_){}
}
window.addEventListener('error', (e) => {
  append('[js error] '+(e && e.message ? e.message : 'unknown')+'\n');
});
if(heavySearchEl){ heavySearchEl.addEventListener('input', applyHeavyFilter); }
if(minSizeMBEl){ minSizeMBEl.addEventListener('input', applyHeavyFilter); }
if(sortModeEl){ sortModeEl.addEventListener('change', applyHeavyFilter); }
window.addEventListener('keydown', (e) => {
  const key = (e.key || '').toLowerCase();
  const target = konamiSeq[konamiPos].toLowerCase();
  if(key === target){
    konamiPos++;
    if(konamiPos >= konamiSeq.length){
      konamiPos = 0;
      triggerKonami('keys');
    }
  } else {
    konamiPos = key === konamiSeq[0].toLowerCase() ? 1 : 0;
  }
});
applyTheme();
applyLang();
setInterval(poll,900);
setDefaults();
</script>
</body>
</html>`
