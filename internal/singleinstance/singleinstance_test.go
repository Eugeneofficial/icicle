package singleinstance

import (
	"strconv"
	"strings"
	"testing"
)

func TestLockAddressDeterministicAndInRange(t *testing.T) {
	a := lockAddress("icicle_single_instance_v1")
	b := lockAddress("icicle_single_instance_v1")
	if a != b {
		t.Fatalf("lockAddress must be deterministic: %q != %q", a, b)
	}
	if !strings.HasPrefix(a, "127.0.0.1:") {
		t.Fatalf("unexpected lock address prefix: %q", a)
	}
	portRaw := strings.TrimPrefix(a, "127.0.0.1:")
	port, err := strconv.Atoi(portRaw)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	if port < 49152 || port > 65535 {
		t.Fatalf("port out of expected dynamic range: %d", port)
	}
}
