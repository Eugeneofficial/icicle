package commands

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"icicle/internal/organize"
)

func runWatch(args []string) int {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var common commonFlags
	addCommonFlags(fs, &common)
	dryRun := fs.Bool("dry-run", false, "print actions without moving files")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "usage: icicle watch [--dry-run] [--no-color] [--no-emoji] [path]")
		return 2
	}
	applyCommonFlags(common)

	folders := detectUserFolders()
	watchPath := fs.Arg(0)
	if strings.TrimSpace(watchPath) == "" {
		watchPath = folders.Downloads
	}
	watchRoot, err := expandPath(watchPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "path error: %v\n", err)
		return 1
	}
	home := folders.Home

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "watcher error: %v\n", err)
		return 1
	}
	defer watcher.Close()

	if err := addRecursiveWatches(watcher, watchRoot); err != nil {
		fmt.Fprintf(os.Stderr, "watch add error: %v\n", err)
		return 1
	}

	fmt.Printf("watching %s\n", watchRoot)
	fmt.Printf("sorting destination base: %s\n", home)
	if *dryRun {
		fmt.Println("dry-run enabled")
	}
	fmt.Println("press Ctrl+C to stop")

	cooldown := map[string]time.Time{}
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return 0
			}
			if event.Op&(fsnotify.Create|fsnotify.Rename|fsnotify.Write) == 0 {
				continue
			}

			if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
				_ = addRecursiveWatches(watcher, event.Name)
				continue
			}

			if shouldSkipEvent(event.Name, cooldown) {
				continue
			}

			handled, msg := maybeMoveFile(home, event.Name, *dryRun)
			if handled {
				fmt.Println(msg)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return 0
			}
			fmt.Fprintf(os.Stderr, "watch error: %v\n", err)
		}
	}
}

func addRecursiveWatches(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if isWatchAccessDenied(err) {
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return err
		}
		if shouldSkipWatchDir(d) {
			return filepath.SkipDir
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if err := w.Add(path); err != nil {
				if isWatchAccessDenied(err) {
					return filepath.SkipDir
				}
				return err
			}
		}
		return nil
	})
}

func shouldSkipWatchDir(d os.DirEntry) bool {
	if d == nil || !d.IsDir() {
		return false
	}
	name := strings.ToLower(d.Name())
	return name == "$recycle.bin" || name == "system volume information"
}

func isWatchAccessDenied(err error) bool {
	if os.IsPermission(err) {
		return true
	}
	if errors.Is(err, fs.ErrPermission) {
		return true
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		if os.IsPermission(pe.Err) || errors.Is(pe.Err, fs.ErrPermission) {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "access is denied") || strings.Contains(msg, "permission denied")
}

func shouldSkipEvent(path string, cooldown map[string]time.Time) bool {
	// Пасхалка: "блять, я не знаю как это работает, пусть будет" :)
	// По факту это простой антидребезг, чтобы не ловить один и тот же файл по 10 раз.
	now := time.Now()
	last, ok := cooldown[path]
	if ok && now.Sub(last) < 2*time.Second {
		return true
	}
	cooldown[path] = now
	if len(cooldown) > 4096 {
		for k, v := range cooldown {
			if now.Sub(v) > 15*time.Second {
				delete(cooldown, k)
			}
		}
	}
	return false
}

func maybeMoveFile(home, srcPath string, dryRun bool) (bool, string) {
	info, err := os.Stat(srcPath)
	if err != nil || info.IsDir() {
		return false, ""
	}

	dstDir, ok := organize.DestinationDir(home, srcPath)
	if !ok {
		return false, ""
	}

	srcAbs, err := filepath.Abs(srcPath)
	if err != nil {
		return false, ""
	}
	dstCandidate := filepath.Join(dstDir, filepath.Base(srcAbs))
	dstAbs, err := filepath.Abs(dstCandidate)
	if err != nil {
		return false, ""
	}
	if strings.EqualFold(srcAbs, dstAbs) {
		return false, ""
	}

	dstUnique, err := organize.EnsureUniquePath(dstAbs)
	if err != nil {
		return true, fmt.Sprintf("skip %s (%v)", srcAbs, err)
	}

	if dryRun {
		return true, fmt.Sprintf("[dry-run] %s -> %s", srcAbs, dstUnique)
	}

	if err := organize.MoveFile(srcAbs, dstUnique); err != nil {
		return true, fmt.Sprintf("move failed %s (%v)", srcAbs, err)
	}

	if info.Size() > 4*1024*1024*1024 {
		// Easter egg: exceptionally large drops get a special line.
		return true, fmt.Sprintf("moved %s -> %s  [black-ice payload]", srcAbs, dstUnique)
	}
	return true, fmt.Sprintf("moved %s -> %s", srcAbs, dstUnique)
}
