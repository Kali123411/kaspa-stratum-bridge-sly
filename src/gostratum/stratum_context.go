package gostratum

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type StratumContext struct {
	parentContext context.Context
	RemoteAddr    string
	RemotePort    int
	WalletAddr    string
	WorkerName    string
	RemoteApp     string
	Id            int32
	Logger        *zap.Logger
	connection    net.Conn
	disconnecting uint32 // Use atomic flag for safe updates
	onDisconnect  chan *StratumContext
	State         any
	writeLock     int32
	Extranonce    string
}

type ContextSummary struct {
	RemoteAddr string
	RemotePort int
	WalletAddr string
	WorkerName string
	RemoteApp  string
}

var ErrorDisconnected = fmt.Errorf("disconnecting")

func (sc *StratumContext) Connected() bool {
	return atomic.LoadUint32(&sc.disconnecting) == 0
}

func (sc *StratumContext) Summary() ContextSummary {
	return ContextSummary{
		RemoteAddr: sc.RemoteAddr,
		RemotePort: sc.RemotePort,
		WalletAddr: sc.WalletAddr,
		WorkerName: sc.WorkerName,
		RemoteApp:  sc.RemoteApp,
	}
}

func NewMockContext(ctx context.Context, logger *zap.Logger, state any) (*StratumContext, *MockConnection) {
	mc := NewMockConnection()
	return &StratumContext{
		parentContext: ctx,
		State:         state,
		RemoteAddr:    "127.0.0.1",
		RemotePort:    50001,
		WalletAddr:    uuid.NewString(),
		WorkerName:    uuid.NewString(),
		RemoteApp:     "mock.context",
		Logger:        logger,
		connection:    mc,
	}, mc
}

func (sc *StratumContext) String() string {
	serialized, _ := json.Marshal(sc)
	return string(serialized)
}

func (sc *StratumContext) Reply(response JsonRpcResponse) error {
	if atomic.LoadUint32(&sc.disconnecting) == 1 {
		return ErrorDisconnected
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return errors.Wrap(err, "failed encoding jsonrpc response")
	}
	encoded = append(encoded, '\n')
	return sc.writeWithBackoff(encoded)
}

func (sc *StratumContext) Send(event JsonRpcEvent) error {
	if atomic.LoadUint32(&sc.disconnecting) == 1 {
		return ErrorDisconnected
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return errors.Wrap(err, "failed encoding jsonrpc event")
	}
	encoded = append(encoded, '\n')
	return sc.writeWithBackoff(encoded)
}

var errWriteBlocked = fmt.Errorf("error writing to socket, previous write pending")

func (sc *StratumContext) write(data []byte) error {
	if atomic.CompareAndSwapInt32(&sc.writeLock, 0, 1) {
		defer atomic.StoreInt32(&sc.writeLock, 0)
		deadline := time.Now().Add(5 * time.Second)
		if err := sc.connection.SetWriteDeadline(deadline); err != nil {
			return errors.Wrap(err, "failed setting write deadline for connection")
		}
		_, err := sc.connection.Write(data)
		sc.checkDisconnect(err)
		return err
	}
	return errWriteBlocked
}

func (sc *StratumContext) writeWithBackoff(data []byte) error {
	delay := 5 * time.Millisecond
	for i := 0; i < 3; i++ {
		err := sc.write(data)
		if err == nil {
			return nil
		} else if err == errWriteBlocked {
			time.Sleep(delay)
			delay *= 2 // Exponential backoff
			continue
		} else {
			return err
		}
	}
	return fmt.Errorf("failed writing to socket after 3 attempts")
}

func (sc *StratumContext) ReplyStaleShare(id any) error {
	return sc.Reply(JsonRpcResponse{
		Id:     id,
		Result: nil,
		Error:  []any{21, "Job not found", nil},
	})
}
func (sc *StratumContext) ReplyDupeShare(id any) error {
	return sc.Reply(JsonRpcResponse{
		Id:     id,
		Result: nil,
		Error:  []any{22, "Duplicate share submitted", nil},
	})
}

func (sc *StratumContext) ReplyBadShare(id any) error {
	return sc.Reply(JsonRpcResponse{
		Id:     id,
		Result: nil,
		Error:  []any{20, "Unknown problem", nil},
	})
}

func (sc *StratumContext) ReplyLowDiffShare(id any) error {
	return sc.Reply(JsonRpcResponse{
		Id:     id,
		Result: nil,
		Error:  []any{23, "Invalid difficulty", nil},
	})
}

func (sc *StratumContext) Disconnect() {
	if atomic.CompareAndSwapUint32(&sc.disconnecting, 0, 1) {
		sc.Logger.Info("disconnecting")
		if sc.connection != nil {
			sc.connection.Close()
		}
		if sc.onDisconnect != nil { // Prevent nil-channel panic
			sc.onDisconnect <- sc
		}
	}
}

func (sc *StratumContext) checkDisconnect(err error) {
	if err != nil {
		sc.Logger.Error("connection error, disconnecting", zap.Error(err))
		go sc.Disconnect()
	}
}

// Context interface impl

func (StratumContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (StratumContext) Done() <-chan struct{} {
	return nil
}

func (StratumContext) Err() error {
	return nil
}

func (d StratumContext) Value(key any) any {
	return d.parentContext.Value(key)
}
