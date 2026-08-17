package mimicry

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"net"
	"testing"
	"time"

	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/p2p"
	gethrlpx "github.com/ethereum/go-ethereum/p2p/rlpx"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// dupResponseHelloMessage mirrors the Hello struct in message_hello.go.
// Kept local to this file (rather than a shared test helper) so this test
// has no dependency on test infrastructure from other in-flight changes.
type dupResponseHelloMessage struct {
	Version    uint64
	Name       string
	Caps       []p2p.Cap
	ListenPort uint64
	ID         []byte
	Rest       []rlp.RawValue `rlp:"tail"`
}

// dupResponseTestPeer is a minimal, protocol-faithful devp2p peer used to
// drive Client through a real handshake and Hello exchange.
type dupResponseTestPeer struct {
	listener net.Listener
	priv     *ecdsa.PrivateKey
	conn     net.Conn
}

func newDupResponseTestPeer(t *testing.T) *dupResponseTestPeer {
	t.Helper()

	priv, err := crypto.GenerateKey()
	require.NoError(t, err)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	p := &dupResponseTestPeer{listener: ln, priv: priv}

	t.Cleanup(func() {
		_ = ln.Close()
		if p.conn != nil {
			_ = p.conn.Close()
		}
	})

	return p
}

func (p *dupResponseTestPeer) enode() string {
	pub := crypto.FromECDSAPub(&p.priv.PublicKey)[1:]

	return fmt.Sprintf("enode://%s@%s", hex.EncodeToString(pub), p.listener.Addr().String())
}

func (p *dupResponseTestPeer) acceptAndHandshake(t *testing.T) *gethrlpx.Conn {
	t.Helper()

	conn, err := p.listener.Accept()
	require.NoError(t, err)

	p.conn = conn

	rc := gethrlpx.NewConn(conn, nil)
	_, err = rc.Handshake(p.priv)
	require.NoError(t, err)

	code, data, _, err := rc.Read()
	require.NoError(t, err)
	require.Equal(t, uint64(HelloCode), code)

	var clientHello dupResponseHelloMessage
	require.NoError(t, rlp.DecodeBytes(data, &clientHello))

	pub := crypto.FromECDSAPub(&p.priv.PublicKey)[1:]
	ourHello := &dupResponseHelloMessage{
		Version:    P2PProtocolVersion,
		Name:       "test-peer",
		Caps:       SupportedEthCaps(),
		ListenPort: 0,
		ID:         pub,
	}

	encoded, err := rlp.EncodeToBytes(ourHello)
	require.NoError(t, err)
	_, err = rc.Write(HelloCode, encoded)
	require.NoError(t, err)

	// The client enables snappy on its own connection as soon as it
	// processes our Hello (see handleHello); both sides must agree.
	rc.SetSnappy(true)

	return rc
}

// TestGetPooledTransactions_DuplicateResponseDoesNotDeadlock guards against
// EXECUTION-01: a peer that replies to a single GetPooledTransactions
// request with two PooledTransactions messages carrying the same request
// ID used to be able to deadlock the session read loop. The second
// handlePooledTransactions call would block sending on an unbuffered,
// already-drained channel while holding pooledTransactionsMux, and the
// original caller's deferred cleanup would then block forever trying to
// acquire the same mutex.
func TestGetPooledTransactions_DuplicateResponseDoesNotDeadlock(t *testing.T) {
	peer := newDupResponseTestPeer(t)

	serverErr := make(chan error, 1)
	helloDone := make(chan struct{})

	go func() {
		rc := peer.acceptAndHandshake(t)
		close(helloDone)

		code, data, _, err := rc.Read()
		if err != nil {
			serverErr <- err

			return
		}
		if code != uint64(GetPooledTransactionsCode) {
			serverErr <- fmt.Errorf("expected GetPooledTransactions code %d, got %d", GetPooledTransactionsCode, code)

			return
		}

		var req eth.GetPooledTransactionsPacket
		if err := rlp.DecodeBytes(data, &req); err != nil {
			serverErr <- err

			return
		}

		// Reply TWICE with the same request ID, back-to-back -- a
		// malicious or buggy peer replaying/duplicating a response.
		resp := eth.PooledTransactionsPacket{RequestId: req.RequestId}

		encoded, err := rlp.EncodeToBytes(&resp)
		if err != nil {
			serverErr <- err

			return
		}
		if _, err := rc.Write(uint64(PooledTransactionsCode), encoded); err != nil {
			serverErr <- err

			return
		}
		if _, err := rc.Write(uint64(PooledTransactionsCode), encoded); err != nil {
			serverErr <- err

			return
		}

		serverErr <- nil
	}()

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	client, err := New(context.Background(), log, peer.enode(), "test-client")
	require.NoError(t, err)

	require.NoError(t, client.Start(context.Background()))
	t.Cleanup(func() { _ = client.Stop(context.Background()) })

	select {
	case <-helloDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Hello exchange never completed")
	}
	// Let the client finish processing the server's Hello (handleHello
	// enables snappy on the client's side); otherwise GetPooledTransactions
	// could race ahead and write before snappy is enabled, while the
	// server here already expects it.
	time.Sleep(100 * time.Millisecond)

	firstDone := make(chan error, 1)
	go func() {
		_, err := client.GetPooledTransactions(context.Background(), []gethcommon.Hash{{0x01}})
		firstDone <- err
	}()

	select {
	case err := <-firstDone:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("GetPooledTransactions did not return within 3s for the duplicated response")
	}

	require.NoError(t, <-serverErr)

	// The real regression check: pooledTransactionsMux must not be stuck.
	// Before the fix, the second (duplicate) response's send could block
	// forever while holding this mutex, so any later acquisition -- by a
	// fresh GetPooledTransactions call, or the first call's own deferred
	// cleanup -- would hang permanently. Probe the mutex directly instead
	// of waiting out GetPooledTransactions' own 10s internal timeout.
	acquired := make(chan struct{})
	go func() {
		client.pooledTransactionsMux.Lock()
		client.pooledTransactionsMux.Unlock()
		close(acquired)
	}()

	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("pooledTransactionsMux could not be acquired 2s after the duplicate response: it is deadlocked")
	}
}
