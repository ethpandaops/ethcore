// Package enr provides utilities for parsing and extracting data from Ethereum Node Records (ENR).
// It supports parsing ENR strings and extracting various fields including network information,
// cryptographic keys, and Ethereum 2.0 specific metadata.
package enr

import (
	"fmt"
	"net"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// s256raw represents a raw secp256k1 public key field in an ENR record.
type s256raw []byte

// ENRKey returns the key identifier for the secp256k1 public key field.
func (s256raw) ENRKey() string { return "secp256k1" }

// eth2 represents an Ethereum 2.0 public key field in an ENR record.
type eth2 []byte

// ENRKey returns the key identifier for the eth2 public key field.
func (eth2) ENRKey() string { return "eth2" }

// attnets represents an attestation subnet bitfield in an ENR record.
// It indicates which attestation subnets the node is subscribed to.
type attnets []byte

// ENRKey returns the key identifier for the attestation subnet bitfield.
func (attnets) ENRKey() string { return "attnets" }

// syncnets represents a sync subnet bitfield in an ENR record.
// It indicates which sync subnets the node is subscribed to.
type syncnets []byte

// ENRKey returns the key identifier for the sync subnet bitfield.
func (syncnets) ENRKey() string { return "syncnets" }

// cgc represents the custody group count field in an ENR record.
// It indicates the number of custody groups the node supports.
type cgc []byte

// ENRKey returns the key identifier for the custody group count field.
func (cgc) ENRKey() string { return "cgc" }

// nfd represents the next fork digest field in an ENR record.
// It contains the digest of the next scheduled fork.
type nfd []byte

// ENRKey returns the key identifier for the next fork digest field.
func (nfd) ENRKey() string { return "nfd" }

// quic represents the QUIC port field for IPv4 in an ENR record.
type quic uint16

// ENRKey returns the key identifier for the QUIC port field.
func (quic) ENRKey() string { return "quic" }

// quic6 represents the QUIC port field for IPv6 in an ENR record.
type quic6 uint16

// ENRKey returns the key identifier for the QUIC6 port field.
func (quic6) ENRKey() string { return "quic6" }

// Parse parses an ENR record string and extracts all available fields.
// It returns a populated ENR struct with all parsed values or an error if parsing fails.
func Parse(record string) (*ENR, error) {
	n, err := enode.Parse(enode.ValidSchemes, record)
	if err != nil {
		return nil, fmt.Errorf("failed to parse enr: %w", err)
	}

	return &ENR{
		Enr:       record,
		Signature: parseSignature(n),
		Seq:       parseSeq(n),
		ID:        parseID(n),
		Secp256k1: parseSecp256k1(n),
		IP4:       parseIP4(n),
		IP6:       parseIP6(n),
		TCP4:      parseTCP4(n),
		TCP6:      parseTCP6(n),
		UDP4:      parseUDP4(n),
		UDP6:      parseUDP6(n),
		QUIC4:     parseQUIC4(n),
		QUIC6:     parseQUIC6(n),
		ETH2:      parseETH2(n),
		Attnets:   parseAttnets(n),
		Syncnets:  parseSyncnets(n),
		CGC:       parseCGC(n),
		NFD:       parseNFD(n),
		NodeID:    parseNodeID(n),
		PeerID:    parsePeerID(n),
	}, nil
}

// GetEnr returns the raw ENR string.
func (e *ENR) GetEnr() string {
	if e == nil {
		return ""
	}

	return e.Enr
}

// GetSignature returns the cryptographic signature of the ENR record.
// Returns nil if the ENR is nil or the signature is not present.
func (e *ENR) GetSignature() *[]byte {
	if e == nil {
		return nil
	}

	return e.Signature
}

// GetSeq returns the sequence number of the ENR record.
// Returns nil if the ENR is nil or the sequence number is not present.
func (e *ENR) GetSeq() *uint64 {
	if e == nil {
		return nil
	}

	return e.Seq
}

// GetID returns the identity scheme of the ENR record.
// Returns nil if the ENR is nil or the ID is not present.
func (e *ENR) GetID() *string {
	if e == nil {
		return nil
	}

	return e.ID
}

// GetNodeID returns the node ID of the ENR record.
// Returns nil if the ENR is nil or the node ID is not present.
func (e *ENR) GetNodeID() *string {
	if e == nil {
		return nil
	}

	return e.NodeID
}

// GetPeerID returns the libp2p peer ID derived from the ENR record.
// Returns nil if the ENR is nil or the peer ID is not present.
func (e *ENR) GetPeerID() *string {
	if e == nil {
		return nil
	}

	return e.PeerID
}

// GetSecp256k1 returns the secp256k1 public key from the ENR record.
// Returns nil if the ENR is nil or the key is not present.
func (e *ENR) GetSecp256k1() *[]byte {
	if e == nil {
		return nil
	}

	return e.Secp256k1
}

// GetIP4 returns the IPv4 address from the ENR record.
// Returns nil if the ENR is nil or the IP4 address is not present.
func (e *ENR) GetIP4() *string {
	if e == nil {
		return nil
	}

	return e.IP4
}

// GetIP6 returns the IPv6 address from the ENR record.
// Returns nil if the ENR is nil or the IP6 address is not present.
func (e *ENR) GetIP6() *string {
	if e == nil {
		return nil
	}

	return e.IP6
}

// GetTCP4 returns the TCP port for IPv4 from the ENR record.
// Returns nil if the ENR is nil or the port is not present.
func (e *ENR) GetTCP4() *uint32 {
	if e == nil {
		return nil
	}

	return e.TCP4
}

// GetTCP6 returns the TCP port for IPv6 from the ENR record.
// Returns nil if the ENR is nil or the port is not present.
func (e *ENR) GetTCP6() *uint32 {
	if e == nil {
		return nil
	}

	return e.TCP6
}

// GetUDP4 returns the UDP port for IPv4 from the ENR record.
// Returns nil if the ENR is nil or the port is not present.
func (e *ENR) GetUDP4() *uint32 {
	if e == nil {
		return nil
	}

	return e.UDP4
}

// GetUDP6 returns the UDP port for IPv6 from the ENR record.
// Returns nil if the ENR is nil or the port is not present.
func (e *ENR) GetUDP6() *uint32 {
	if e == nil {
		return nil
	}

	return e.UDP6
}

// GetQUIC4 returns the QUIC port for IPv4 from the ENR record.
// Returns nil if the ENR is nil or the port is not present.
func (e *ENR) GetQUIC4() *uint32 {
	if e == nil {
		return nil
	}

	return e.QUIC4
}

// GetQUIC6 returns the QUIC port for IPv6 from the ENR record.
// Returns nil if the ENR is nil or the port is not present.
func (e *ENR) GetQUIC6() *uint32 {
	if e == nil {
		return nil
	}

	return e.QUIC6
}

// GetETH2 returns the Ethereum 2.0 public key from the ENR record.
// Returns nil if the ENR is nil or the key is not present.
func (e *ENR) GetETH2() *[]byte {
	if e == nil {
		return nil
	}

	return e.ETH2
}

// GetAttnets returns the attestation subnet bitfield from the ENR record.
// Returns nil if the ENR is nil or the bitfield is not present.
func (e *ENR) GetAttnets() *[]byte {
	if e == nil {
		return nil
	}

	return e.Attnets
}

// GetSyncnets returns the sync subnet bitfield from the ENR record.
// Returns nil if the ENR is nil or the bitfield is not present.
func (e *ENR) GetSyncnets() *[]byte {
	if e == nil {
		return nil
	}

	return e.Syncnets
}

// GetCGC returns the custody group count field from the ENR record.
// Returns nil if the ENR is nil or the CGC field is not present.
func (e *ENR) GetCGC() *[]byte {
	if e == nil {
		return nil
	}

	return e.CGC
}

// GetNFD returns the next fork digest field from the ENR record.
// Returns nil if the ENR is nil or the NFD field is not present.
func (e *ENR) GetNFD() *[]byte {
	if e == nil {
		return nil
	}

	return e.NFD
}

// parseSignature extracts the cryptographic signature from an ENR record.
// Returns a pointer to the signature bytes.
func parseSignature(node *enode.Node) *[]byte {
	signature := node.Record().Signature()

	return &signature
}

// parseSeq extracts the sequence number from an ENR record.
// Returns a pointer to the sequence number.
func parseSeq(node *enode.Node) *uint64 {
	seq := node.Seq()

	return &seq
}

// parseID extracts the identity scheme from an ENR record.
// Returns a pointer to the identity scheme string (e.g., "v4").
func parseID(node *enode.Node) *string {
	id := node.Record().IdentityScheme()

	return &id
}

// parseSecp256k1 extracts the secp256k1 public key from an ENR record.
// Returns a pointer to the public key bytes, or nil if not present.
func parseSecp256k1(node *enode.Node) *[]byte {
	field := s256raw{}
	err := node.Record().Load(&field)

	if err != nil {
		return nil
	}

	f := []byte(field)

	return &f
}

// parseIP4 extracts the IPv4 address from an ENR record.
// Returns a pointer to the IP address string, or nil if not present or unspecified.
func parseIP4(node *enode.Node) *string {
	ip := node.IP()
	if ip.IsUnspecified() || ip.String() == "<nil>" {
		return nil
	}

	f := ip.String()

	return &f
}

// parseIP6 extracts the IPv6 address from an ENR record.
// Returns a pointer to the IP address string, or nil if not present or unspecified.
func parseIP6(node *enode.Node) *string {
	var field enr.IPv6

	err := node.Record().Load(&field)
	if err != nil {
		return nil
	}

	ip := net.IP(field)
	if ip.IsUnspecified() || ip.String() == "<nil>" {
		return nil
	}

	f := ip.String()

	return &f
}

// parseTCP4 extracts the TCP port for IPv4 from an ENR record.
// Returns a pointer to the port number, or nil if not present or zero.
func parseTCP4(node *enode.Node) *uint32 {
	tcp := node.TCP()
	if tcp == 0 {
		return nil
	}

	field := uint32(tcp) //nolint:gosec // fine.

	return &field
}

// parseTCP6 extracts the TCP port for IPv6 from an ENR record.
// Returns a pointer to the port number, or nil if not present or zero.
func parseTCP6(node *enode.Node) *uint32 {
	var field enr.TCP6

	err := node.Record().Load(&field)
	if err != nil {
		return nil
	}

	f := uint32(field)

	if f == 0 {
		return nil
	}

	return &f
}

// parseUDP4 extracts the UDP port for IPv4 from an ENR record.
// Returns a pointer to the port number, or nil if not present or zero.
func parseUDP4(node *enode.Node) *uint32 {
	udp := node.UDP()
	if udp == 0 {
		return nil
	}

	field := uint32(udp) //nolint:gosec // fine.

	return &field
}

// parseUDP6 extracts the UDP port for IPv6 from an ENR record.
// Returns a pointer to the port number, or nil if not present or zero.
func parseUDP6(node *enode.Node) *uint32 {
	var field enr.UDP6

	err := node.Record().Load(&field)
	if err != nil {
		return nil
	}

	f := uint32(field)

	if f == 0 {
		return nil
	}

	return &f
}

// parseQUIC4 extracts the QUIC port for IPv4 from an ENR record.
// Returns a pointer to the port number, or nil if not present or zero.
func parseQUIC4(node *enode.Node) *uint32 {
	var field quic

	err := node.Record().Load(&field)
	if err != nil {
		return nil
	}

	f := uint32(field)

	if f == 0 {
		return nil
	}

	return &f
}

// parseQUIC6 extracts the QUIC port for IPv6 from an ENR record.
// Returns a pointer to the port number, or nil if not present or zero.
func parseQUIC6(node *enode.Node) *uint32 {
	var field quic6

	err := node.Record().Load(&field)
	if err != nil {
		return nil
	}

	f := uint32(field)

	if f == 0 {
		return nil
	}

	return &f
}

// parseETH2 extracts the Ethereum 2.0 public key from an ENR record.
// Returns a pointer to the public key bytes, or nil if not present.
func parseETH2(node *enode.Node) *[]byte {
	field := eth2{}

	err := node.Record().Load(&field)

	if err != nil {
		return nil
	}

	f := []byte(field)

	return &f
}

// parseAttnets extracts the attestation subnet bitfield from an ENR record.
// Returns a pointer to the bitfield bytes, or nil if not present.
func parseAttnets(node *enode.Node) *[]byte {
	field := attnets{}

	err := node.Record().Load(&field)

	if err != nil {
		return nil
	}

	f := []byte(field)

	return &f
}

// parseSyncnets extracts the sync subnet bitfield from an ENR record.
// Returns a pointer to the bitfield bytes, or nil if not present.
func parseSyncnets(node *enode.Node) *[]byte {
	field := syncnets{}

	err := node.Record().Load(&field)

	if err != nil {
		return nil
	}

	f := []byte(field)

	return &f
}

// parseCGC extracts the custody group count from an ENR record.
// Returns a pointer to the count bytes, or nil if not present.
func parseCGC(node *enode.Node) *[]byte {
	field := cgc{}

	err := node.Record().Load(&field)

	if err != nil {
		return nil
	}

	f := []byte(field)

	return &f
}

// parseNFD extracts the next fork digest from an ENR record.
// Returns a pointer to the digest bytes, or nil if not present.
func parseNFD(node *enode.Node) *[]byte {
	field := nfd{}

	err := node.Record().Load(&field)

	if err != nil {
		return nil
	}

	f := []byte(field)

	return &f
}

// parseNodeID extracts the node ID from an ENR record.
// Returns a pointer to the node ID string.
func parseNodeID(node *enode.Node) *string {
	f := node.ID().String()

	return &f
}

// parsePeerID derives the libp2p peer ID from the ENR record's public key.
// Returns a pointer to the peer ID string, or nil if derivation fails.
func parsePeerID(node *enode.Node) *string {
	key := node.Pubkey()
	pubkey := crypto.FromECDSAPub(key)

	sPubkey, err := libp2pcrypto.UnmarshalSecp256k1PublicKey(pubkey)
	if err != nil {
		return nil
	}

	peerID, err := peer.IDFromPublicKey(sPubkey)
	if err != nil {
		return nil
	}

	f := peerID.String()

	return &f
}
