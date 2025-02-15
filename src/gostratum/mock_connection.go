package gostratum

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// MockConnection simulates a network connection for testing purposes
type MockConnection struct {
	id           string
	lock         sync.Mutex // Prevents double closing of channels
	inChan       chan []byte
	outChan      chan []byte
	closed       int32 // Atomic flag to track if the connection is closed
	closeOnce    sync.Once // Ensures channels are closed only once
	readTimeout  time.Time
	writeTimeout time.Time
}

var channelCounter int32

// NewMockConnection creates a new mock connection
func NewMockConnection() *MockConnection {
	return &MockConnection{
		id:      fmt.Sprintf("mc_%d", atomic.AddInt32(&channelCounter, 1)),
		inChan:  make(chan []byte, 10),  // Buffered to prevent blocking
		outChan: make(chan []byte, 10), // Buffered to prevent blocking
	}
}

// AsyncWriteTestDataToReadBuffer writes test data asynchronously
func (mc *MockConnection) AsyncWriteTestDataToReadBuffer(s string) {
	go func() {
		mc.inChan <- []byte(s)
	}()
}

// ReadTestDataFromBuffer reads test data from the output buffer
func (mc *MockConnection) ReadTestDataFromBuffer(handler func([]byte)) {
	read, ok := <-mc.outChan
	if ok {
		handler(read)
	}
}

// AsyncReadTestDataFromBuffer asynchronously reads data
func (mc *MockConnection) AsyncReadTestDataFromBuffer(handler func([]byte)) {
	go func() {
		read, ok := <-mc.outChan
		if ok {
			handler(read)
		}
	}()
}

// Read reads data from the mock connection
func (mc *MockConnection) Read(b []byte) (int, error) {
	select {
	case data, ok := <-mc.inChan:
		if !ok {
			return 0, context.DeadlineExceeded
		}
		return copy(b, data), nil
	case <-time.After(time.Until(mc.readTimeout)): // Simulate timeout behavior
		return 0, context.DeadlineExceeded
	}
}

// Write writes data to the mock connection
func (mc *MockConnection) Write(b []byte) (int, error) {
	select {
	case mc.outChan <- b:
		return len(b), nil
	case <-time.After(time.Until(mc.writeTimeout)): // Simulate timeout behavior
		return 0, context.DeadlineExceeded
	}
}

// Close safely closes the connection, ensuring channels are closed only once
func (mc *MockConnection) Close() error {
	mc.closeOnce.Do(func() {
		mc.lock.Lock()
		defer mc.lock.Unlock()
		atomic.StoreInt32(&mc.closed, 1)
		close(mc.inChan)
		close(mc.outChan)
	})
	return nil
}

// MockAddr represents a fake network address for testing
type MockAddr struct {
	id string
}

func (ma MockAddr) Network() string { return "mock" }
func (ma MockAddr) String() string  { return ma.id }

// LocalAddr returns the local address of the mock connection
func (mc *MockConnection) LocalAddr() net.Addr {
	return MockAddr{id: mc.id}
}

// RemoteAddr returns the remote address of the mock connection
func (mc *MockConnection) RemoteAddr() net.Addr {
	return MockAddr{id: mc.id}
}

// SetDeadline sets both read and write deadlines
func (mc *MockConnection) SetDeadline(t time.Time) error {
	mc.SetReadDeadline(t)
	mc.SetWriteDeadline(t)
	return nil
}

// SetReadDeadline sets the read deadline
func (mc *MockConnection) SetReadDeadline(t time.Time) error {
	mc.lock.Lock()
	defer mc.lock.Unlock()
	mc.readTimeout = t
	return nil
}

// SetWriteDeadline sets the write deadline
func (mc *MockConnection) SetWriteDeadline(t time.Time) error {
	mc.lock.Lock()
	defer mc.lock.Unlock()
	mc.writeTimeout = t
	return nil
}
