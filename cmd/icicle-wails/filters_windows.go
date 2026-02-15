//go:build windows && wails

package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"icicle/internal/scan"
	"icicle/internal/ui"
)

type ScanFilters struct {
	IncludePath string `json:"includePath"`
	IgnorePath  string `json:"ignorePath"`
	IncludeExt  string `json:"includeExt"`
	IgnoreExt   string `json:"ignoreExt"`
}

func (f ScanFilters) active() bool {
	return strings.TrimSpace(f.IncludePath) != "" || strings.TrimSpace(f.IgnorePath) != "" || strings.TrimSpace(f.IncludeExt) != "" || strings.TrimSpace(f.IgnoreExt) != ""
}

func (a *App) RunHeavyFastFiltered(path string, n int, maxFiles int, workers int, filters ScanFilters) (HeavyResult, error) {
	if !filters.active() {
		return a.RunHeavyFast(path, n, maxFiles, workers)
	}
	path = a.normalizePath(path, a.folders.Home)
	if n <= 0 {
		n = 20
	}
	if maxFiles < 0 {
		maxFiles = 0
	}
	pred := newPathFilter(filters)
	started := time.Now()
	top := scan.NewTopFiles(n)
	total := int64(0)
	seen := 0
	count, err := scan.WalkAllLimit(path, maxFiles, func(p string, size int64) {
		if !pred.allow(p) {
			return
		}
		seen++
		total += size
		top.Push(scan.FileInfo{Path: p, Size: size})
	})
	if err != nil {
		return HeavyResult{}, err
	}
	items := top.ListDesc()
	outItems := make([]HeavyItem, 0, len(items))
	for _, it := range items {
		outItems = append(outItems, HeavyItem{Path: it.Path, Size: it.Size, Human: ui.HumanBytes(it.Size)})
	}
	outItems = a.markNewHeavy(path, outItems)
	res := HeavyResult{
		Items:      outItems,
		Seen:       seen,
		Limited:    maxFiles > 0 && count >= maxFiles,
		DurationMS: time.Since(started).Milliseconds(),
	}
	a.appendLog(fmt.Sprintf("[heavy-filtered] seen=%d ms=%d", res.Seen, res.DurationMS))
	_ = total
	return res, nil
}

func (a *App) RunTreeFastFiltered(path string, topN int, width int, maxFiles int, workers int, filters ScanFilters) (TreeResult, error) {
	if !filters.active() {
		return a.RunTreeFast(path, topN, width, maxFiles, workers)
	}
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
	pred := newPathFilter(filters)
	started := time.Now()
	root := filepath.Clean(path)
	rootPrefix := root
	if !strings.HasSuffix(rootPrefix, string(filepath.Separator)) {
		rootPrefix += string(filepath.Separator)
	}
	byChild := map[string]int64{}
	var total int64
	var rootFiles int64
	top := scan.NewTopFiles(topN)
	seen := 0
	count, err := scan.WalkAllLimit(root, maxFiles, func(p string, size int64) {
		if !pred.allow(p) {
			return
		}
		seen++
		total += size
		rel := p
		if strings.HasPrefix(p, rootPrefix) {
			rel = p[len(rootPrefix):]
		}
		if rel == "" {
			rootFiles += size
		} else {
			idx := strings.IndexAny(rel, `\\/`)
			if idx < 0 {
				rootFiles += size
			} else if idx > 0 {
				byChild[rel[:idx]] += size
			}
		}
		top.Push(scan.FileInfo{Path: p, Size: size})
	})
	if err != nil {
		return TreeResult{}, err
	}
	childNames := make([]string, 0, len(byChild))
	for k := range byChild {
		childNames = append(childNames, k)
	}
	sort.Slice(childNames, func(i, j int) bool { return byChild[childNames[i]] > byChild[childNames[j]] })
	var b strings.Builder
	theme := ui.Theme{NoColor: true, NoEmoji: true}
	b.WriteString(fmt.Sprintf("%s  (total: %s)\n", root, ui.HumanBytes(total)))
	limit := 20
	if len(childNames) < limit {
		limit = len(childNames)
	}
	for i := 0; i < limit; i++ {
		name := childNames[i]
		size := byChild[name]
		ratio := 0.0
		if total > 0 {
			ratio = float64(size) / float64(total)
		}
		prefix := "|-"
		if i == limit-1 && rootFiles == 0 {
			prefix = "`-"
		}
		b.WriteString(fmt.Sprintf("%s [DIR] %-20s %8s  %s\n", prefix, name, ui.HumanBytes(size), theme.Bar(ratio, width)))
	}
	if rootFiles > 0 {
		ratio := 0.0
		if total > 0 {
			ratio = float64(rootFiles) / float64(total)
		}
		b.WriteString(fmt.Sprintf("`- [FILES] %-18s %8s  %s\n", "(root)", ui.HumanBytes(rootFiles), theme.Bar(ratio, width)))
	}
	b.WriteString("\nTOP FILES:\n")
	for _, file := range top.ListDesc() {
		rel, relErr := filepath.Rel(root, file.Path)
		if relErr != nil {
			rel = file.Path
		}
		b.WriteString(fmt.Sprintf("%8s  %s\n", ui.HumanBytes(file.Size), rel))
	}
	res := TreeResult{Output: b.String(), Seen: seen, Limited: maxFiles > 0 && count >= maxFiles, DurationMS: time.Since(started).Milliseconds()}
	a.appendLog(fmt.Sprintf("[tree-filtered] seen=%d ms=%d", res.Seen, res.DurationMS))
	return res, nil
}

func (a *App) ExtensionStatsFastFiltered(path string, limit int, maxFiles int, workers int, filters ScanFilters) (ExtStatsResult, error) {
	if !filters.active() {
		return a.ExtensionStatsFast(path, limit, maxFiles, workers)
	}
	path = a.normalizePath(path, a.folders.Home)
	if limit <= 0 {
		limit = 20
	}
	if maxFiles < 0 {
		maxFiles = 0
	}
	pred := newPathFilter(filters)
	started := time.Now()
	byExt := map[string]ExtStat{}
	seen := 0
	count, err := scan.WalkAllLimit(path, maxFiles, func(p string, size int64) {
		if !pred.allow(p) {
			return
		}
		seen++
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
		return ExtStatsResult{}, err
	}
	out := make([]ExtStat, 0, len(byExt))
	for _, v := range byExt {
		v.Human = ui.HumanBytes(v.Size)
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Size == out[j].Size {
			return out[i].Count > out[j].Count
		}
		return out[i].Size > out[j].Size
	})
	if len(out) > limit {
		out = out[:limit]
	}
	res := ExtStatsResult{Items: out, Seen: seen, Limited: maxFiles > 0 && count >= maxFiles, DurationMS: time.Since(started).Milliseconds()}
	a.appendLog(fmt.Sprintf("[ext-filtered] seen=%d ms=%d", res.Seen, res.DurationMS))
	return res, nil
}

type pathFilter struct {
	includePath []string
	ignorePath  []string
	includeExt  map[string]bool
	ignoreExt   map[string]bool
}

func newPathFilter(in ScanFilters) pathFilter {
	f := pathFilter{
		includePath: splitFilterCSV(in.IncludePath),
		ignorePath:  splitFilterCSV(in.IgnorePath),
		includeExt:  splitExtCSV(in.IncludeExt),
		ignoreExt:   splitExtCSV(in.IgnoreExt),
	}
	return f
}

func splitFilterCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(strings.ReplaceAll(p, "/", "\\")))
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func splitExtCSV(raw string) map[string]bool {
	parts := strings.Split(raw, ",")
	out := map[string]bool{}
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p == "" {
			continue
		}
		if !strings.HasPrefix(p, ".") {
			p = "." + p
		}
		out[p] = true
	}
	return out
}

func (f pathFilter) allow(path string) bool {
	lp := strings.ToLower(strings.ReplaceAll(path, "/", "\\"))
	ext := strings.ToLower(filepath.Ext(lp))
	if len(f.includeExt) > 0 {
		if !f.includeExt[ext] {
			return false
		}
	}
	if len(f.ignoreExt) > 0 && f.ignoreExt[ext] {
		return false
	}
	if len(f.includePath) > 0 {
		ok := false
		for _, pat := range f.includePath {
			if strings.Contains(lp, pat) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	for _, pat := range f.ignorePath {
		if strings.Contains(lp, pat) {
			return false
		}
	}
	return true
}

func parseIntSafe(raw string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return def
	}
	return n
}
