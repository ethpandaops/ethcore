package discovery

import (
	"fmt"

	"github.com/ethereum/go-ethereum/p2p/enode"
)

// ENRToEnode converts an ENR string to an enode URL string.
func ENRToEnode(enr string) (*enode.Node, error) {
	// Parse the ENR string
	node, err := enode.Parse(enode.ValidSchemes, enr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ENR: %v", err)
	}

	return node, nil
}
