package mimicry

type Mode string

const (
	// ModeDiscovery is the mode where the node is only discovering peers. Once a peer connection is established
	// and we get the `status` from the peer we will disconnect them.
	ModeDiscovery Mode = "discovery"
)
