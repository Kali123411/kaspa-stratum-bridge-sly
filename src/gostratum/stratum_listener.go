package gostratum

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type DisconnectChannel chan *StratumContext
type StateGenerator func() any
type EventHandler func(ctx *StratumContext, event JsonRpcEvent) error

type StratumClientListener interface {
	OnConnect(ctx *StratumContext)
	OnDisconnect(ctx *StratumContext)
}

type StratumHandlerMap map[string]EventHandler

type StratumStats struct {
	Disconnects int64
}

type StratumListenerConfig struct {
	Logger         *zap.Logger
	HandlerMap     StratumHandlerMap
	ClientListener StratumClientListener
	StateGenerator StateGenerator
	Port           string
}

type StratumListener struct {
	StratumListenerConfig
	shuttingDown      int32 // Use atomic flag instead of bool
	disconnectChannel DisconnectChannel
	stats             StratumStats
	workerGroup       sync.WaitGroup
}

func NewListener(cfg StratumListenerConfig) *StratumListener {
	listener := &StratumListener{
		StratumListenerConfig: cfg,
		workerGroup:           sync.WaitGroup{},
		disconnectChannel:     make(DisconnectChannel),
	}

	listener.Logger = listener.Logger.With(
		zap.String("component", "stratum"),
		zap.String("address", listener.Port),
	)

	if listener.StateGenerator == nil {
		listener.Logger.Warn("no state generator provided, using default")
		listener.StateGenerator = func() any { return nil }
	}

	return listener
}

func (s *StratumListener) Listen(ctx context.Context) error {
	atomic.StoreInt32(&s.shuttingDown, 0) // Reset shutdown flag

	serverContext, cancel := context.WithCancel(ctx)
	defer cancel()

	lc := net.ListenConfig{}
	server, err := lc.Listen(ctx, "tcp", s.Port)
	if err != nil {
		return errors.Wrapf(err, "failed listening to socket %s", s.Port)
	}
	defer server.Close()

	s.workerGroup.Add(2) // Ensure both listeners are accounted for
	go s.disconnectListener(serverContext)
	go s.tcpListener(serverContext, server)

	// Wait until the context is cancelled
	<-ctx.Done()

	// Graceful shutdown
	atomic.StoreInt32(&s.shuttingDown, 1)
	server.Close()
	s.workerGroup.Wait()
	return context.Canceled
}

func (s *StratumListener) newClient(ctx context.Context, connection net.Conn) {
	addr := connection.RemoteAddr().String()
	port := connection.RemoteAddr().(*net.TCPAddr).Port
	parts := strings.Split(addr, ":")
	if len(parts) > 0 {
		addr = parts[0] // Trim off the port
	}

	clientContext := &StratumContext{
		parentContext: ctx,
		RemoteAddr:    addr,
		RemotePort:    port,
		Logger:        s.Logger.With(zap.String("client", addr)),
		connection:    connection,
		State:         s.StateGenerator(),
		onDisconnect:  s.disconnectChannel,
	}

	s.Logger.Info(fmt.Sprintf("new client connecting - %s", addr))

	if s.ClientListener != nil {
		s.ClientListener.OnConnect(clientContext)
	}

	go spawnClientListener(clientContext, connection, s)
}

func (s *StratumListener) HandleEvent(ctx *StratumContext, event JsonRpcEvent) error {
	if handler, exists := s.HandlerMap[string(event.Method)]; exists {
		return handler(ctx, event)
	}
	return nil
}

func (s *StratumListener) disconnectListener(ctx context.Context) {
	defer s.workerGroup.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case client, ok := <-s.disconnectChannel:
			if !ok {
				return // Channel closed
			}
			s.Logger.Info(fmt.Sprintf("client disconnecting - %s", client.RemoteAddr))
			atomic.AddInt64(&s.stats.Disconnects, 1)
			if s.ClientListener != nil {
				s.ClientListener.OnDisconnect(client)
			}
		}
	}
}

func (s *StratumListener) tcpListener(ctx context.Context, server net.Listener) {
	defer s.workerGroup.Done()

	for {
		connection, err := server.Accept()
		if err != nil {
			if atomic.LoadInt32(&s.shuttingDown) == 1 {
				s.Logger.Info("server shutting down, stopping listener")
				return
			}
			s.Logger.Error("failed to accept incoming connection", zap.Error(err))
			continue
		}
		s.newClient(ctx, connection)
	}
}
