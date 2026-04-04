//go:build windows && wails

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"icicle/internal/scan"
	"icicle/internal/ui"
)

// ── § Duplicates ───────────────────────────────────────────────────

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
