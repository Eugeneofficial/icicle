//go:build windows && wails

package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type CleanupScheduleStatus struct {
	Running     bool   `json:"running"`
	Path        string `json:"path"`
	Preset      string `json:"preset"`
	IntervalSec int    `json:"intervalSec"`
	Safe        bool   `json:"safe"`
	DryRun      bool   `json:"dryRun"`
	MaxDelete   int    `json:"maxDelete"`
	LastRunUnix int64  `json:"lastRunUnix"`
	LastStatus  string `json:"lastStatus"`
}

type scheduledCleanupState struct {
	Running     bool
	Path        string
	Preset      string
	IntervalSec int
	Safe        bool
	DryRun      bool
	MaxDelete   int
	LastRunUnix int64
	LastStatus  string
	Cancel      context.CancelFunc
}

func (a *App) StartScheduledCleanup(path string, preset string, intervalSec int, safe bool, dryRun bool, maxDelete int) error {
	path = a.normalizePath(path, a.folders.Downloads)
	preset = strings.TrimSpace(strings.ToLower(preset))
	if preset == "" {
		preset = "dev-cache"
	}
	if intervalSec < 60 {
		intervalSec = 60
	}
	if maxDelete <= 0 {
		maxDelete = 150
	}
	a.mu.Lock()
	if a.cleanup.Running {
		a.mu.Unlock()
		return fmt.Errorf("scheduled cleanup is already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cleanup = scheduledCleanupState{
		Running:     true,
		Path:        path,
		Preset:      preset,
		IntervalSec: intervalSec,
		Safe:        safe,
		DryRun:      dryRun,
		MaxDelete:   maxDelete,
		LastStatus:  "started",
		Cancel:      cancel,
	}
	a.mu.Unlock()
	go a.cleanupLoop(ctx)
	a.appendLog(fmt.Sprintf("[cleanup-schedule] started: every %ds preset=%s path=%s", intervalSec, preset, path))
	return nil
}

func (a *App) StopScheduledCleanup() {
	a.mu.Lock()
	cancel := a.cleanup.Cancel
	running := a.cleanup.Running
	a.cleanup.Running = false
	a.cleanup.Cancel = nil
	a.mu.Unlock()
	if running {
		a.appendLog("[cleanup-schedule] stopped")
	}
	if cancel != nil {
		cancel()
	}
}

func (a *App) ScheduledCleanupStatus() CleanupScheduleStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return CleanupScheduleStatus{
		Running:     a.cleanup.Running,
		Path:        a.cleanup.Path,
		Preset:      a.cleanup.Preset,
		IntervalSec: a.cleanup.IntervalSec,
		Safe:        a.cleanup.Safe,
		DryRun:      a.cleanup.DryRun,
		MaxDelete:   a.cleanup.MaxDelete,
		LastRunUnix: a.cleanup.LastRunUnix,
		LastStatus:  a.cleanup.LastStatus,
	}
}

func (a *App) RunScheduledCleanupOnce(path string, preset string, safe bool, dryRun bool, maxDelete int) (BatchResult, error) {
	path = a.normalizePath(path, a.folders.Downloads)
	if maxDelete <= 0 {
		maxDelete = 150
	}
	res, err := a.ScanCleanupPreset(path, preset, maxDelete, 0)
	if err != nil {
		return BatchResult{}, err
	}
	paths := make([]string, 0, len(res.Candidates))
	for i, c := range res.Candidates {
		if i >= maxDelete {
			break
		}
		paths = append(paths, c.Path)
	}
	if dryRun {
		a.appendLog(fmt.Sprintf("[cleanup-schedule] dry-run candidates=%d", len(paths)))
		return BatchResult{Processed: len(paths), Succeeded: len(paths)}, nil
	}
	br := a.ApplyPresetCleanup(paths, safe)
	a.appendLog(fmt.Sprintf("[cleanup-schedule] cleanup done: %d/%d", br.Succeeded, br.Processed))
	return br, nil
}

func (a *App) cleanupLoop(ctx context.Context) {
	run := func() {
		a.mu.Lock()
		st := a.cleanup
		a.mu.Unlock()
		br, err := a.RunScheduledCleanupOnce(st.Path, st.Preset, st.Safe, st.DryRun, st.MaxDelete)
		status := "ok"
		if err != nil {
			status = "error: " + err.Error()
		} else {
			status = fmt.Sprintf("ok: %d/%d", br.Succeeded, br.Processed)
		}
		a.mu.Lock()
		a.cleanup.LastRunUnix = time.Now().Unix()
		a.cleanup.LastStatus = status
		a.mu.Unlock()
	}
	run()
	a.mu.Lock()
	interval := a.cleanup.IntervalSec
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

type encryptedProfile struct {
	Version   int    `json:"version"`
	Salt      string `json:"salt"`
	Nonce     string `json:"nonce"`
	CipherB64 string `json:"cipherB64"`
}

type profilePayload struct {
	SavedFolders []string    `json:"savedFolders"`
	RouteRules   []RouteRule `json:"routeRules"`
	ExportedAt   int64       `json:"exportedAt"`
}

func (a *App) ExportProfileEncrypted(passphrase string) (string, error) {
	passphrase = strings.TrimSpace(passphrase)
	if len(passphrase) < 6 {
		return "", fmt.Errorf("passphrase must be at least 6 chars")
	}
	rules, err := a.ListRoutingRules()
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	saved := make([]string, len(a.saved))
	copy(saved, a.saved)
	a.mu.Unlock()
	pl := profilePayload{SavedFolders: saved, RouteRules: rules, ExportedAt: time.Now().Unix()}
	plain, err := json.Marshal(pl)
	if err != nil {
		return "", err
	}
	salt := make([]byte, 16)
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	key := deriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	cipherRaw := aead.Seal(nil, nonce, plain, nil)
	out := encryptedProfile{Version: 1, Salt: base64.StdEncoding.EncodeToString(salt), Nonce: base64.StdEncoding.EncodeToString(nonce), CipherB64: base64.StdEncoding.EncodeToString(cipherRaw)}
	body, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	target, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Export encrypted profile",
		DefaultFilename: "icicle-profile.enc.json",
		Filters:         []wailsruntime.FileFilter{{DisplayName: "Encrypted JSON", Pattern: "*.json"}},
	})
	if err != nil || strings.TrimSpace(target) == "" {
		return "", err
	}
	if err := os.WriteFile(target, body, 0o600); err != nil {
		return "", err
	}
	a.appendLog("[profile] exported: " + target)
	return target, nil
}

func (a *App) ImportProfileEncrypted(passphrase string) (string, error) {
	passphrase = strings.TrimSpace(passphrase)
	if len(passphrase) < 6 {
		return "", fmt.Errorf("passphrase must be at least 6 chars")
	}
	source, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:   "Import encrypted profile",
		Filters: []wailsruntime.FileFilter{{DisplayName: "Encrypted JSON", Pattern: "*.json"}},
	})
	if err != nil || strings.TrimSpace(source) == "" {
		return "", err
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return "", err
	}
	var enc encryptedProfile
	if err := json.Unmarshal(data, &enc); err != nil {
		return "", err
	}
	salt, err := base64.StdEncoding.DecodeString(enc.Salt)
	if err != nil {
		return "", err
	}
	nonce, err := base64.StdEncoding.DecodeString(enc.Nonce)
	if err != nil {
		return "", err
	}
	cipherRaw, err := base64.StdEncoding.DecodeString(enc.CipherB64)
	if err != nil {
		return "", err
	}
	key := deriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plain, err := aead.Open(nil, nonce, cipherRaw, nil)
	if err != nil {
		return "", fmt.Errorf("wrong passphrase or corrupted file")
	}
	var pl profilePayload
	if err := json.Unmarshal(plain, &pl); err != nil {
		return "", err
	}
	pl.SavedFolders = dedupePaths(pl.SavedFolders)
	a.mu.Lock()
	a.saved = pl.SavedFolders
	err = a.saveSavedLocked()
	a.mu.Unlock()
	if err != nil {
		return "", err
	}
	if err := a.SaveRoutingRules(pl.RouteRules); err != nil {
		return "", err
	}
	a.appendLog(fmt.Sprintf("[profile] imported: folders=%d rules=%d", len(pl.SavedFolders), len(pl.RouteRules)))
	return source, nil
}

func deriveKey(passphrase string, salt []byte) []byte {
	h := sha256.Sum256(append([]byte(passphrase), salt...))
	for i := 0; i < 12000; i++ {
		h = sha256.Sum256(h[:])
	}
	k := make([]byte, 32)
	copy(k, h[:])
	return k
}
