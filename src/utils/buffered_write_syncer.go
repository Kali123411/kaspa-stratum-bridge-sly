package utils

import (
	"bufio"
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/multierr"
	"go.uber.org/zap/zapcore"
)

const (
	// Default buffer size for logging (256 kB)
	_defaultBufferSize = 256 * 1024

	// Default interval for flushing logs
	_defaultFlushInterval = 30 * time.Second
)

var ErrWSFlush = errors.New("unable to flush write stream")

// BufferedWriteSyncer buffers logs in memory and periodically flushes them
// to disk or another write syncer at a defined interval.
type BufferedWriteSyncer struct {
	WS            zapcore.WriteSyncer // Wrapped WriteSyncer (disk, console, etc.)
	Size          int                 // Max buffer size before flushing (defaults to 256 KB)
	FlushInterval time.Duration       // Auto flush interval (defaults to 30 seconds)
	Clock         zapcore.Clock       // Custom clock, defaulting to system clock

	// Internal state
	mu          sync.Mutex
	initialized bool
	stopped     bool
	writer      *bufio.Writer
	ticker      *time.Ticker
	stop        chan struct{}
	done        chan struct{}
}

// initialize sets up the buffer, ticker, and flush loop.
func (s *BufferedWriteSyncer) initialize() {
	if s.Size == 0 {
		s.Size = _defaultBufferSize
	}
	if s.FlushInterval == 0 {
		s.FlushInterval = _defaultFlushInterval
	}
	if s.Clock == nil {
		s.Clock = zapcore.DefaultClock
	}

	s.ticker = s.Clock.NewTicker(s.FlushInterval)
	s.writer = bufio.NewWriterSize(s.WS, s.Size)
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	s.initialized = true
	go s.flushLoop()
}

// Write buffers log data, flushing when full or at set intervals.
func (s *BufferedWriteSyncer) Write(bs []byte) (int, error) {
	tryCount := 0
	for {
		if s.mu.TryLock() {
			break
		}
		if tryCount < 5 {
			time.Sleep(100 * time.Millisecond)
			tryCount++
		} else {
			// Drop log messages if we can't acquire lock
			return len(bs), nil
		}
	}
	defer s.mu.Unlock()

	if !s.initialized {
		s.initialize()
	}

	// Flush buffer if new write won't fit and buffer isn't empty
	if len(bs) > s.writer.Available() && s.writer.Buffered() > 0 {
		if err := s.Flush(); err != nil {
			if errors.Is(err, ErrWSFlush) {
				// Drop log messages if flushing fails
				return len(bs), nil
			}
			return 0, err
		}
	}

	return s.writer.Write(bs)
}

// Flush forces a manual flush of buffered logs with a timeout.
func (s *BufferedWriteSyncer) Flush() error {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	c := make(chan error, 1)

	go func() {
		err := s.writer.Flush()
		select {
		case <-ctx.Done():
			c <- ErrWSFlush
		default:
			c <- err
		}
	}()

	return <-c
}

// Sync flushes logs and ensures data is written to disk.
func (s *BufferedWriteSyncer) Sync() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	if s.initialized {
		err = s.Flush()
	}
	if errors.Is(err, ErrWSFlush) {
		return nil
	}
	return multierr.Append(err, s.WS.Sync())
}

// flushLoop automatically flushes logs at defined intervals.
func (s *BufferedWriteSyncer) flushLoop() {
	defer close(s.done)

	for {
		select {
		case <-s.ticker.C:
			_ = s.Sync() // Ignore errors; bufio stores errors internally
		case <-s.stop:
			return
		}
	}
}

// Stop safely shuts down buffering, ensuring all logs are written.
func (s *BufferedWriteSyncer) Stop() (err error) {
	var alreadyStopped bool

	func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		if !s.initialized {
			return
		}
		if s.stopped {
			alreadyStopped = true
			return
		}
		s.stopped = true

		s.ticker.Stop()
		close(s.stop) // Stop flush loop
		<-s.done      // Wait for loop to exit
	}()

	// Ensure we don't double-call Sync()
	if !alreadyStopped {
		err = s.Sync()
	}

	return err
}
