package mimicry

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chuckpreslar/emission"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/rlpx"
	"github.com/sirupsen/logrus"
)

// ServerPeer represents a single inbound RLPx connection.
type ServerPeer struct {
	log    logrus.FieldLogger
	server *Server

	remoteID     enode.ID
	remotePubkey *ecdsa.PublicKey

	conn     net.Conn
	rlpxConn *rlpx.Conn

	ethCapVersion uint
	remoteHello   *Hello

	pooledTransactionsMap map[uint64]chan *PooledTransactions

	done   chan struct{}
	closed atomic.Bool
}

// Server accepts incoming RLPx connections.
type Server struct {
	log    logrus.FieldLogger
	config *ServerConfig
	broker *emission.Emitter

	privateKey *ecdsa.PrivateKey

	listener net.Listener

	peers   map[enode.ID]*ServerPeer
	peersMu sync.RWMutex

	wg     sync.WaitGroup
	closed atomic.Bool
	done   chan struct{}
}

// NewServer creates a new Server with the given configuration.
func NewServer(log logrus.FieldLogger, config *ServerConfig) (*Server, error) {
	if config.StatusProvider == nil {
		return nil, errors.New("StatusProvider is required")
	}

	if config.ListenAddr == "" {
		return nil, errors.New("ListenAddr is required")
	}

	// Apply defaults
	if config.HandshakeTimeout == 0 {
		config.HandshakeTimeout = 5 * time.Second
	}

	if config.ReadTimeout == 0 {
		config.ReadTimeout = 30 * time.Second
	}

	privateKey := config.PrivateKey
	if privateKey == nil {
		var err error

		privateKey, err = crypto.GenerateKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate private key: %w", err)
		}
	}

	return &Server{
		log:        log.WithField("component", "execution_mimicry_server"),
		config:     config,
		broker:     emission.NewEmitter(),
		privateKey: privateKey,
		peers:      make(map[enode.ID]*ServerPeer, 64),
		done:       make(chan struct{}),
	}, nil
}

// Start begins listening for incoming connections.
func (s *Server) Start(ctx context.Context) error {
	s.log.WithField("addr", s.config.ListenAddr).Info("starting execution mimicry server")

	var err error

	s.listener, err = net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.log.WithField("addr", s.listener.Addr().String()).Info("listening for connections")

	s.wg.Add(1)

	go s.acceptLoop(ctx)

	return nil
}

// Stop stops the server and disconnects all peers.
func (s *Server) Stop(ctx context.Context) error {
	if s.closed.Swap(true) {
		return nil // Already closed
	}

	s.log.Info("stopping execution mimicry server")

	close(s.done)

	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			s.log.WithError(err).Warn("error closing listener")
		}
	}

	// Close all peers
	s.peersMu.Lock()

	for _, peer := range s.peers {
		peer.Close()
	}

	s.peersMu.Unlock()

	// Wait for goroutines with timeout
	done := make(chan struct{})

	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.log.Debug("all goroutines stopped")
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		s.log.Warn("shutdown timeout, some goroutines may still be running")
	}

	return nil
}

// acceptLoop accepts incoming connections.
func (s *Server) acceptLoop(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.done:
			return
		default:
		}

		// Check max peers
		if s.config.MaxPeers > 0 {
			s.peersMu.RLock()
			peerCount := len(s.peers)
			s.peersMu.RUnlock()

			if peerCount >= s.config.MaxPeers {
				time.Sleep(100 * time.Millisecond)

				continue
			}
		}

		conn, err := s.listener.Accept()
		if err != nil {
			if s.closed.Load() {
				return
			}

			s.log.WithError(err).Warn("accept error")

			continue
		}

		s.wg.Add(1)

		go s.handleConnection(ctx, conn)
	}
}

// handleConnection handles a single incoming connection.
func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer s.wg.Done()

	remoteAddr := conn.RemoteAddr().String()
	log := s.log.WithField("remote_addr", remoteAddr)

	log.Debug("new incoming connection")

	peer := &ServerPeer{
		log:                   log,
		server:                s,
		conn:                  conn,
		pooledTransactionsMap: make(map[uint64]chan *PooledTransactions, 16),
		done:                  make(chan struct{}),
	}

	// Perform RLPx handshake (responder mode)
	if err := peer.handshake(ctx, s.config.HandshakeTimeout); err != nil {
		log.WithError(err).Debug("handshake failed")
		conn.Close()

		return
	}

	// Check if peer already exists
	s.peersMu.Lock()

	if existing, ok := s.peers[peer.remoteID]; ok {
		s.peersMu.Unlock()
		log.WithField("peer_id", peer.remoteID.String()).Debug("peer already connected, closing new connection")
		existing.Close()
		peer.Close()

		return
	}

	s.peers[peer.remoteID] = peer
	s.peersMu.Unlock()

	// Emit peer connected event
	s.publishPeerConnected(peer)

	// Run session
	if err := peer.startSession(ctx); err != nil {
		log.WithError(err).Debug("peer session ended")
	}

	// Unregister peer
	s.peersMu.Lock()
	delete(s.peers, peer.remoteID)
	s.peersMu.Unlock()

	// Emit peer disconnected event
	s.publishPeerDisconnected(peer)
}

// isNetworkAllowed checks if Status matches allowed networks.
func (s *Server) isNetworkAllowed(status Status) bool {
	// No filter = allow all
	if len(s.config.AllowedNetworks) == 0 {
		return true
	}

	// Must match at least one allowed network
	for _, network := range s.config.AllowedNetworks {
		if network.Matches(status) {
			return true
		}
	}

	return false
}

// Peers returns all connected peers.
func (s *Server) Peers() []*ServerPeer {
	s.peersMu.RLock()
	defer s.peersMu.RUnlock()

	peers := make([]*ServerPeer, 0, len(s.peers))
	for _, p := range s.peers {
		peers = append(peers, p)
	}

	return peers
}

// PeerCount returns the number of connected peers.
func (s *Server) PeerCount() int {
	s.peersMu.RLock()
	defer s.peersMu.RUnlock()

	return len(s.peers)
}

// Broker returns the event broker.
func (s *Server) Broker() *emission.Emitter {
	return s.broker
}

// ServerPeer methods

// handshake performs the RLPx handshake in responder mode.
func (sp *ServerPeer) handshake(ctx context.Context, timeout time.Duration) error {
	if err := sp.conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("error setting handshake deadline: %w", err)
	}

	// nil pubkey = responder mode
	sp.rlpxConn = rlpx.NewConn(sp.conn, nil)

	remotePubkey, err := sp.rlpxConn.Handshake(sp.server.privateKey)
	if err != nil {
		return fmt.Errorf("rlpx handshake failed: %w", err)
	}

	sp.remotePubkey = remotePubkey
	sp.remoteID = enode.PubkeyToIDV4(remotePubkey)

	// Clear deadline
	if err := sp.conn.SetDeadline(time.Time{}); err != nil {
		return fmt.Errorf("error clearing deadline: %w", err)
	}

	sp.log = sp.log.WithField("peer_id", sp.remoteID.String())
	sp.log.Debug("rlpx handshake complete")

	return nil
}

// startSession runs the peer session (Hello exchange + message loop).
func (sp *ServerPeer) startSession(ctx context.Context) error {
	defer sp.Close()

	// Send Hello first
	if err := sp.sendHello(ctx); err != nil {
		return fmt.Errorf("error sending hello: %w", err)
	}

	// Enter message loop
	return sp.messageLoop(ctx)
}

// messageLoop handles incoming messages.
func (sp *ServerPeer) messageLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-sp.done:
			return nil
		default:
		}

		// Set read deadline
		if err := sp.conn.SetReadDeadline(time.Now().Add(sp.server.config.ReadTimeout)); err != nil {
			return fmt.Errorf("error setting read deadline: %w", err)
		}

		code, data, _, err := sp.rlpxConn.Read()
		if err != nil {
			return fmt.Errorf("error reading: %w", err)
		}

		if err := sp.handleMessage(ctx, code, data); err != nil {
			return err
		}
	}
}

// handleMessage routes messages to appropriate handlers.
func (sp *ServerPeer) handleMessage(ctx context.Context, code uint64, data []byte) error {
	//nolint:gosec // not an overflow issue here.
	switch int(code) {
	case HelloCode:
		return sp.handleHello(ctx, code, data)
	case DisconnectCode:
		sp.handleDisconnect(ctx, code, data)

		return nil
	case PingCode:
		return sp.handlePing(ctx, code, data)
	case StatusCode:
		return sp.handleStatus(ctx, code, data)
	case TransactionsCode:
		return sp.handleTransactions(ctx, code, data)
	case GetBlockHeadersCode:
		return sp.handleGetBlockHeaders(ctx, code, data)
	case BlockHeadersCode:
		return sp.handleBlockHeaders(ctx, code, data)
	case GetBlockBodiesCode:
		return sp.handleGetBlockBodies(ctx, code, data)
	case NewPooledTransactionHashesCode:
		return sp.handleNewPooledTransactionHashes(ctx, code, data)
	case PooledTransactionsCode:
		return sp.handlePooledTransactions(ctx, code, data)
	case GetReceiptsCode:
		return sp.handleGetReceipts(ctx, code, data)
	case BlockRangeUpdateCode:
		return sp.handleBlockRangeUpdate(ctx, code, data)
	default:
		sp.log.WithField("code", code).Debug("received unhandled message code")
	}

	return nil
}

// Close terminates the peer connection.
func (sp *ServerPeer) Close() error {
	if sp.closed.Swap(true) {
		return nil // Already closed
	}

	close(sp.done)

	if sp.rlpxConn != nil {
		if err := sp.rlpxConn.Close(); err != nil {
			sp.log.WithError(err).Debug("error closing rlpx connection")
		}
	}

	return nil
}

// RemoteID returns the remote node ID.
func (sp *ServerPeer) RemoteID() enode.ID {
	return sp.remoteID
}

// RemotePubkey returns the remote public key.
func (sp *ServerPeer) RemotePubkey() *ecdsa.PublicKey {
	return sp.remotePubkey
}

// Message handlers for ServerPeer

func (sp *ServerPeer) sendHello(ctx context.Context) error {
	sp.log.WithField("code", HelloCode).Debug("sending Hello")

	encodedData, err := encodeHello(sp.server.privateKey, SupportedEthCaps())
	if err != nil {
		return fmt.Errorf("error encoding hello: %w", err)
	}

	if _, err := sp.rlpxConn.Write(HelloCode, encodedData); err != nil {
		return fmt.Errorf("error sending hello: %w", err)
	}

	return nil
}

func (sp *ServerPeer) handleHello(ctx context.Context, code uint64, data []byte) error {
	sp.log.WithField("code", code).Debug("received Hello")

	hello, err := decodeHello(data)
	if err != nil {
		return err
	}

	sp.log.WithFields(logrus.Fields{
		"version": hello.Version,
		"caps":    hello.Caps,
		"name":    hello.Name,
	}).Debug("received hello message")

	if err := hello.Validate(); err != nil {
		return err
	}

	sp.ethCapVersion = hello.ETHProtocolVersion()
	sp.remoteHello = hello

	sp.server.publishHello(ctx, sp, hello)

	// Enable snappy compression
	sp.rlpxConn.SetSnappy(true)

	return nil
}

func (sp *ServerPeer) handleStatus(ctx context.Context, code uint64, data []byte) error {
	sp.log.WithFields(logrus.Fields{
		"code":          code,
		"ethCapVersion": sp.ethCapVersion,
	}).Debug("received Status")

	status, err := decodeStatus(data, sp.ethCapVersion)
	if err != nil {
		return err
	}

	// Check network filter BEFORE publishing/responding
	if !sp.server.isNetworkAllowed(status) {
		sp.log.WithFields(logrus.Fields{
			"network_id": status.GetNetworkID(),
			"genesis":    hex.EncodeToString(status.GetGenesis()[:8]),
		}).Debug("peer rejected: network not allowed")

		return fmt.Errorf("network not allowed")
	}

	sp.server.publishStatus(ctx, sp, status)

	// Use StatusProvider callback for response
	responseStatus, err := sp.server.config.StatusProvider(sp.remoteHello)
	if err != nil {
		return fmt.Errorf("status provider error: %w", err)
	}

	return sp.sendStatus(ctx, responseStatus)
}

func (sp *ServerPeer) sendStatus(ctx context.Context, status Status) error {
	sp.log.WithFields(logrus.Fields{
		"code":          StatusCode,
		"ethCapVersion": sp.ethCapVersion,
	}).Debug("sending Status")

	encodedData, err := encodeStatus(status)
	if err != nil {
		return fmt.Errorf("error encoding status: %w", err)
	}

	if _, err := sp.rlpxConn.Write(StatusCode, encodedData); err != nil {
		return fmt.Errorf("error sending status: %w", err)
	}

	return nil
}

func (sp *ServerPeer) handleDisconnect(ctx context.Context, code uint64, data []byte) {
	sp.log.WithField("code", code).Debug("received Disconnect")

	disconnect, err := decodeDisconnect(data)
	if err != nil {
		sp.log.WithError(err).Debug("error decoding disconnect")

		return
	}

	sp.server.publishDisconnect(ctx, sp, disconnect)
}

func (sp *ServerPeer) handlePing(ctx context.Context, code uint64, data []byte) error {
	sp.log.WithField("code", code).Debug("received Ping")

	// Respond with Pong
	if _, err := sp.rlpxConn.Write(PongCode, []byte{}); err != nil {
		return fmt.Errorf("error sending pong: %w", err)
	}

	return nil
}

func (sp *ServerPeer) handleTransactions(ctx context.Context, code uint64, data []byte) error {
	sp.log.WithField("code", code).Debug("received Transactions")

	transactions, err := decodeTransactions(data)
	if err != nil {
		return err
	}

	sp.server.publishTransactions(ctx, sp, transactions)

	return nil
}

func (sp *ServerPeer) handleGetBlockHeaders(ctx context.Context, code uint64, data []byte) error {
	sp.log.WithField("code", code).Debug("received GetBlockHeaders")
	// Server doesn't respond to block header requests

	return nil
}

func (sp *ServerPeer) handleBlockHeaders(ctx context.Context, code uint64, data []byte) error {
	sp.log.WithField("code", code).Debug("received BlockHeaders")

	return nil
}

func (sp *ServerPeer) handleGetBlockBodies(ctx context.Context, code uint64, data []byte) error {
	sp.log.WithField("code", code).Debug("received GetBlockBodies")
	// Server doesn't respond to block body requests

	return nil
}

func (sp *ServerPeer) handleNewPooledTransactionHashes(ctx context.Context, code uint64, data []byte) error {
	sp.log.WithField("code", code).Debug("received NewPooledTransactionHashes")

	hashes, err := decodeNewPooledTransactionHashes(data)
	if err != nil {
		return err
	}

	sp.server.publishNewPooledTransactionHashes(ctx, sp, hashes)

	return nil
}

func (sp *ServerPeer) handlePooledTransactions(ctx context.Context, code uint64, data []byte) error {
	sp.log.WithField("code", code).Debug("received PooledTransactions")

	return nil
}

func (sp *ServerPeer) handleGetReceipts(ctx context.Context, code uint64, data []byte) error {
	sp.log.WithField("code", code).Debug("received GetReceipts")
	// Server doesn't respond to receipt requests

	return nil
}

func (sp *ServerPeer) handleBlockRangeUpdate(ctx context.Context, code uint64, data []byte) error {
	sp.log.WithField("code", code).Debug("received BlockRangeUpdate")

	return nil
}
