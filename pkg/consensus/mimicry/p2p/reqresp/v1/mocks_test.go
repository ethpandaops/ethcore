package v1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/connmgr"
	ic "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	ma "github.com/multiformats/go-multiaddr"
)

// mockHost implements a mock libp2p host for testing.
type mockHost struct {
	mu            sync.RWMutex
	id            peer.ID
	streams       map[string]*mockStream
	handlers      map[protocol.ID]network.StreamHandler
	newStreamFunc func(ctx context.Context, p peer.ID, pids ...protocol.ID) (network.Stream, error)
}

func newMockHost(id peer.ID) *mockHost {
	return &mockHost{
		id:       id,
		streams:  make(map[string]*mockStream),
		handlers: make(map[protocol.ID]network.StreamHandler),
	}
}

func (h *mockHost) ID() peer.ID {
	return h.id
}

func (h *mockHost) Peerstore() peerstore.Peerstore {
	return nil
}

func (h *mockHost) Addrs() []ma.Multiaddr {
	return nil
}

func (h *mockHost) Network() network.Network {
	return nil
}

func (h *mockHost) Mux() protocol.Switch {
	return nil
}

func (h *mockHost) Connect(ctx context.Context, pi peer.AddrInfo) error {
	return nil
}

func (h *mockHost) SetStreamHandler(pid protocol.ID, handler network.StreamHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handlers[pid] = handler
}

func (h *mockHost) SetStreamHandlerMatch(pid protocol.ID, m func(protocol.ID) bool, handler network.StreamHandler) {
	h.SetStreamHandler(pid, handler)
}

func (h *mockHost) RemoveStreamHandler(pid protocol.ID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.handlers, pid)
}

func (h *mockHost) NewStream(ctx context.Context, p peer.ID, pids ...protocol.ID) (network.Stream, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.newStreamFunc != nil {
		return h.newStreamFunc(ctx, p, pids...)
	}

	if len(pids) == 0 {
		return nil, errors.New("no protocol specified")
	}

	streamID := fmt.Sprintf("%s-%s-%s", h.id, p, pids[0])
	stream := newMockStream(streamID, pids[0], h.id, p)
	h.streams[streamID] = stream

	return stream, nil
}

func (h *mockHost) Close() error {
	return nil
}

func (h *mockHost) ConnManager() connmgr.ConnManager {
	return nil
}

func (h *mockHost) EventBus() event.Bus {
	return nil
}

// mockStream implements a mock network stream for testing.
type mockStream struct {
	mu            sync.RWMutex
	id            string
	protocol      protocol.ID
	localPeer     peer.ID
	remotePeer    peer.ID
	readBuffer    []byte
	writeBuffer   []byte
	readClosed    bool
	writeClosed   bool
	resetErr      error
	closeErr      error
	deadline      time.Time
	readDeadline  time.Time
	writeDeadline time.Time
	stat          network.Stats
}

func newMockStream(id string, proto protocol.ID, local, remote peer.ID) *mockStream {
	return &mockStream{
		id:         id,
		protocol:   proto,
		localPeer:  local,
		remotePeer: remote,
	}
}

func (s *mockStream) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.readClosed {
		return 0, io.EOF
	}

	if s.resetErr != nil {
		return 0, s.resetErr
	}

	if len(s.readBuffer) == 0 {
		return 0, io.EOF
	}

	n := copy(p, s.readBuffer)
	s.readBuffer = s.readBuffer[n:]

	return n, nil
}

func (s *mockStream) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.writeClosed {
		return 0, errors.New("write on closed stream")
	}

	if s.resetErr != nil {
		return 0, s.resetErr
	}

	s.writeBuffer = append(s.writeBuffer, p...)

	return len(p), nil
}

func (s *mockStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closeErr != nil {
		return s.closeErr
	}

	s.readClosed = true
	s.writeClosed = true

	return nil
}

func (s *mockStream) CloseRead() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.readClosed = true

	return nil
}

func (s *mockStream) CloseWrite() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.writeClosed = true

	return nil
}

func (s *mockStream) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resetErr = network.ErrReset
	s.readClosed = true
	s.writeClosed = true

	return nil
}

func (s *mockStream) ResetWithError(errCode network.StreamErrorCode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Convert error code to error
	s.resetErr = fmt.Errorf("stream reset with error code: %d", errCode)
	s.readClosed = true
	s.writeClosed = true

	return nil
}

func (s *mockStream) SetDeadline(t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.deadline = t
	s.readDeadline = t
	s.writeDeadline = t

	return nil
}

func (s *mockStream) SetReadDeadline(t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.readDeadline = t

	return nil
}

func (s *mockStream) SetWriteDeadline(t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.writeDeadline = t

	return nil
}

func (s *mockStream) ID() string {
	return s.id
}

func (s *mockStream) Protocol() protocol.ID {
	return s.protocol
}

func (s *mockStream) SetProtocol(id protocol.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.protocol = id

	return nil
}

func (s *mockStream) Stat() network.Stats {
	return s.stat
}

func (s *mockStream) Conn() network.Conn {
	return &mockConn{
		localPeer:  s.localPeer,
		remotePeer: s.remotePeer,
	}
}

func (s *mockStream) Scope() network.StreamScope {
	return nil
}

// Helper methods for testing.
func (s *mockStream) setReadData(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.readBuffer = data
}

func (s *mockStream) getWrittenData() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data := make([]byte, len(s.writeBuffer))
	copy(data, s.writeBuffer)

	return data
}

func (s *mockStream) setResetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resetErr = err
}

func (s *mockStream) setCloseError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closeErr = err
}

// mockEncoder implements a simple encoder for testing.
type mockEncoder struct {
	encodeFunc func(msg any) ([]byte, error)
	decodeFunc func(data []byte, msgType any) error
}

func (e *mockEncoder) Encode(msg any) ([]byte, error) {
	if e.encodeFunc != nil {
		return e.encodeFunc(msg)
	}

	// Simple string encoding for testing
	if str, ok := msg.(string); ok {
		return []byte(str), nil
	}

	return nil, errors.New("unsupported type")
}

func (e *mockEncoder) Decode(data []byte, msgType any) error {
	if e.decodeFunc != nil {
		return e.decodeFunc(data, msgType)
	}

	// Simple string decoding for testing
	if ptr, ok := msgType.(*string); ok {
		*ptr = string(data)

		return nil
	}

	return errors.New("unsupported type")
}

// mockCompressor implements a simple compressor for testing.
type mockCompressor struct {
	compressFunc   func(data []byte) ([]byte, error)
	decompressFunc func(data []byte) ([]byte, error)
}

func (c *mockCompressor) Compress(data []byte) ([]byte, error) {
	if c.compressFunc != nil {
		return c.compressFunc(data)
	}

	// Simple prefix compression for testing
	return append([]byte("COMPRESSED:"), data...), nil
}

func (c *mockCompressor) Decompress(data []byte) ([]byte, error) {
	if c.decompressFunc != nil {
		return c.decompressFunc(data)
	}

	// Simple prefix decompression for testing
	prefix := []byte("COMPRESSED:")
	if len(data) < len(prefix) {
		return nil, errors.New("invalid compressed data")
	}

	return data[len(prefix):], nil
}

// mockStreamHandler implements StreamHandler for testing.
type mockStreamHandler struct {
	handleFunc func(ctx context.Context, stream network.Stream)
}

func (h *mockStreamHandler) HandleStream(ctx context.Context, stream network.Stream) {
	if h.handleFunc != nil {
		h.handleFunc(ctx, stream)
	}
}

// Test protocol implementations.
type testProtocol struct {
	id              protocol.ID
	maxRequestSize  uint64
	maxResponseSize uint64
}

func (p testProtocol) ID() protocol.ID {
	return p.id
}

func (p testProtocol) MaxRequestSize() uint64 {
	return p.maxRequestSize
}

func (p testProtocol) MaxResponseSize() uint64 {
	return p.maxResponseSize
}

// Chunked test protocol.
type testChunkedProtocol struct {
	testProtocol
	chunked bool
}

func (p testChunkedProtocol) IsChunked() bool {
	return p.chunked
}

// Test request/response types.
type testRequest struct {
	Message string
	ID      int
}

type testResponse struct {
	Message string
	ID      int
	Time    time.Time
}

// Helper function to create a connected pair of mock hosts.
func createConnectedMockHosts() (*mockHost, *mockHost) {
	host1 := newMockHost("peer1")
	host2 := newMockHost("peer2")

	// Set up host1 to create streams that connect to host2's handlers
	host1.newStreamFunc = func(ctx context.Context, p peer.ID, pids ...protocol.ID) (network.Stream, error) {
		if len(pids) == 0 {
			return nil, errors.New("no protocol specified")
		}

		streamID := fmt.Sprintf("%s-%s-%s", host1.id, p, pids[0])
		stream := newMockStream(streamID, pids[0], host1.id, p)

		// Find handler in host2
		host2.mu.RLock()
		handler, ok := host2.handlers[pids[0]]
		host2.mu.RUnlock()

		if ok && handler != nil {
			// Simulate the remote side handling the stream
			go func() {
				// Create a bidirectional mock stream
				remoteStream := &bidirectionalMockStream{
					mockStream:   stream,
					localStream:  stream,
					remoteBuffer: make(chan []byte, 100),
				}
				handler(remoteStream)
			}()
		}

		return stream, nil
	}

	return host1, host2
}

// mockConn implements a mock network connection for testing.
type mockConn struct {
	localPeer  peer.ID
	remotePeer peer.ID
}

func (c *mockConn) Close() error {
	return nil
}

func (c *mockConn) CloseWithError(errCode network.ConnErrorCode) error {
	return nil
}

func (c *mockConn) IsClosed() bool {
	return false
}

func (c *mockConn) ID() string {
	return fmt.Sprintf("%s-%s", c.localPeer, c.remotePeer)
}

func (c *mockConn) NewStream(context.Context) (network.Stream, error) {
	return nil, errors.New("not implemented")
}

func (c *mockConn) GetStreams() []network.Stream {
	return nil
}

func (c *mockConn) Stat() network.ConnStats {
	return network.ConnStats{}
}

func (c *mockConn) Scope() network.ConnScope {
	return nil
}

func (c *mockConn) LocalPeer() peer.ID {
	return c.localPeer
}

func (c *mockConn) RemotePeer() peer.ID {
	return c.remotePeer
}

func (c *mockConn) RemotePublicKey() ic.PubKey {
	return nil
}

func (c *mockConn) ConnState() network.ConnectionState {
	return network.ConnectionState{}
}

func (c *mockConn) LocalMultiaddr() ma.Multiaddr {
	return nil
}

func (c *mockConn) RemoteMultiaddr() ma.Multiaddr {
	return nil
}

// bidirectionalMockStream simulates a bidirectional stream.
type bidirectionalMockStream struct {
	*mockStream
	localStream  *mockStream
	remoteBuffer chan []byte
}

func (s *bidirectionalMockStream) Write(p []byte) (int, error) {
	// Write to the local stream's read buffer
	s.localStream.mu.Lock()
	s.localStream.readBuffer = append(s.localStream.readBuffer, p...)
	s.localStream.mu.Unlock()

	return len(p), nil
}

func (s *bidirectionalMockStream) Read(p []byte) (int, error) {
	// Read from the local stream's write buffer
	s.localStream.mu.Lock()
	defer s.localStream.mu.Unlock()

	if len(s.localStream.writeBuffer) == 0 {
		return 0, io.EOF
	}

	n := copy(p, s.localStream.writeBuffer)
	s.localStream.writeBuffer = s.localStream.writeBuffer[n:]

	return n, nil
}
