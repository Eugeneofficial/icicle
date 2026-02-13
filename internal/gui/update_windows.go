//go:build windows

package gui

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultUpdateRepo = "Eugeneofficial/icicle"

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type releaseInfo struct {
	TagName string         `json:"tag_name"`
	Name    string         `json:"name"`
	Body    string         `json:"body"`
	Assets  []releaseAsset `json:"assets"`
}

type updateInfo struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	HasUpdate bool   `json:"hasUpdate"`
	AssetName string `json:"assetName"`
	AssetURL  string `json:"assetUrl"`
	Notes     string `json:"notes"`
}

func updateRepo() string {
	if v := strings.TrimSpace(os.Getenv("ICICLE_UPDATE_REPO")); v != "" {
		return v
	}
	return defaultUpdateRepo
}

func parseVersion(v string) []int {
	v = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(v), "v"))
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		num := strings.Builder{}
		for _, r := range p {
			if r < '0' || r > '9' {
				break
			}
			num.WriteRune(r)
		}
		if num.Len() == 0 {
			out = append(out, 0)
			continue
		}
		n, _ := strconv.Atoi(num.String())
		out = append(out, n)
	}
	return out
}

func compareVersions(a, b string) int {
	av := parseVersion(a)
	bv := parseVersion(b)
	n := len(av)
	if len(bv) > n {
		n = len(bv)
	}
	for i := 0; i < n; i++ {
		ai, bi := 0, 0
		if i < len(av) {
			ai = av[i]
		}
		if i < len(bv) {
			bi = bv[i]
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return 0
}

func pickAsset(assets []releaseAsset) releaseAsset {
	if len(assets) == 0 {
		return releaseAsset{}
	}
	// ну блять я не знаю как это работает, пусть будет this priority.
	for _, a := range assets {
		n := strings.ToLower(a.Name)
		if n == "icicle.exe" {
			return a
		}
	}
	for _, a := range assets {
		n := strings.ToLower(a.Name)
		if strings.HasSuffix(n, ".zip") && strings.Contains(n, "windows") {
			return a
		}
	}
	for _, a := range assets {
		n := strings.ToLower(a.Name)
		if strings.HasSuffix(n, ".exe") || strings.HasSuffix(n, ".zip") {
			return a
		}
	}
	return assets[0]
}

func fetchLatestRelease(repo string) (releaseInfo, error) {
	url := "https://api.github.com/repos/" + repo + "/releases/latest"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return releaseInfo{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "icicle-updater/"+time.Now().Format("20060102"))
	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return releaseInfo{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return releaseInfo{}, fmt.Errorf("github api status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var rel releaseInfo
	if err := json.NewDecoder(res.Body).Decode(&rel); err != nil {
		return releaseInfo{}, err
	}
	return rel, nil
}

func checkForUpdate(currentVersion string) (updateInfo, error) {
	repo := updateRepo()
	rel, err := fetchLatestRelease(repo)
	if err != nil {
		return updateInfo{}, err
	}
	latest := strings.TrimSpace(rel.TagName)
	if latest == "" {
		latest = strings.TrimSpace(rel.Name)
	}
	if latest == "" {
		return updateInfo{}, fmt.Errorf("release has no version")
	}
	asset := pickAsset(rel.Assets)
	info := updateInfo{
		Current:   currentVersion,
		Latest:    strings.TrimPrefix(latest, "v"),
		HasUpdate: compareVersions(currentVersion, latest) < 0,
		AssetName: asset.Name,
		AssetURL:  asset.URL,
		Notes:     rel.Body,
	}
	return info, nil
}

func downloadFile(url, dst string) error {
	client := &http.Client{Timeout: 30 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "icicle-updater")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("download failed: status %d", res.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, res.Body)
	return err
}

func extractExe(zipPath, dstExe string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		name := strings.ToLower(filepath.Base(f.Name))
		if name != "icicle.exe" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		out, err := os.Create(dstExe)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			_ = out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("icicle.exe not found in archive")
}

func prepareUpdateBinary(appPath, currentVersion string) (updateInfo, string, error) {
	info, err := checkForUpdate(currentVersion)
	if err != nil {
		return updateInfo{}, "", err
	}
	if !info.HasUpdate {
		return info, "", nil
	}
	if info.AssetURL == "" {
		return updateInfo{}, "", fmt.Errorf("release has no downloadable asset")
	}

	tmpDir := filepath.Join(os.TempDir(), "icicle-update")
	_ = os.MkdirAll(tmpDir, 0o755)
	downloadPath := filepath.Join(tmpDir, fmt.Sprintf("icicle-%s.tmp", time.Now().Format("20060102150405")))
	if err := downloadFile(info.AssetURL, downloadPath); err != nil {
		return updateInfo{}, "", err
	}

	newExe := appPath + ".new"
	name := strings.ToLower(info.AssetName)
	if strings.HasSuffix(name, ".exe") {
		_ = os.Remove(newExe)
		if err := os.Rename(downloadPath, newExe); err != nil {
			if err := copyFile(downloadPath, newExe); err != nil {
				return updateInfo{}, "", err
			}
			_ = os.Remove(downloadPath)
		}
		return info, newExe, nil
	}

	if strings.HasSuffix(name, ".zip") {
		if err := extractExe(downloadPath, newExe); err != nil {
			return updateInfo{}, "", err
		}
		_ = os.Remove(downloadPath)
		return info, newExe, nil
	}

	return updateInfo{}, "", fmt.Errorf("unsupported asset type: %s", info.AssetName)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func launchSwapScript(targetExe, newExe string, oldPID int) error {
	scriptPath := filepath.Join(os.TempDir(), fmt.Sprintf("icicle-update-%d.cmd", time.Now().UnixNano()))
	script := strings.Join([]string{
		"@echo off",
		"setlocal",
		"set \"TARGET=" + targetExe + "\"",
		"set \"NEW=" + newExe + "\"",
		"set \"PID=" + strconv.Itoa(oldPID) + "\"",
		"for /L %%i in (1,1,120) do (",
		"  tasklist /FI \"PID eq %PID%\" | find \"%PID%\" >nul",
		"  if errorlevel 1 goto :swap",
		"  timeout /t 1 /nobreak >nul",
		")",
		":swap",
		"move /Y \"%NEW%\" \"%TARGET%\" >nul 2>nul",
		"if errorlevel 1 (",
		"  copy /Y \"%NEW%\" \"%TARGET%\" >nul 2>nul",
		"  del /F /Q \"%NEW%\" >nul 2>nul",
		")",
		"start \"\" \"%TARGET%\" gui",
		"del /F /Q \"%~f0\"",
	}, "\r\n")
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		return err
	}
	return execCommand("cmd.exe", "/C", scriptPath)
}

func execCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Start()
}
