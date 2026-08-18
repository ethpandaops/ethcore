package mimicry

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"testing"
	"time"

	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	gethrlpx "github.com/ethereum/go-ethereum/p2p/rlpx"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// testHelloMessage mirrors the Hello struct in message_hello.go, kept
// separate so this test doesn't depend on package-internal wiring beyond
// the public devp2p wire format.
type testHelloMessage struct {
	Version    uint64
	Name       string
	Caps       []p2p.Cap
	ListenPort uint64
	ID         []byte
	Rest       []rlp.RawValue `rlp:"tail"`
}

// testRLPxPeer is a minimal, protocol-faithful devp2p peer used to drive
// Client against real handshake and Hello exchanges without needing a real
// execution node. It owns the accepted connection for its whole lifetime
// so nothing relies on a goroutine-local variable staying reachable.
type testRLPxPeer struct {
	listener net.Listener
	priv     *ecdsa.PrivateKey
	conn     net.Conn
}

func newTestRLPxPeer(t *testing.T) *testRLPxPeer {
	t.Helper()

	priv, err := crypto.GenerateKey()
	require.NoError(t, err)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	p := &testRLPxPeer{listener: ln, priv: priv}

	t.Cleanup(func() {
		_ = ln.Close()
		if p.conn != nil {
			_ = p.conn.Close()
		}
	})

	return p
}

func (p *testRLPxPeer) enode() string {
	pub := crypto.FromECDSAPub(&p.priv.PublicKey)[1:]

	return fmt.Sprintf("enode://%s@%s", hex.EncodeToString(pub), p.listener.Addr().String())
}

// acceptAndHandshake accepts one connection, completes the RLPx handshake
// as recipient, and exchanges Hello. The connection is stored on p for the
// lifetime of the peer (see the type doc comment).
func (p *testRLPxPeer) acceptAndHandshake(t *testing.T) *gethrlpx.Conn {
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

	var clientHello testHelloMessage
	require.NoError(t, rlp.DecodeBytes(data, &clientHello))

	pub := crypto.FromECDSAPub(&p.priv.PublicKey)[1:]
	ourHello := &testHelloMessage{
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
	// processes our Hello (see handleHello); the wire format requires both
	// sides to agree, so we must match it here.
	rc.SetSnappy(true)

	return rc
}

// largeTransactions builds a Transactions frame just under the RLPx frame
// size cap, filled with random (so snappy can't shrink it back down) data.
func largeTransactions(t *testing.T, size int) *Transactions {
	t.Helper()

	data := make([]byte, size)
	_, err := rand.Read(data)
	require.NoError(t, err)

	to := gethcommon.HexToAddress("0x000000000000000000000000000000000000ff")
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    0,
		GasPrice: big.NewInt(1),
		Gas:      21000,
		To:       &to,
		Value:    big.NewInt(0),
		Data:     data,
	})

	var list rlp.RawList[*types.Transaction]
	require.NoError(t, list.Append(tx))

	return &Transactions{RawList: list}
}

// TestWriteRLPx_UnresponsivePeerDoesNotBlockForever guards against
// EXECUTION-03: a peer that stops draining its socket after the handshake
// must not be able to park writeRLPx (and therefore every exported sender,
// since they all serialize on writeMu) forever. Before the fix, no write
// deadline was ever set on the underlying connection.
func TestWriteRLPx_UnresponsivePeerDoesNotBlockForever(t *testing.T) {
	original := writeTimeout
	writeTimeout = 200 * time.Millisecond
	t.Cleanup(func() { writeTimeout = original })

	peer := newTestRLPxPeer(t)

	handshakeDone := make(chan struct{})

	go func() {
		peer.acceptAndHandshake(t)
		close(handshakeDone)
		// Go silent from here: never read again. This is the "peer stops
		// draining its socket" scenario EXECUTION-03 describes. peer.conn
		// stays referenced (and open) via the testRLPxPeer struct itself,
		// which the test keeps alive until its Cleanup runs.
	}()

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	client, err := New(context.Background(), log, peer.enode(), "test-client")
	require.NoError(t, err)

	require.NoError(t, client.Start(context.Background()))
	t.Cleanup(func() { _ = client.Stop(context.Background()) })

	select {
	case <-handshakeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("server never completed the handshake")
	}

	time.Sleep(100 * time.Millisecond) // let the client process our Hello (enables snappy)

	txs := largeTransactions(t, 12_000_000) // under the ~16MB RLPx frame cap, large enough to fill kernel buffers

	done := make(chan error, 1)
	start := time.Now()

	go func() { done <- client.Transactions(context.Background(), txs) }()

	select {
	case err := <-done:
		elapsed := time.Since(start)
		require.Error(t, err, "Transactions must fail once the write deadline is exceeded")
		require.Less(t, elapsed, 2*time.Second, "Transactions took too long to give up")
	case <-time.After(2 * time.Second):
		t.Fatal("Transactions did not return within 2s against an unresponsive peer; the write deadline is not being enforced")
	}
}
