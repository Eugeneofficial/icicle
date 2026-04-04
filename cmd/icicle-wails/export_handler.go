//go:build windows && wails

package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

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
