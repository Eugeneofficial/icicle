//go:build windows && wails

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ── § Utilities ────────────────────────────────────────────────────

func (a *App) normalizePath(path string, fallback string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		path = fallback
	}
	return filepath.Clean(path)
}

func normalizePathKey(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return strings.ToLower(filepath.Clean(path))
}

func (a *App) markNewHeavy(root string, items []HeavyItem) []HeavyItem {
	if len(items) == 0 {
		return items
	}
	rootKey := normalizePathKey(root)
	if rootKey == "" {
		return items
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.heavySeenByRoot == nil {
		a.heavySeenByRoot = map[string]map[string]struct{}{}
	}
	seen := a.heavySeenByRoot[rootKey]
	if seen == nil {
		seen = map[string]struct{}{}
		for _, it := range items {
			seen[normalizePathKey(it.Path)] = struct{}{}
		}
		a.heavySeenByRoot[rootKey] = seen
		return items
	}
	if len(seen) > 500000 {
		seen = map[string]struct{}{}
	}
	for i := range items {
		key := normalizePathKey(items[i].Path)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; !ok {
			items[i].New = true
		}
		seen[key] = struct{}{}
	}
	a.heavySeenByRoot[rootKey] = seen
	return items
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
