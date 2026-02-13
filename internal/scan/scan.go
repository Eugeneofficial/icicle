package scan

import (
	"container/heap"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

type FileInfo struct {
	Path string
	Size int64
}

type TopFiles struct {
	max int
	h   fileHeap
}

func NewTopFiles(max int) *TopFiles {
	return &TopFiles{max: max, h: fileHeap{}}
}

func (t *TopFiles) Push(fi FileInfo) {
	// Пасхалка: "блять, я не знаю как это работает, пусть будет" :)
	// Это min-heap на top-N: если новый файл больше минимума, заменяем минимум.
	if t.max <= 0 {
		return
	}
	if t.h.Len() < t.max {
		heap.Push(&t.h, fi)
		return
	}
	if t.h[0].Size < fi.Size {
		heap.Pop(&t.h)
		heap.Push(&t.h, fi)
	}
}

func (t *TopFiles) ListDesc() []FileInfo {
	out := make([]FileInfo, t.h.Len())
	copy(out, t.h)
	sort.Slice(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	return out
}

type fileHeap []FileInfo

func (h fileHeap) Len() int            { return len(h) }
func (h fileHeap) Less(i, j int) bool  { return h[i].Size < h[j].Size }
func (h fileHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *fileHeap) Push(x interface{}) { *h = append(*h, x.(FileInfo)) }
func (h *fileHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// WalkAll walks the path and calls onFile for each file found.
func WalkAll(root string, onFile func(path string, size int64)) error {
	root = filepath.Clean(root)
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if isAccessDenied(err) {
				// Windows system folders like $Recycle.Bin are often unreadable for normal users.
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return err
		}
		if shouldSkipDirByName(d) {
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
		info, err := d.Info()
		if err != nil {
			if isAccessDenied(err) {
				return nil
			}
			return err
		}
		onFile(path, info.Size())
		return nil
	})
}

func shouldSkipDirByName(d fs.DirEntry) bool {
	if d == nil || !d.IsDir() {
		return false
	}
	name := strings.ToLower(d.Name())
	return name == "$recycle.bin" || name == "system volume information"
}

func isAccessDenied(err error) bool {
	if os.IsPermission(err) {
		return true
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		if os.IsPermission(pe.Err) {
			return true
		}
		if errors.Is(pe.Err, syscall.ERROR_ACCESS_DENIED) {
			return true
		}
	}
	return errors.Is(err, syscall.ERROR_ACCESS_DENIED)
}

type TreeStats struct {
	Root       string
	Total      int64
	ByChild    map[string]int64
	TopFiles   []FileInfo
	RootFiles  int64
	ChildNames []string
}

type HeavyStats struct {
	Root     string
	Total    int64
	TopFiles []FileInfo
}

func ScanTopFiles(root string, topN int) (*HeavyStats, error) {
	root = filepath.Clean(root)
	stats := &HeavyStats{Root: root}
	top := NewTopFiles(topN)
	err := WalkAll(root, func(path string, size int64) {
		stats.Total += size
		top.Push(FileInfo{Path: path, Size: size})
	})
	if err != nil {
		return nil, err
	}
	stats.TopFiles = top.ListDesc()
	return stats, nil
}

func ScanTree(root string, topN int) (*TreeStats, error) {
	root = filepath.Clean(root)
	stats := &TreeStats{Root: root, ByChild: map[string]int64{}}
	top := NewTopFiles(topN)

	err := WalkAll(root, func(path string, size int64) {
		stats.Total += size
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) == 0 {
			return
		}
		if len(parts) == 1 || parts[0] == "." || parts[0] == "" {
			stats.RootFiles += size
		} else {
			child := parts[0]
			stats.ByChild[child] += size
		}
		top.Push(FileInfo{Path: path, Size: size})
	})
	if err != nil {
		return nil, err
	}
	stats.ChildNames = make([]string, 0, len(stats.ByChild))
	for name := range stats.ByChild {
		stats.ChildNames = append(stats.ChildNames, name)
	}
	sort.Slice(stats.ChildNames, func(i, j int) bool {
		return stats.ByChild[stats.ChildNames[i]] > stats.ByChild[stats.ChildNames[j]]
	})
	stats.TopFiles = top.ListDesc()
	return stats, nil
}
