package host

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"

	"github.com/chuckpreslar/emission"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	gcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/libp2p/go-libp2p"
	mplex "github.com/libp2p/go-libp2p-mplex"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/sirupsen/logrus"
)

// Config is the configuration for the Mimicry host.
type Config struct {
	IPAddr  net.IP `yaml:"ipAddr"`
	UDPPort int    `yaml:"udpPort"`
	TCPPort int    `yaml:"tcpPort"`
	PrivKey string `yaml:"privKey"`
}

// Validate validates the Mimicry host config.
func (c *Config) Validate() error {
	if c.IPAddr == nil {
		return errors.New("ipAddr is required")
	}

	return nil
}

// Node is a Mimicry host
// It is responsible for managing the libp2p host and the ENR.
type Node struct {
	log logrus.FieldLogger

	config    *Config
	userAgent string

	listenIP net.IP

	host host.Host

	broker *emission.Emitter

	DerivedPrivKey *crypto.Secp256k1PrivateKey

	metrics *Metrics
}

func NewNode(ctx context.Context, log logrus.FieldLogger, config *Config, userAgent string) (*Node, error) {
	return &Node{
		log:       log.WithField("module", "mimicry/host"),
		config:    config,
		broker:    emission.NewEmitter(),
		userAgent: userAgent,
		metrics:   NewMetrics(),
	}, nil
}

func (n *Node) Start(ctx context.Context) (host.Host, error) {
	n.log.WithFields(logrus.Fields{
		"ipAddr":  n.config.IPAddr,
		"udpPort": n.config.UDPPort,
		"tcpPort": n.config.TCPPort,
	}).Info("Starting host")

	if n.config.IPAddr.String() == "0.0.0.0" {
		ip, err := discoverIP()
		if err != nil {
			return nil, fmt.Errorf("failed to discover IP address: %w", err)
		}

		n.listenIP = ip
	} else {
		n.listenIP = n.config.IPAddr
	}

	addrStrings := []string{
		fmt.Sprintf("/ip4/%s/tcp/%d", n.listenIP.String(), n.config.TCPPort),
	}

	n.log.WithField("multiaddr", addrStrings[0]).Info("Listening on multiaddr")

	// Start our libp2p host
	if _, errr := n.derivePrivateKey(); errr != nil {
		return nil, fmt.Errorf("failed to derive private key: %w", errr)
	}

	str, err := rcmgr.NewStatsTraceReporter()
	if err != nil {
		return nil, err
	}

	rmgr, err := rcmgr.NewResourceManager(
		rcmgr.NewFixedLimiter(rcmgr.DefaultLimits.AutoScale()),
		rcmgr.WithTraceReporter(str),
	)
	if err != nil {
		return nil, err
	}

	libp2pOptions := []libp2p.Option{
		libp2p.ListenAddrStrings(addrStrings...),
		libp2p.UserAgent(n.userAgent),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Muxer(mplex.ID, mplex.DefaultTransport),
		libp2p.DefaultMuxers,
		libp2p.Security(noise.ID, noise.New),
		libp2p.Ping(true),
		libp2p.DisableRelay(),
		libp2p.Identity(n.DerivedPrivKey),
		libp2p.ResourceManager(rmgr),
	}

	h, err := libp2p.New(libp2pOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	h.Network().Notify(n)

	n.host = h

	n.log.Info("Host started")

	return h, nil
}

func (n *Node) Stop(ctx context.Context) error {
	return n.host.Close()
}

func (n *Node) Peerstore() peerstore.Peerstore {
	return n.host.Peerstore()
}

func (n *Node) Network() network.Network {
	return n.host.Network()
}

func (n *Node) Connectedness(p peer.ID) network.Connectedness {
	return n.host.Network().Connectedness(p)
}

func (n *Node) Host() host.Host {
	return n.host
}

func (n *Node) ConnectToPeer(ctx context.Context, p peer.AddrInfo) error {
	// Emit BeforePeerConnect event
	n.emitBeforePeerConnect(p.ID)

	n.log.WithFields(logrus.Fields{
		"peer": p.ID,
	}).Debug("Connecting to peer")

	return n.host.Connect(ctx, p)
}

func (n *Node) DisconnectFromPeer(ctx context.Context, p peer.ID) error {
	// Emit BeforePeerDisconnect event
	n.emitBeforePeerDisconnect(p)

	// Close the peer connection
	if err := n.host.Network().ClosePeer(p); err != nil {
		return fmt.Errorf("failed to disconnect from peer %s: %w", p, err)
	}

	return nil
}

func (n *Node) AddProtocols(p peer.ID, protocols ...protocol.ID) error {
	return n.Peerstore().AddProtocols(p, protocols...)
}

func (n *Node) derivePrivateKey() (*crypto.Secp256k1PrivateKey, error) {
	// Ensure this is only run once
	if n.DerivedPrivKey != nil {
		return n.DerivedPrivKey, nil
	}

	var err error

	var privBytes []byte

	if n.config.PrivKey == "" {
		key, errr := ecdsa.GenerateKey(gcrypto.S256(), rand.Reader)
		if errr != nil {
			return nil, fmt.Errorf("failed to generate key: %w", errr)
		}

		privBytes = gcrypto.FromECDSA(key)
		if len(privBytes) != secp256k1.PrivKeyBytesLen {
			return nil, fmt.Errorf("expected secp256k1 data size to be %d", secp256k1.PrivKeyBytesLen)
		}
	} else {
		privBytes, err = hex.DecodeString(n.config.PrivKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decode private key: %w", err)
		}
	}

	n.DerivedPrivKey = (*crypto.Secp256k1PrivateKey)(secp256k1.PrivKeyFromBytes(privBytes))

	if n.config.PrivKey == "" {
		n.config.PrivKey = hex.EncodeToString(privBytes)
	}

	return n.DerivedPrivKey, nil
}

//nolint:unused // Will revisit if not-needed.
func (n *Node) createLocalNode(
	privKey *crypto.Secp256k1PrivateKey,
	ipAddr net.IP,
	udpPort, tcpPort int,
) (*enode.LocalNode, error) {
	db, err := enode.OpenDB("")
	if err != nil {
		return nil, err
	}

	rawKey, err := privKey.Raw()
	if err != nil {
		return nil, err
	}

	priv := new(ecdsa.PrivateKey)
	k := new(big.Int).SetBytes(rawKey)
	priv.D = k
	priv.Curve = gcrypto.S256()
	priv.X, priv.Y = gcrypto.S256().ScalarBaseMult(rawKey)

	localNode := enode.NewLocalNode(db, priv)

	ipEntry := enr.IP(ipAddr)
	udpEntry := enr.UDP(udpPort) //nolint:gosec // not concerned about overflow here.
	tcpEntry := enr.TCP(tcpPort) //nolint:gosec // not concerned about overflow here.

	localNode.Set(ipEntry)
	localNode.Set(udpEntry)
	localNode.Set(tcpEntry)
	localNode.SetFallbackIP(ipAddr)
	localNode.SetFallbackUDP(udpPort)

	return localNode, nil
}

func (n *Node) updatePeerMetrics() {
	peers := map[network.Direction]map[network.Connectedness]int{}

	for _, p := range n.host.Network().Peers() {
		if conns := n.host.Network().ConnsToPeer(p); len(conns) > 0 {
			for _, conn := range conns {
				if _, ok := peers[conn.Stat().Direction]; !ok {
					peers[conn.Stat().Direction] = map[network.Connectedness]int{}
				}

				peers[conn.Stat().Direction][n.host.Network().Connectedness(p)]++
			}
		}
	}

	for direction, stateCounts := range peers {
		for state, count := range stateCounts {
			s := state.String()

			if s == "NotConnected" {
				s = "disconnected"
			}

			s = strings.ToLower(s)

			n.metrics.SetPeers(count, direction.String(), s)
		}
	}
}

func (n *Node) Connected(net network.Network, conn network.Conn) {
	n.metrics.PeerConnectsTotal.Inc()

	n.updatePeerMetrics()

	n.log.WithFields(logrus.Fields{
		"peer": conn.RemotePeer(),
	}).Debug("Connected to peer")

	n.emitAfterPeerConnect(net, conn)
}

func (n *Node) Disconnected(net network.Network, conn network.Conn) {
	n.metrics.PeerDisconnectsTotal.Inc()

	n.updatePeerMetrics()

	n.log.WithFields(logrus.Fields{
		"peer": conn.RemotePeer(),
	}).Debug("Disconnected from peer")

	// Emit AfterPeerDisconnect event
	n.emitAfterPeerDisconnect(net, conn)
}

func (n *Node) Listen(net network.Network, addr ma.Multiaddr) {
	n.log.WithFields(logrus.Fields{
		"addr": addr,
	}).Info("Listening on address")
}

func (n *Node) ListenClose(net network.Network, addr ma.Multiaddr) {
	n.log.WithFields(logrus.Fields{
		"addr": addr,
	}).Info("Stopped listening on address")
}

func discoverIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, errors.New("failed to get local address")
	}

	return localAddr.IP, nil
}
