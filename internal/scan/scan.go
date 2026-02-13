package scan

import (
	"container/heap"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
)

type FileInfo struct {
	Path string
	Size int64
}

var errStopWalk = fmt.Errorf("scan stop")

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

// WalkAllLimit walks files up to maxFiles and then stops gracefully.
func WalkAllLimit(root string, maxFiles int, onFile func(path string, size int64)) (int, error) {
	if maxFiles <= 0 {
		err := WalkAll(root, onFile)
		return 0, err
	}
	root = filepath.Clean(root)
	count := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if isAccessDenied(err) {
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
		count++
		if count >= maxFiles {
			return errStopWalk
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStopWalk) {
		return count, err
	}
	return count, nil
}

func shouldSkipDirByName(d fs.DirEntry) bool {
	if d == nil || !d.IsDir() {
		return false
	}
	return shouldSkipDirName(d.Name())
}

func shouldSkipDirName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
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

func walkFilesConcurrent(root string, maxFiles int, onFile func(path string, size int64)) (int, bool, error) {
	root = filepath.Clean(root)
	workers := runtime.NumCPU()
	if workers < 4 {
		workers = 4
	}
	if workers > 16 {
		workers = 16
	}
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var seen atomic.Int64
	var stop atomic.Bool
	var firstErr error
	var errMu sync.Mutex

	setErr := func(err error) {
		if err == nil {
			return
		}
		errMu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		errMu.Unlock()
		stop.Store(true)
	}

	var walkDir func(dir string)
	walkDir = func(dir string) {
		if stop.Load() {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if !isAccessDenied(err) {
				setErr(err)
			}
			return
		}
		for _, e := range entries {
			if stop.Load() {
				return
			}
			name := e.Name()
			full := filepath.Join(dir, name)
			t := e.Type()
			if t&os.ModeSymlink != 0 {
				continue
			}
			if e.IsDir() {
				if shouldSkipDirName(name) {
					continue
				}
				// Try to process subdir in parallel; fallback to inline walk to avoid deadlocks.
				select {
				case sem <- struct{}{}:
					wg.Add(1)
					go func(p string) {
						defer func() {
							<-sem
							wg.Done()
						}()
						walkDir(p)
					}(full)
				default:
					walkDir(full)
				}
				continue
			}
			info, err := e.Info()
			if err != nil {
				if !isAccessDenied(err) {
					setErr(err)
				}
				continue
			}
			onFile(full, info.Size())
			n := int(seen.Add(1))
			if maxFiles > 0 && n >= maxFiles {
				stop.Store(true)
				return
			}
		}
	}

	// Root scan runs in current goroutine; spawned subdir goroutines are tracked via WaitGroup.
	walkDir(root)
	wg.Wait()

	errMu.Lock()
	err := firstErr
	errMu.Unlock()
	count := int(seen.Load())
	limited := maxFiles > 0 && count >= maxFiles
	return count, limited, err
}

func ScanTopFiles(root string, topN int) (*HeavyStats, error) {
	root = filepath.Clean(root)
	stats := &HeavyStats{Root: root}
	top := NewTopFiles(topN)
	var mu sync.Mutex
	_, _, err := walkFilesConcurrent(root, 0, func(path string, size int64) {
		mu.Lock()
		stats.Total += size
		top.Push(FileInfo{Path: path, Size: size})
		mu.Unlock()
	})
	if err != nil {
		return nil, err
	}
	stats.TopFiles = top.ListDesc()
	return stats, nil
}

func ScanTopFilesLimited(root string, topN int, maxFiles int) (*HeavyStats, int, bool, error) {
	root = filepath.Clean(root)
	stats := &HeavyStats{Root: root}
	top := NewTopFiles(topN)
	var mu sync.Mutex
	seen, limited, err := walkFilesConcurrent(root, maxFiles, func(path string, size int64) {
		mu.Lock()
		stats.Total += size
		top.Push(FileInfo{Path: path, Size: size})
		mu.Unlock()
	})
	if err != nil {
		return nil, seen, false, err
	}
	stats.TopFiles = top.ListDesc()
	return stats, seen, limited, nil
}

func ScanTree(root string, topN int) (*TreeStats, error) {
	root = filepath.Clean(root)
	stats := &TreeStats{Root: root, ByChild: map[string]int64{}}
	top := NewTopFiles(topN)
	rootPrefix := root
	if !strings.HasSuffix(rootPrefix, string(filepath.Separator)) {
		rootPrefix += string(filepath.Separator)
	}
	var mu sync.Mutex
	_, _, err := walkFilesConcurrent(root, 0, func(path string, size int64) {
		mu.Lock()
		stats.Total += size
		rel := path
		if strings.HasPrefix(path, rootPrefix) {
			rel = path[len(rootPrefix):]
		}
		if rel == "" {
			stats.RootFiles += size
		} else {
			idx := strings.IndexAny(rel, `\/`)
			if idx < 0 {
				stats.RootFiles += size
			} else if idx > 0 {
				stats.ByChild[rel[:idx]] += size
			}
		}
		top.Push(FileInfo{Path: path, Size: size})
		mu.Unlock()
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
