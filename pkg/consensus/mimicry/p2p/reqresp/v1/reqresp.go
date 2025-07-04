package v1

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/sirupsen/logrus"
)

// ReqResp implements the Service interface for request/response communication.
type ReqResp struct {
	host     host.Host
	client   Client
	registry *HandlerRegistry
	config   ServiceConfig
	log      logrus.FieldLogger

	mu       sync.RWMutex
	started  bool
	handlers map[protocol.ID]StreamHandler
}

// New creates a new ReqResp service.
func New(h host.Host, config ServiceConfig, log logrus.FieldLogger) *ReqResp {
	if config.HandlerConfig.Encoder == nil || config.ClientConfig.Encoder == nil {
		panic("encoder must be provided in config")
	}

	return &ReqResp{
		host:     h,
		client:   NewClient(h, config.ClientConfig, log),
		registry: NewHandlerRegistry(log),
		config:   config,
		log:      log.WithField("component", "reqresp"),
		handlers: make(map[protocol.ID]StreamHandler),
	}
}

// Start starts the service.
func (r *ReqResp) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return fmt.Errorf("service already started")
	}

	// Register all handlers with the host
	for protoID, handler := range r.handlers {
		r.host.SetStreamHandler(protoID, r.wrapStreamHandler(handler))
		r.log.WithField("protocol", protoID).Info("Registered protocol handler")
	}

	r.started = true
	r.log.Info("ReqResp service started")

	return nil
}

// Stop stops the service.
func (r *ReqResp) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.started {
		return fmt.Errorf("service not started")
	}

	// Remove all handlers from the host
	for protoID := range r.handlers {
		r.host.RemoveStreamHandler(protoID)
		r.log.WithField("protocol", protoID).Debug("Removed protocol handler")
	}

	r.started = false
	r.log.Info("ReqResp service stopped")

	return nil
}

// Register registers a handler for a protocol.
func (r *ReqResp) Register(protocolID protocol.ID, handler StreamHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[protocolID]; exists {
		return ErrHandlerExists
	}

	r.handlers[protocolID] = handler

	// If service is already started, register with host immediately
	if r.started {
		r.host.SetStreamHandler(protocolID, r.wrapStreamHandler(handler))
	}

	return r.registry.Register(protocolID, handler)
}

// Unregister removes a handler for a protocol.
func (r *ReqResp) Unregister(protocolID protocol.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[protocolID]; !exists {
		return ErrNoHandler
	}

	delete(r.handlers, protocolID)

	// If service is running, remove from host
	if r.started {
		r.host.RemoveStreamHandler(protocolID)
	}

	return r.registry.Unregister(protocolID)
}

// SendRequest sends a request to a peer and waits for a response.
func (r *ReqResp) SendRequest(ctx context.Context, peerID peer.ID, protocolID protocol.ID, req any, resp any) error {
	if !r.started {
		return ErrServiceStopped
	}

	return r.client.SendRequest(ctx, peerID, protocolID, req, resp)
}

// SendRequestWithTimeout sends a request with a custom timeout.
func (r *ReqResp) SendRequestWithTimeout(ctx context.Context, peerID peer.ID, protocolID protocol.ID, req any, resp any, timeout time.Duration) error {
	if !r.started {
		return ErrServiceStopped
	}

	return r.client.SendRequestWithTimeout(ctx, peerID, protocolID, req, resp, timeout)
}

// Host returns the underlying libp2p host.
func (r *ReqResp) Host() host.Host {
	return r.host
}

// SupportedProtocols returns the list of registered protocols.
func (r *ReqResp) SupportedProtocols() []protocol.ID {
	r.mu.RLock()
	defer r.mu.RUnlock()

	protocols := make([]protocol.ID, 0, len(r.handlers))
	for protoID := range r.handlers {
		protocols = append(protocols, protoID)
	}

	return protocols
}

// wrapStreamHandler wraps a StreamHandler with logging and metrics.
func (r *ReqResp) wrapStreamHandler(handler StreamHandler) network.StreamHandler {
	return func(stream network.Stream) {
		ctx := context.Background()
		peerID := stream.Conn().RemotePeer()
		protoID := stream.Protocol()

		r.log.WithFields(logrus.Fields{
			"peer":     peerID,
			"protocol": protoID,
		}).Debug("Handling incoming stream")

		// Call the handler
		handler.HandleStream(ctx, stream)
	}
}

// RegisterProtocol is a convenience function to register a protocol with a typed handler.
func RegisterProtocol[TReq, TResp any](
	service *ReqResp,
	protocol Protocol[TReq, TResp],
	handler RequestHandler[TReq, TResp],
) error {
	h := NewHandler(protocol, handler, service.config.HandlerConfig, service.log)

	return service.Register(protocol.ID(), h)
}

// RegisterChunkedProtocol is a convenience function to register a chunked protocol with a typed handler.
func RegisterChunkedProtocol[TReq, TResp any](
	service *ReqResp,
	protocol Protocol[TReq, TResp],
	handler ChunkedRequestHandler[TReq, TResp],
) error {
	h := NewChunkedHandler(protocol, handler, service.config.HandlerConfig, service.log)

	return service.Register(protocol.ID(), h)
}
