package gostratum

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// spawnClientListener listens for incoming client messages and processes them
func spawnClientListener(ctx *StratumContext, connection net.Conn, s *StratumListener) error {
	defer ctx.Disconnect()

	for {
		err := readFromConnection(connection, func(line string) error {
			// ctx.Logger.Info("client message: ", zap.String("raw", line))

			// Unmarshal the JSON-RPC event
			event, err := UnmarshalEvent(line)
			if err != nil {
				ctx.Logger.Error("error unmarshalling event", zap.String("raw", line))
				return err
			}

			// Process the event
			return s.HandleEvent(ctx, event)
		})

		// Handle timeout error correctly
		if errors.Is(err, os.ErrDeadlineExceeded) {
			continue // Expected timeout, retry read loop
		}

		// Check if the context has been cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if ctx.parentContext.Err() != nil {
			return ctx.parentContext.Err()
		}

		// Handle unexpected errors
		if err != nil {
			if errors.Is(err, io.EOF) {
				ctx.Logger.Info("client disconnected")
				return nil // Graceful exit on disconnect
			}
			ctx.Logger.Error("error reading from socket", zap.Error(err))
			return err
		}
	}
}

// LineCallback defines the function signature for handling each received line
type LineCallback func(line string) error

// readFromConnection reads incoming data from the connection and processes each line
func readFromConnection(connection net.Conn, cb LineCallback) error {
	// Set a read deadline
	deadline := time.Now().Add(5 * time.Second).UTC()
	if err := connection.SetReadDeadline(deadline); err != nil {
		return errors.Wrap(err, "failed to set read deadline")
	}

	reader := bufio.NewReader(connection) // Buffered reader for efficiency

	for {
		// Read until newline
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, net.ErrDeadlineExceeded) {
				return err // Handle timeout gracefully
			}
			if errors.Is(err, io.EOF) {
				return err // Client disconnected
			}
			return errors.Wrap(err, "error reading from connection")
		}

		// Clean up the received data
		line = strings.TrimSpace(line)
		line = strings.ReplaceAll(line, "\x00", "") // Remove null bytes

		// Process the line using callback
		if err := cb(line); err != nil {
			return err
		}
	}
}
