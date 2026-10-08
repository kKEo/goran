// Package logsink buffers task output and ships it to the server in chunks,
// so a task's log is visible while it runs without one request per line.
package logsink

import (
	"sync"
	"time"
)

// Flusher sends one chunk. A returned error keeps the chunk for the next try.
type Flusher func(chunk string) error

const (
	dropMarker = "\n[goran] log truncated: server unreachable for too long\n"
	hardCap    = 1 << 20 // 1 MiB of unsent output before we start dropping
)

// Sink is an io.Writer that flushes on an interval and on size.
type Sink struct {
	mu      sync.Mutex
	buf     []byte
	flushMu sync.Mutex
	flush   Flusher
	maxBuf  int
	every   time.Duration
	stop    chan struct{}
	done    chan struct{}
	started bool
	once    sync.Once
}

// New creates a sink flushing every interval or whenever maxBuf bytes are pending.
func New(flush Flusher, interval time.Duration, maxBuf int) *Sink {
	if maxBuf <= 0 {
		maxBuf = 8 * 1024
	}
	if interval <= 0 {
		interval = time.Second
	}
	return &Sink{flush: flush, maxBuf: maxBuf, every: interval, stop: make(chan struct{}), done: make(chan struct{})}
}

// Write never fails: output is buffered and shipped asynchronously.
func (s *Sink) Write(p []byte) (int, error) {
	s.mu.Lock()
	s.buf = append(s.buf, p...)
	if len(s.buf) > hardCap {
		keep := s.buf[len(s.buf)-hardCap/2:]
		s.buf = append([]byte(dropMarker), keep...)
	}
	full := len(s.buf) >= s.maxBuf
	s.mu.Unlock()
	if full {
		_ = s.Flush()
	}
	return len(p), nil
}

// Flush ships everything buffered so far. Concurrent flushes are serialised.
func (s *Sink) Flush() error {
	s.flushMu.Lock()
	defer s.flushMu.Unlock()
	s.mu.Lock()
	if len(s.buf) == 0 {
		s.mu.Unlock()
		return nil
	}
	chunk := string(s.buf)
	s.mu.Unlock()
	if err := s.flush(chunk); err != nil {
		return err
	}
	s.mu.Lock()
	s.buf = append([]byte(nil), s.buf[len(chunk):]...)
	s.mu.Unlock()
	return nil
}

// Start begins periodic flushing.
func (s *Sink) Start() {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()
	go func() {
		defer close(s.done)
		t := time.NewTicker(s.every)
		defer t.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-t.C:
				_ = s.Flush()
			}
		}
	}()
}

// Close stops the ticker and performs a final flush.
func (s *Sink) Close() error {
	s.once.Do(func() {
		close(s.stop)
		s.mu.Lock()
		started := s.started
		s.mu.Unlock()
		if started {
			<-s.done
		}
	})
	return s.Flush()
}
