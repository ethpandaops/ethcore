package enr

type ENR struct {
	// Enr is the enr of the node record.
	Enr string
	// Signature is the cryptographic signature of record contents
	Signature *[]byte
	// Seq is the sequence number, a 64-bit unsigned integer.
	// Nodes should increase the number whenever the record changes and republish the record
	Seq *uint64
	// ID is the name of identity scheme, e.g. “v4”
	ID *string
	// NodeID is the node ID of the node record.
	NodeID *string
	// PeerID is the peer ID of the node record.
	PeerID *string
	// Secp256k1 is the secp256k1 public key of the node record.
	Secp256k1 *[]byte
	// IP4 is the IPv4 address of the node record.
	IP4 *string
	// IP6 is the IPv6 address of the node record.
	IP6 *string
	// TCP4 is the TCP port of the node record.
	TCP4 *uint32
	// TCP6 is the TCP port of the node record.
	TCP6 *uint32
	// UDP4 is the UDP port of the node record.
	UDP4 *uint32
	// UDP6 is the UDP port of the node record.
	UDP6 *uint32
	// QUIC4 is the QUIC port for IPv4 of the node record.
	QUIC4 *uint32
	// QUIC6 is the QUIC port for IPv6 of the node record.
	QUIC6 *uint32
	// Eth2 is the eth2 public key of the node record.
	ETH2 *[]byte
	// Attnets is the attestation subnet bitfield of the node record.
	Attnets *[]byte
	// Syncnets is the sync subnet bitfield of the node record.
	Syncnets *[]byte
	// CGC is the custody group count of the node record.
	CGC *[]byte
	// NFD is the next fork digest of the next scheduled fork.
	NFD *[]byte
}
