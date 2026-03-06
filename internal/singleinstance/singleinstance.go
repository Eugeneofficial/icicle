package singleinstance

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

var lockListener net.Listener

func lockAddress(name string) string {
	if strings.TrimSpace(name) == "" {
		name = "icicle"
	}
	// Deterministic per-name localhost port in a high range.
	// Avoids collisions with unrelated local listeners better than a single fixed port.
	sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(name))))
	v := binary.BigEndian.Uint16(sum[:2])
	port := 49152 + int(v%16384)
	return "127.0.0.1:" + strconv.Itoa(port)
}

func Acquire(name string) (bool, error) {
	addr := lockAddress(name)
	ln, err := net.Listen("tcp", addr)
	if err == nil {
		lockListener = ln
		return true, nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "only one usage") || strings.Contains(msg, "address already in use") {
		// If something else is listening on this endpoint, treat it as a lock conflict only
		// when it behaves like our lock (accepts a connection on localhost).
		conn, dialErr := net.DialTimeout("tcp", addr, 250*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			return false, nil
		}
		return false, fmt.Errorf("instance lock address conflict at %s: %w", addr, err)
	}
	return false, fmt.Errorf("instance lock listen failed: %w", err)
}

func Release() {
	if lockListener != nil {
		_ = lockListener.Close()
		lockListener = nil
	}
}
