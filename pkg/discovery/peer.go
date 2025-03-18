package discovery

import (
	"crypto/elliptic"
	"errors"
	"fmt"
	"net"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

type ConnectablePeer struct {
	AddrInfo peer.AddrInfo
	Enode    *enode.Node
}

func DeriveDetailsFromNode(node *enode.Node) (*ConnectablePeer, error) {
	pubKey := node.Pubkey()
	if pubKey == nil {
		return nil, errors.New("public key is nil")
	}

	secpKey, err := crypto.UnmarshalSecp256k1PublicKey(elliptic.Marshal(secp256k1.S256(), pubKey.X, pubKey.Y))
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal secp256k1 public key: %w", err)
	}

	peerID, err := peer.IDFromPublicKey(secpKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer ID from public key: %w", err)
	}

	var ipVersion string

	ip := node.IP()

	switch {
	case ip.To4() != nil && len(ip.To4()) == net.IPv4len:
		ipVersion = "ip4"
	case ip.To16() != nil && len(ip.To16()) == net.IPv6len:
		ipVersion = "ip6"
	default:
		return nil, errors.New("no IP address found in ENR")
	}

	maddrs := []ma.Multiaddr{}

	if node.UDP() != 0 {
		maddrStr := fmt.Sprintf("/%s/%s/udp/%d", ipVersion, node.IP(), node.UDP())

		maddr, err := ma.NewMultiaddr(maddrStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse multiaddress %s: %w", maddrStr, err)
		}

		maddrs = append(maddrs, maddr)
	}

	if node.TCP() != 0 {
		maddrStr := fmt.Sprintf("/%s/%s/tcp/%d", ipVersion, node.IP(), node.TCP())

		maddr, err := ma.NewMultiaddr(maddrStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse multiaddress %s: %w", maddrStr, err)
		}

		maddrs = append(maddrs, maddr)
	}

	return &ConnectablePeer{
		AddrInfo: peer.AddrInfo{
			ID:    peerID,
			Addrs: maddrs,
		},
		Enode: node,
	}, nil
}
