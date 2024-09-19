package mimicry

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	gcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/libp2p/go-libp2p"
	mplex "github.com/libp2p/go-libp2p-mplex"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/p2p/encoder"
	"github.com/sirupsen/logrus"
)

var sszNetworkEncoder = encoder.SszNetworkEncoder{}

const (
	StatusProtocolID   = "/eth2/beacon_chain/req/status/1/" + encoder.ProtocolSuffixSSZSnappy
	GoodbyeProtocolID  = "/eth2/beacon_chain/req/goodbye/1/" + encoder.ProtocolSuffixSSZSnappy
	PingProtocolID     = "/eth2/beacon_chain/req/ping/1/" + encoder.ProtocolSuffixSSZSnappy
	MetaDataProtocolID = "/eth2/beacon_chain/req/metadata/2/" + encoder.ProtocolSuffixSSZSnappy
)

var (
	SupportedProtocols = []protocol.ID{
		StatusProtocolID,
		GoodbyeProtocolID,
		PingProtocolID,
		MetaDataProtocolID,
	}
)

type Client struct {
	log       logrus.FieldLogger
	config    *Config
	mode      Mode
	userAgent string

	localNode *enode.LocalNode
	host      host.Host
	privKey   *crypto.Secp256k1PrivateKey

	ctx      context.Context
	cancel   context.CancelFunc
	metadata *common.MetaData
	status   *common.Status

	statusMu sync.Mutex
}

func NewClient(log logrus.FieldLogger, config *Config, mode Mode, userAgent string) *Client {
	return &Client{
		log:       log,
		userAgent: userAgent,
		config:    config,
		mode:      mode,
		statusMu:  sync.Mutex{},
		status:    &common.Status{},
		metadata: &common.MetaData{
			SeqNumber: 0,
			Attnets:   common.AttnetBits{},
			Syncnets:  common.SyncnetBits{},
		},
	}
}

func (c *Client) Start(ctx context.Context) error {
	c.log.WithFields(logrus.Fields{
		"mode": c.mode,
	}).Info("Starting Mimicry client")

	if _, err := c.derivePrivateKey(); err != nil {
		return fmt.Errorf("failed to derive private key: %w", err)
	}

	libp2pOptions := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", c.config.IPAddr.String(), c.config.TCPPort)),
		libp2p.UserAgent(c.userAgent),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Muxer(mplex.ID, mplex.DefaultTransport),
		libp2p.DefaultMuxers,
		libp2p.Security(noise.ID, noise.New),
		libp2p.Ping(false),
		libp2p.Identity(c.privKey),
		libp2p.ResourceManager(&network.NullResourceManager{}),
	}

	h, err := libp2p.New(libp2pOptions...)
	if err != nil {
		return fmt.Errorf("failed to create libp2p host: %w", err)
	}

	c.host = h

	localNode, err := createLocalNode(c.privKey, c.config.IPAddr, c.config.UDPPort, c.config.TCPPort)
	if err != nil {
		return fmt.Errorf("failed to create local node: %w", err)
	}

	c.localNode = localNode

	if err := c.registerHandlers(); err != nil {
		return fmt.Errorf("failed to register handlers: %w", err)
	}

	c.log.Info("Successfully started Mimicry client")

	return nil
}

func (c *Client) Stop() error {
	if err := c.host.Close(); err != nil {
		return fmt.Errorf("failed to close host: %w", err)
	}

	return nil
}

func (c *Client) SetStatus(status *common.Status) {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()

	c.status = status
}

func (c *Client) GetStatus() common.Status {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()

	return *c.status
}

func (c *Client) derivePrivateKey() (*crypto.Secp256k1PrivateKey, error) {
	if c.privKey != nil {
		return c.privKey, nil
	}

	var err error

	var privBytes []byte

	if c.config.PrivKey == "" {
		key, errr := ecdsa.GenerateKey(gcrypto.S256(), rand.Reader)
		if errr != nil {
			return nil, fmt.Errorf("failed to generate key: %w", errr)
		}

		privBytes = gcrypto.FromECDSA(key)
		if len(privBytes) != secp256k1.PrivKeyBytesLen {
			return nil, fmt.Errorf("expected secp256k1 data size to be %d", secp256k1.PrivKeyBytesLen)
		}
	} else {
		privBytes, err = hex.DecodeString(c.config.PrivKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decode private key: %w", err)
		}
	}

	c.privKey = (*crypto.Secp256k1PrivateKey)(secp256k1.PrivKeyFromBytes(privBytes))

	if c.config.PrivKey == "" {
		c.config.PrivKey = hex.EncodeToString(privBytes)
	}

	return c.privKey, nil
}

func (c *Client) ConnectToPeer(ctx context.Context, p peer.AddrInfo, enr *enode.Node) error {
	c.log.WithFields(logrus.Fields{
		"peer": p.ID,
	}).Info("Connecting to peer")

	// Check if we're already connected to the peer
	if status := c.host.Network().Connectedness(p.ID); status == network.Connected {
		return errors.New("already connected to peer")
	}

	// Connect to the peer
	if err := c.host.Connect(ctx, p); err != nil {
		return fmt.Errorf("failed to connect to peer: %w", err)
	}

	// Add the supported protocols to the peer
	if err := c.host.Peerstore().AddProtocols(p.ID, SupportedProtocols...); err != nil {
		return fmt.Errorf("failed to add protocols to peer: %w", err)
	}

	// Send a status request
	_, err := c.RequestStatusFromPeer(ctx, p.ID)
	if err != nil {
		return fmt.Errorf("failed to request status: %w", err)
	}

	return nil
}

func (c *Client) DisconnectFromPeer(ctx context.Context, peerID peer.ID) error {
	c.log.WithFields(logrus.Fields{
		"peer": peerID,
	}).Info("Disconnecting from peer")

	// Send a goodbye message
	goodbye := common.Goodbye(0)
	resp := common.Goodbye(0)

	if err := c.sendRequest(ctx, &Request{
		ProtocolID: GoodbyeProtocolID,
		PeerID:     peerID,
		Payload:    &goodbye,
		Timeout:    time.Second * 30,
	}, &resp); err != nil {
		return fmt.Errorf("failed to send goodbye message: %w", err)
	}

	return c.host.Network().ClosePeer(peerID)
}
