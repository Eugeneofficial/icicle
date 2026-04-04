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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	// Uses min-heap to efficiently track only the N largest files.
	// If heap is not full, push directly. Otherwise, replace minimum if new file is larger.
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

// isAccessDenied is kept for backward compatibility within the package.
func isAccessDenied(err error) bool {
	return IsAccessDenied(err)
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

type ExtStatsItem struct {
	Ext   string
	Count int
	Size  int64
}

type OverviewStats struct {
	Root     string
	Total    int64
	Seen     int
	Limited  bool
	ByChild  map[string]int64
	TopFiles []FileInfo
	ExtStats []ExtStatsItem
}

func walkFilesConcurrent(root string, maxFiles int, workersOverride int, onFile func(path string, size int64)) (int, bool, error) {
	root = filepath.Clean(root)
	workers := workersOverride
	if workers <= 0 {
		workers = scanWorkers()
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
			full := fastJoin(dir, name)
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
			if maxFiles > 0 {
				for {
					cur := seen.Load()
					if int(cur) >= maxFiles {
						stop.Store(true)
						return
					}
					if seen.CompareAndSwap(cur, cur+1) {
						break
					}
				}
			}
			onFile(full, info.Size())
			if maxFiles <= 0 {
				seen.Add(1)
				continue
			}
			if int(seen.Load()) >= maxFiles {
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

// Scan worker concurrency constants.
const (
	minScanWorkers    = 8   // minimum workers for IO-bound scanning
	maxScanWorkers    = 32  // default maximum workers
	maxEnvScanWorkers = 128 // maximum workers when overridden via env var
)

func scanWorkers() int {
	// IO-bound scanning benefits from higher concurrency than CPU count.
	workers := runtime.NumCPU() * 2
	if workers < minScanWorkers {
		workers = minScanWorkers
	}
	if workers > maxScanWorkers {
		workers = maxScanWorkers
	}
	if raw := strings.TrimSpace(os.Getenv("ICICLE_SCAN_WORKERS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			if n < 1 {
				n = 1
			}
			if n > maxEnvScanWorkers {
				n = maxEnvScanWorkers
			}
			workers = n
		}
	}
	return workers
}

func fastJoin(dir, name string) string {
	if dir == "" {
		return name
	}
	last := dir[len(dir)-1]
	if os.IsPathSeparator(last) {
		return dir + name
	}
	return dir + string(filepath.Separator) + name
}

func firstPathSegment(rel string) string {
	if rel == "" {
		return ""
	}
	for i := 0; i < len(rel); i++ {
		if rel[i] == '\\' || rel[i] == '/' {
			if i == 0 {
				return ""
			}
			return rel[:i]
		}
	}
	return rel
}

func fastLowerExt(path string) string {
	// Faster than filepath.Ext for hot loops; supports Windows and POSIX separators.
	lastSep := -1
	lastDot := -1
	for i := len(path) - 1; i >= 0; i-- {
		c := path[i]
		if c == '\\' || c == '/' {
			lastSep = i
			break
		}
		if c == '.' && lastDot < 0 {
			lastDot = i
		}
	}
	if lastDot <= lastSep || lastDot < 0 || lastDot == len(path)-1 {
		return "(no_ext)"
	}
	return strings.ToLower(path[lastDot:])
}

func ScanTopFiles(root string, topN int, workers int) (*HeavyStats, error) {
	root = filepath.Clean(root)
	stats := &HeavyStats{Root: root}
	top := NewTopFiles(topN)
	var mu sync.Mutex
	_, _, err := walkFilesConcurrent(root, 0, workers, func(path string, size int64) {
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

func ScanTopFilesLimited(root string, topN int, maxFiles int, workers int) (*HeavyStats, int, bool, error) {
	root = filepath.Clean(root)
	stats := &HeavyStats{Root: root}
	top := NewTopFiles(topN)
	var mu sync.Mutex
	seen, limited, err := walkFilesConcurrent(root, maxFiles, workers, func(path string, size int64) {
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

func ScanTree(root string, topN int, workers int) (*TreeStats, error) {
	root = filepath.Clean(root)
	stats := &TreeStats{Root: root, ByChild: map[string]int64{}}
	top := NewTopFiles(topN)
	rootPrefix := root
	if !strings.HasSuffix(rootPrefix, string(filepath.Separator)) {
		rootPrefix += string(filepath.Separator)
	}
	var mu sync.Mutex
	_, _, err := walkFilesConcurrent(root, 0, workers, func(path string, size int64) {
		mu.Lock()
		stats.Total += size
		rel := path
		if strings.HasPrefix(path, rootPrefix) {
			rel = path[len(rootPrefix):]
		}
		if rel == "" {
			stats.RootFiles += size
		} else {
			child := firstPathSegment(rel)
			if child == rel || child == "" {
				stats.RootFiles += size
			} else {
				stats.ByChild[child] += size
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

func ScanTreeLimited(root string, topN int, maxFiles int, workers int) (*TreeStats, int, bool, error) {
	root = filepath.Clean(root)
	stats := &TreeStats{Root: root, ByChild: map[string]int64{}}
	top := NewTopFiles(topN)
	rootPrefix := root
	if !strings.HasSuffix(rootPrefix, string(filepath.Separator)) {
		rootPrefix += string(filepath.Separator)
	}
	var mu sync.Mutex
	seen, limited, err := walkFilesConcurrent(root, maxFiles, workers, func(path string, size int64) {
		mu.Lock()
		stats.Total += size
		rel := path
		if strings.HasPrefix(path, rootPrefix) {
			rel = path[len(rootPrefix):]
		}
		if rel == "" {
			stats.RootFiles += size
		} else {
			child := firstPathSegment(rel)
			if child == rel || child == "" {
				stats.RootFiles += size
			} else {
				stats.ByChild[child] += size
			}
		}
		top.Push(FileInfo{Path: path, Size: size})
		mu.Unlock()
	})
	if err != nil {
		return nil, seen, false, err
	}
	stats.ChildNames = make([]string, 0, len(stats.ByChild))
	for name := range stats.ByChild {
		stats.ChildNames = append(stats.ChildNames, name)
	}
	sort.Slice(stats.ChildNames, func(i, j int) bool {
		return stats.ByChild[stats.ChildNames[i]] > stats.ByChild[stats.ChildNames[j]]
	})
	stats.TopFiles = top.ListDesc()
	return stats, seen, limited, nil
}

func ScanExtStatsLimited(root string, maxFiles int, workers int) ([]ExtStatsItem, int, bool, error) {
	root = filepath.Clean(root)
	byExt := map[string]ExtStatsItem{}
	var mu sync.Mutex
	seen, limited, err := walkFilesConcurrent(root, maxFiles, workers, func(path string, size int64) {
		ext := fastLowerExt(path)
		mu.Lock()
		cur := byExt[ext]
		cur.Ext = ext
		cur.Count++
		cur.Size += size
		byExt[ext] = cur
		mu.Unlock()
	})
	if err != nil {
		return nil, seen, limited, err
	}
	out := make([]ExtStatsItem, 0, len(byExt))
	for _, v := range byExt {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Size == out[j].Size {
			return out[i].Count > out[j].Count
		}
		return out[i].Size > out[j].Size
	})
	return out, seen, limited, nil
}

func ScanOverviewLimited(root string, maxFiles int, topFilesN int, topExtN int, workers int) (*OverviewStats, error) {
	root = filepath.Clean(root)
	stats := &OverviewStats{
		Root:    root,
		ByChild: map[string]int64{},
	}
	top := NewTopFiles(topFilesN)
	extMap := map[string]ExtStatsItem{}
	rootPrefix := root
	if !strings.HasSuffix(rootPrefix, string(filepath.Separator)) {
		rootPrefix += string(filepath.Separator)
	}

	var mu sync.Mutex
	seen, limited, err := walkFilesConcurrent(root, maxFiles, workers, func(path string, size int64) {
		ext := fastLowerExt(path)
		rel := path
		if strings.HasPrefix(path, rootPrefix) {
			rel = path[len(rootPrefix):]
		}
		child := "(root)"
		if first := firstPathSegment(rel); first != "" && first != rel {
			child = first
		}

		mu.Lock()
		stats.Total += size
		stats.ByChild[child] += size
		top.Push(FileInfo{Path: path, Size: size})
		cur := extMap[ext]
		cur.Ext = ext
		cur.Count++
		cur.Size += size
		extMap[ext] = cur
		mu.Unlock()
	})
	if err != nil {
		return nil, err
	}

	stats.Seen = seen
	stats.Limited = limited
	stats.TopFiles = top.ListDesc()
	stats.ExtStats = make([]ExtStatsItem, 0, len(extMap))
	for _, v := range extMap {
		stats.ExtStats = append(stats.ExtStats, v)
	}
	sort.Slice(stats.ExtStats, func(i, j int) bool {
		if stats.ExtStats[i].Size == stats.ExtStats[j].Size {
			return stats.ExtStats[i].Count > stats.ExtStats[j].Count
		}
		return stats.ExtStats[i].Size > stats.ExtStats[j].Size
	})
	if topExtN > 0 && len(stats.ExtStats) > topExtN {
		stats.ExtStats = stats.ExtStats[:topExtN]
	}
	return stats, nil
}
