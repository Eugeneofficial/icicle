package singleinstance

import (
	"fmt"
	"net"
	"strings"
)

var lockListener net.Listener

func lockAddress(name string) string {
	if strings.TrimSpace(name) == "" {
		name = "icicle"
	}
	// Stable local lock port; avoids multi-instance launches.
	return "127.0.0.1:41941"
}

func Acquire(name string) (bool, error) {
	addr := lockAddress(name)
	ln, err := net.Listen("tcp", addr)
	if err == nil {
		lockListener = ln
		return true, nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "only one usage") ||
		strings.Contains(strings.ToLower(err.Error()), "address already in use") {
		return false, nil
	}
	return false, fmt.Errorf("instance lock listen failed: %w", err)
}

func Release() {
	if lockListener != nil {
		_ = lockListener.Close()
		lockListener = nil
	}
}
