package organize

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDestinationDir(t *testing.T) {
	home := filepath.Clean(`C:\Users\demo`)
	dst, ok := DestinationDir(home, `C:\tmp\movie.mp4`)
	if !ok {
		t.Fatalf("expected extension to be recognized")
	}
	want := filepath.Join(home, "Videos")
	if dst != want {
		t.Fatalf("got %q want %q", dst, want)
	}
}

func TestDestinationDirUnknown(t *testing.T) {
	_, ok := DestinationDir(`C:\Users\demo`, `C:\tmp\file.unknown`)
	if ok {
		t.Fatalf("expected unknown extension to be skipped")
	}
}

func TestEnsureUniquePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.zip")
	if err := osWrite(path); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	got, err := EnsureUniquePath(path)
	if err != nil {
		t.Fatalf("EnsureUniquePath error: %v", err)
	}
	if got == path {
		t.Fatalf("expected unique path different from existing path")
	}
}

func osWrite(path string) error {
	return os.WriteFile(path, []byte("x"), 0o644)
}
