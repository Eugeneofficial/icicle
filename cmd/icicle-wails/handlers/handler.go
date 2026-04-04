// Package handlers provides Wails backend handlers for the desktop UI.
// Each handler group encapsulates a specific domain of functionality.
package handlers

import (
	"bytes"
	"context"
	"sync"
)

const (
	maxLogBytes  = 2 * 1024 * 1024 // 2MB max log size
	keepLogBytes = 1 * 1024 * 1024 // keep last 1MB when trimming
)

// AppHandler holds shared state and utilities for Wails handlers.
type AppHandler struct {
	Ctx context.Context

	// Log buffer for watch activity
	Mu     sync.Mutex
	LogBuf bytes.Buffer
}

// AppendLog adds a line to the activity log with size limiting.
func (h *AppHandler) AppendLog(line string) {
	h.Mu.Lock()
	defer h.Mu.Unlock()

	// Ensure line ends with newline
	if line[len(line)-1] != '\n' {
		line += "\n"
	}
	h.LogBuf.WriteString(line)

	// Trim from front if too large
	if h.LogBuf.Len() > maxLogBytes {
		full := h.LogBuf.Bytes()
		cutoff := len(full) - keepLogBytes
		if cutoff > 0 {
			// Find next newline to avoid splitting lines
			for cutoff < len(full) && full[cutoff] != '\n' {
				cutoff++
			}
			if cutoff < len(full) {
				cutoff++
			}
			remaining := full[cutoff:]
			h.LogBuf.Reset()
			h.LogBuf.Write(remaining)
		}
	}
}

// ClearLog resets the activity log.
func (h *AppHandler) ClearLog() {
	h.Mu.Lock()
	defer h.Mu.Unlock()
	h.LogBuf.Reset()
}

// WatchLog returns the current activity log.
func (h *AppHandler) WatchLog() string {
	h.Mu.Lock()
	defer h.Mu.Unlock()
	return h.LogBuf.String()
}
