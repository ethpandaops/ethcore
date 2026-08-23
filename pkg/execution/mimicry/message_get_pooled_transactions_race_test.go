package mimicry

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	gethrlpx "github.com/ethereum/go-ethereum/p2p/rlpx"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// execution02HelloMessage mirrors the Hello struct in message_hello.go.
// Kept local to this file (rather than a shared test helper) so this test
// has no dependency on test infrastructure from other in-flight changes.
type execution02HelloMessage struct {
	Version    uint64
	Name       string
	Caps       []p2p.Cap
	ListenPort uint64
	ID         []byte
	Rest       []rlp.RawValue `rlp:"tail"`
}

// execution02TestPeer is a minimal, protocol-faithful devp2p peer used to
// drive Client through a real RLPx handshake, then goes silent -- it
// deliberately never sends its own Hello back. Client.Start returns as
// soon as the low-level handshake succeeds, without waiting on Hello, so
// this is enough to unblock Start. Since the peer never replies with a
// Hello, the client's handleHello (and its SetSnappy call) never run
// during this test, which keeps the concurrent sends below from being
// able to race that unrelated, already-tracked issue (EXECUTION-04).
type execution02TestPeer struct {
	listener net.Listener
	priv     *ecdsa.PrivateKey
	conn     net.Conn
}

func newExecution02TestPeer(t *testing.T) *execution02TestPeer {
	t.Helper()

	priv, err := crypto.GenerateKey()
	require.NoError(t, err)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	p := &execution02TestPeer{listener: ln, priv: priv}

	t.Cleanup(func() {
		_ = ln.Close()
		if p.conn != nil {
			_ = p.conn.Close()
		}
	})

	return p
}

func (p *execution02TestPeer) enode() string {
	pub := crypto.FromECDSAPub(&p.priv.PublicKey)[1:]

	return fmt.Sprintf("enode://%s@%s", hex.EncodeToString(pub), p.listener.Addr().String())
}

func (p *execution02TestPeer) acceptAndHandshake(t *testing.T) *gethrlpx.Conn {
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

	var clientHello execution02HelloMessage
	require.NoError(t, rlp.DecodeBytes(data, &clientHello))

	// Deliberately do not reply with our own Hello -- see the type doc on
	// execution02TestPeer for why.
	return rc
}

// TestGetPooledTransactions_ConcurrentCallsDoNotRaceMap guards against
// EXECUTION-02: the select statement indexed pooledTransactionsMap
// directly, reading it with no lock, while the map is written under
// pooledTransactionsMux both at request setup and in the deferred
// cleanup. Two concurrent GetPooledTransactions calls on the same client
// -- the exact pattern the request-ID-keyed map exists to support -- could
// hit a fatal concurrent map read/write and crash the process. Run under
// -race.
func TestGetPooledTransactions_ConcurrentCallsDoNotRaceMap(t *testing.T) {
	const attempts = 10

	for attempt := 0; attempt < attempts; attempt++ {
		t.Run(fmt.Sprintf("attempt-%d", attempt), func(t *testing.T) {
			peer := newExecution02TestPeer(t)

			handshakeDone := make(chan struct{})

			go func() {
				// Complete the low-level handshake and then go silent: no
				// Hello reply and no request response are ever sent, so
				// both concurrent calls below block in their own select
				// for the same window, maximizing overlap between
				// goroutines, and the client's handleHello/SetSnappy path
				// never runs at all.
				peer.acceptAndHandshake(t)
				close(handshakeDone)
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
				t.Fatal("handshake never completed")
			}

			const concurrentCalls = 30

			var wg sync.WaitGroup
			wg.Add(concurrentCalls)

			for i := 0; i < concurrentCalls; i++ {
				hash := gethcommon.Hash{byte(i)}

				go func() {
					defer wg.Done()
					_, _ = client.GetPooledTransactions(context.Background(), []gethcommon.Hash{hash})
				}()
			}

			wg.Wait()
		})
	}
}
