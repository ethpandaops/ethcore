package mimicry

import (
	"crypto/ecdsa"
	"math/big"
	"net"

	gcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/libp2p/go-libp2p/core/crypto"
)

func createLocalNode(
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
	udpEntry := enr.UDP(udpPort)
	tcpEntry := enr.TCP(tcpPort)

	localNode.Set(ipEntry)
	localNode.Set(udpEntry)
	localNode.Set(tcpEntry)
	localNode.SetFallbackIP(ipAddr)
	localNode.SetFallbackUDP(udpPort)

	return localNode, nil
}
