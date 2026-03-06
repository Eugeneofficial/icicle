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

func TestCopyThenDelete(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dst := filepath.Join(dir, "dst.bin")
	payload := []byte("payload-123")
	if err := os.WriteFile(src, payload, 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := copyThenDelete(src, dst); err != nil {
		t.Fatalf("copyThenDelete: %v", err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("src should be removed, got err=%v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("dst content mismatch: got %q want %q", string(got), string(payload))
	}
}

func TestIsCrossDeviceRename(t *testing.T) {
	if isCrossDeviceRename(pathErr("invalid argument")) {
		t.Fatalf("invalid argument must not be treated as cross-device")
	}
	if !isCrossDeviceRename(pathErr("cross-device link")) {
		t.Fatalf("expected cross-device link text to be detected")
	}
	if !isCrossDeviceRename(pathErr("The system cannot move the file to a different disk drive.")) {
		t.Fatalf("expected different disk drive text to be detected")
	}
	if isCrossDeviceRename(pathErr("access is denied")) {
		t.Fatalf("permission errors must not be treated as cross-device")
	}
}

func pathErr(msg string) error {
	return &os.PathError{Op: "rename", Path: "x", Err: &stringErr{s: msg}}
}

type stringErr struct {
	s string
}

func (e *stringErr) Error() string { return e.s }
