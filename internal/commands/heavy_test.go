package commands

import "testing"

func TestRunHeavyRejectsNegativeLimit(t *testing.T) {
	root := t.TempDir()

	if code := runHeavy([]string{"-n", "-1", root}); code != 2 {
		t.Fatalf("runHeavy(-n -1) code=%d want 2", code)
	}
}
