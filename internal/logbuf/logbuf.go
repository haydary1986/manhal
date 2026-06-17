// Package logbuf keeps the most recent log lines in memory so the admin web can
// show them — letting a non-technical operator diagnose errors without server
// access. It is an io.Writer attached alongside stderr to the standard logger.
package logbuf

import (
	"strings"
	"sync"
)

const defaultMax = 800

// Default is the process-wide buffer the admin logs page reads.
var Default = New(defaultMax)

// Buffer is a thread-safe ring buffer of recent log lines.
type Buffer struct {
	mu    sync.Mutex
	lines []string
	max   int
}

// New returns a buffer holding up to max lines.
func New(max int) *Buffer {
	if max <= 0 {
		max = defaultMax
	}
	return &Buffer{max: max}
}

// Write appends the (possibly multi-line) log output, trimming to the cap.
func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		b.lines = append(b.lines, line)
	}
	if len(b.lines) > b.max {
		b.lines = b.lines[len(b.lines)-b.max:]
	}
	return len(p), nil
}

// Lines returns a copy of the buffered lines, oldest first.
func (b *Buffer) Lines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.lines))
	copy(out, b.lines)
	return out
}
