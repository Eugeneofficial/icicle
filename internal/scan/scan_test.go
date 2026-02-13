package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanTree(t *testing.T) {
	root := t.TempDir()
	mustWriteSized(t, filepath.Join(root, "Videos", "a.mp4"), 10)
	mustWriteSized(t, filepath.Join(root, "Videos", "b.mkv"), 30)
	mustWriteSized(t, filepath.Join(root, "Downloads", "c.zip"), 20)
	mustWriteSized(t, filepath.Join(root, "note.txt"), 5)

	stats, err := ScanTree(root, 2)
	if err != nil {
		t.Fatalf("ScanTree error: %v", err)
	}
	if stats.Total != 65 {
		t.Fatalf("total mismatch: got %d want 65", stats.Total)
	}
	if stats.ByChild["Videos"] != 40 {
		t.Fatalf("Videos total mismatch: got %d want 40", stats.ByChild["Videos"])
	}
	if stats.RootFiles != 5 {
		t.Fatalf("root files mismatch: got %d want 5", stats.RootFiles)
	}
	if len(stats.TopFiles) != 2 {
		t.Fatalf("top len mismatch: got %d want 2", len(stats.TopFiles))
	}
	if stats.TopFiles[0].Size < stats.TopFiles[1].Size {
		t.Fatalf("top files should be in descending order")
	}
}

func mustWriteSized(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	b := make([]byte, size)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
