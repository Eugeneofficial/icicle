package commands

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWaitForStableFileReturnsStableInfo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "video.mp4")
	if err := os.WriteFile(path, []byte("stable"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	info, err := waitForStableFile(path)
	if err != nil {
		t.Fatalf("waitForStableFile error: %v", err)
	}
	if info.Size() != int64(len("stable")) {
		t.Fatalf("size=%d want %d", info.Size(), len("stable"))
	}
}

func TestWaitForStableFileWaitsForChangesToStop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "video.mp4")
	if err := os.WriteFile(path, []byte("a"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = os.WriteFile(path, []byte("abcd"), 0o644)
	}()

	info, err := waitForStableFile(path)
	if err != nil {
		t.Fatalf("waitForStableFile error: %v", err)
	}
	if info.Size() != 4 {
		t.Fatalf("size=%d want 4", info.Size())
	}
}
