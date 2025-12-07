package discovery

import (
	"errors"
	"fmt"
	"net"

	"github.com/OffchainLabs/prysm/v7/crypto/ecdsa"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

type ConnectablePeer struct {
	AddrInfo peer.AddrInfo
	Enode    *enode.Node
}

func DeriveDetailsFromNode(node *enode.Node) (*ConnectablePeer, error) {
	if node == nil {
		return nil, errors.New("node is nil")
	}

	ecdsaPubKey := node.Pubkey()
	if ecdsaPubKey == nil {
		return nil, errors.New("public key is nil")
	}

	// Convert the ECDSA public key to libp2p format using Prysm's implementation.
	//
	// We use Prysm's conversion method here because:
	// 1. Ethereum nodes use secp256k1 elliptic curve for their cryptographic operations
	// 2. While both geth (go-ethereum) and libp2p support secp256k1, they have different
	//    internal representations of the keys
	// 3. Direct conversion attempts (like using crypto/ecdh) fail because Go's standard
	//    crypto libraries don't support the secp256k1 curve
	// 4. Prysm's implementation includes the necessary "hack" to ensure libp2p's secp256k1
	//    keys are recognized as geth's secp256k1 in discovery v5 protocol
	secpKey, err := ecdsa.ConvertToInterfacePubkey(ecdsaPubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to convert ECDSA public key: %w", err)
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
