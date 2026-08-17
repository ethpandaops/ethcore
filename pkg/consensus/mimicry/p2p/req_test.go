package p2p

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const testProtocolID = "/test/req/1"

func newTestHost(t *testing.T) host.Host {
	t.Helper()

	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
		libp2p.DisableRelay(),
	)
	require.NoError(t, err)

	t.Cleanup(func() { _ = h.Close() })

	return h
}

func connectHosts(t *testing.T, a, b host.Host) {
	t.Helper()

	err := a.Connect(context.Background(), peer.AddrInfo{ID: b.ID(), Addrs: b.Addrs()})
	require.NoError(t, err)
}

func newTestReqResp(t *testing.T, h host.Host) *ReqResp {
	t.Helper()

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	return NewReqResp(log, h, encoder.SszNetworkEncoder{}, &ReqRespConfig{
		WriteTimeout:    2 * time.Second,
		ReadTimeout:     2 * time.Second,
		TimeToFirstByte: 2 * time.Second,
	})
}

// TestSendRequest_UnresponsivePeerTimesOut guards against REQRESP-01: a peer
// that accepts the stream and then never responds must not be able to park
// SendRequest's goroutine (and the underlying stream) forever. Before the
// fix, SendRequest had no read or write deadline of its own, so the request
// Timeout only bounded stream setup and this call would hang indefinitely.
func TestSendRequest_UnresponsivePeerTimesOut(t *testing.T) {
	clientHost := newTestHost(t)
	serverHost := newTestHost(t)
	connectHosts(t, clientHost, serverHost)

	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })

	accepted := make(chan struct{})
	serverHost.SetStreamHandler(testProtocolID, func(stream network.Stream) {
		close(accepted)
		// Accept the stream, then go silent: never read further, never
		// write a response, never close. This is exactly the "unresponsive
		// peer" REQRESP-01 describes.
		<-stop
	})

	// SendRequest's read/write deadlines come from ReqRespConfig, not from
	// the per-request Timeout (which only bounds stream creation) -- so the
	// config here, not req.Timeout, is what must be short to prove the fix.
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	client := NewReqResp(log, clientHost, encoder.SszNetworkEncoder{}, &ReqRespConfig{
		WriteTimeout:    300 * time.Millisecond,
		ReadTimeout:     300 * time.Millisecond,
		TimeToFirstByte: 300 * time.Millisecond,
	})

	req := &Request{
		ProtocolID: testProtocolID,
		PeerID:     serverHost.ID(),
		Payload:    nil,
		Timeout:    2 * time.Second,
	}

	done := make(chan error, 1)
	start := time.Now()

	go func() {
		var rsp common.Ping
		done <- client.SendRequest(context.Background(), req, &rsp)
	}()

	select {
	case <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("server never received the stream")
	}

	select {
	case err := <-done:
		elapsed := time.Since(start)
		require.Error(t, err, "SendRequest must fail against an unresponsive peer")
		require.Less(t, elapsed, 2*time.Second, "SendRequest took too long to give up")
	case <-time.After(2 * time.Second):
		t.Fatal("SendRequest did not return within 2s of an unresponsive peer; the read deadline is not being enforced")
	}
}

// TestSendRequest_Success is a regression guard: adding read/write
// deadlines to SendRequest must not break a normal successful exchange.
func TestSendRequest_Success(t *testing.T) {
	clientHost := newTestHost(t)
	serverHost := newTestHost(t)
	connectHosts(t, clientHost, serverHost)

	wantSeq := common.Ping(7)

	server := newTestReqResp(t, serverHost)
	err := server.RegisterHandler(context.Background(), testProtocolID, func(ctx context.Context, stream network.Stream) error {
		return server.WriteResponse(ctx, stream, &wantSeq, nil)
	})
	require.NoError(t, err)

	client := newTestReqResp(t, clientHost)

	req := &Request{
		ProtocolID: testProtocolID,
		PeerID:     serverHost.ID(),
		Payload:    nil,
		Timeout:    2 * time.Second,
	}

	var rsp common.Ping

	done := make(chan error, 1)
	go func() { done <- client.SendRequest(context.Background(), req, &rsp) }()

	select {
	case err := <-done:
		require.NoError(t, err)
		require.Equal(t, wantSeq, rsp)
	case <-time.After(2 * time.Second):
		t.Fatal("SendRequest did not return for a responsive peer")
	}
}
