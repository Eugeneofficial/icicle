//go:build windows && wails

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"icicle/internal/ui"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

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

func (a *App) SnapshotTreemapDiff(leftFile string, rightFile string, top int) (SnapshotTreemapCompare, error) {
	if top <= 0 {
		top = 120
	}
	left, err := readSnapshotFile(leftFile)
	if err != nil {
		return SnapshotTreemapCompare{}, err
	}
	right, err := readSnapshotFile(rightFile)
	if err != nil {
		return SnapshotTreemapCompare{}, err
	}
	leftMap := make(map[string]int64, len(left.Items))
	rightMap := make(map[string]int64, len(right.Items))
	for _, it := range left.Items {
		leftMap[strings.ToLower(it.Path)] = it.Size
	}
	for _, it := range right.Items {
		rightMap[strings.ToLower(it.Path)] = it.Size
	}
	out := SnapshotTreemapCompare{Left: leftFile, Right: rightFile}
	items := make([]SnapshotTreemapItem, 0, len(leftMap)+len(rightMap))
	for p, rv := range rightMap {
		lv := leftMap[p]
		if rv == lv {
			continue
		}
		delta := rv - lv
		status := "changed"
		if lv == 0 {
			status = "added"
		}
		abs := delta
		if abs < 0 {
			abs = -abs
		}
		h := ui.HumanBytes(delta)
		if delta > 0 {
			h = "+" + h
		}
		items = append(items, SnapshotTreemapItem{
			Path:    p,
			Name:    filepath.Base(p),
			Delta:   delta,
			Human:   h,
			Status:  status,
			AbsSize: abs,
		})
	}
	for p, lv := range leftMap {
		if _, ok := rightMap[p]; ok {
			continue
		}
		items = append(items, SnapshotTreemapItem{
			Path:    p,
			Name:    filepath.Base(p),
			Delta:   -lv,
			Human:   "-" + ui.HumanBytes(lv),
			Status:  "removed",
			AbsSize: lv,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].AbsSize > items[j].AbsSize })
	if len(items) > top {
		items = items[:top]
	}
	totalAbs := int64(0)
	for _, it := range items {
		totalAbs += it.AbsSize
	}
	out.TotalAbs = totalAbs
	out.TotalHuman = ui.HumanBytes(totalAbs)
	out.Items = items
	return out, nil
}

func (a *App) ExportSnapshotCompare(leftFile string, rightFile string, top int, format string, mode string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	mode = strings.ToLower(strings.TrimSpace(mode))
	if format == "" {
		format = "csv"
	}
	if mode == "" {
		mode = "diff"
	}

	var (
		body   string
		filter runtime.FileFilter
		ext    string
	)
	switch format {
	case "json":
		filter = runtime.FileFilter{DisplayName: "JSON", Pattern: "*.json"}
		ext = "json"
	case "md":
		filter = runtime.FileFilter{DisplayName: "Markdown", Pattern: "*.md"}
		ext = "md"
	default:
		filter = runtime.FileFilter{DisplayName: "CSV", Pattern: "*.csv"}
		ext = "csv"
		format = "csv"
	}

	if mode == "treemap" {
		res, err := a.SnapshotTreemapDiff(leftFile, rightFile, top)
		if err != nil {
			return "", err
		}
		switch format {
		case "json":
			raw, err := json.MarshalIndent(res, "", "  ")
			if err != nil {
				return "", err
			}
			body = string(raw) + "\n"
		case "md":
			var b strings.Builder
			b.WriteString("# Snapshot Treemap Compare\n\n")
			b.WriteString("- Left: `" + strings.ReplaceAll(res.Left, "`", "'") + "`\n")
			b.WriteString("- Right: `" + strings.ReplaceAll(res.Right, "`", "'") + "`\n")
			b.WriteString("- Total abs delta: **" + res.TotalHuman + "**\n\n")
			b.WriteString("| Status | Delta | Path |\n|---|---:|---|\n")
			for _, it := range res.Items {
				b.WriteString("| " + it.Status + " | " + it.Human + " | `" + strings.ReplaceAll(it.Path, "`", "'") + "` |\n")
			}
			body = b.String()
		default:
			var b strings.Builder
			b.WriteString("status,delta,path\n")
			for _, it := range res.Items {
				b.WriteString(csvLine([]string{it.Status, it.Human, it.Path}) + "\n")
			}
			body = b.String()
		}
	} else {
		res, err := a.SnapshotDiff(leftFile, rightFile, top)
		if err != nil {
			return "", err
		}
		switch format {
		case "json":
			raw, err := json.MarshalIndent(res, "", "  ")
			if err != nil {
				return "", err
			}
			body = string(raw) + "\n"
		case "md":
			var b strings.Builder
			b.WriteString("# Snapshot Diff\n\n")
			b.WriteString("- Left: `" + strings.ReplaceAll(res.Left, "`", "'") + "`\n")
			b.WriteString("- Right: `" + strings.ReplaceAll(res.Right, "`", "'") + "`\n")
			b.WriteString("- Added: **" + strconv.Itoa(res.Added) + "**\n")
			b.WriteString("- Removed: **" + strconv.Itoa(res.Removed) + "**\n")
			b.WriteString("- Changed: **" + strconv.Itoa(res.Changed) + "**\n\n")
			b.WriteString("| Status | Delta | Path |\n|---|---:|---|\n")
			for _, it := range res.Top {
				b.WriteString("| " + it.Status + " | " + it.Human + " | `" + strings.ReplaceAll(it.Path, "`", "'") + "` |\n")
			}
			body = b.String()
		default:
			var b strings.Builder
			b.WriteString("status,delta,path\n")
			for _, it := range res.Top {
				b.WriteString(csvLine([]string{it.Status, it.Human, it.Path}) + "\n")
			}
			body = b.String()
		}
	}

	filename := "snapshot-compare-" + time.Now().Format("20060102-150405") + "." + ext
	target, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export snapshot compare",
		DefaultFilename: filename,
		Filters:         []runtime.FileFilter{filter},
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(target) == "" {
		return "", nil
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		return "", err
	}
	a.appendLog("[snapshot-export] " + target)
	return target, nil
}

func csvLine(parts []string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.ReplaceAll(p, "\"", "\"\"")
		out = append(out, "\""+s+"\"")
	}
	return strings.Join(out, ",")
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
