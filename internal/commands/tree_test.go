package commands

import "testing"

func TestRunTreeRejectsNegativeFlags(t *testing.T) {
	root := t.TempDir()

	if code := runTree([]string{"-w", "-1", root}); code != 2 {
		t.Fatalf("runTree(-w -1) code=%d want 2", code)
	}
	if code := runTree([]string{"-n", "-1", root}); code != 2 {
		t.Fatalf("runTree(-n -1) code=%d want 2", code)
	}
	if code := runTree([]string{"-top", "-1", root}); code != 2 {
		t.Fatalf("runTree(-top -1) code=%d want 2", code)
	}
}
