package beacon

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

// GenerateBeaconTraceID generates a unique trace ID for a beacon node based on their address.
func GenerateBeaconTraceID(address string) (string, error) {
	traceIDs, err := GenerateBeaconTraceIDs([]string{address})
	if err != nil {
		return "", err
	}

	return traceIDs[0], nil
}

// GenerateBeaconTraceIDs generates unique trace IDs for beacon nodes based on their addresses.
func GenerateBeaconTraceIDs(addresses []string) ([]string, error) {
	if len(addresses) == 0 {
		return nil, errors.New("no addresses provided")
	}

	traceIDs := make([]string, len(addresses))
	uniqueIDs := make(map[string]bool)

	for i, address := range addresses {
		// Generate a hash of the address
		hash := sha256.Sum256([]byte(address))
		baseID := base64.URLEncoding.EncodeToString(hash[:])[:8]

		// Ensure uniqueness
		id := baseID
		counter := 1

		for uniqueIDs[id] {
			id = fmt.Sprintf("%s-%d", baseID, counter)
			counter++
		}

		uniqueIDs[id] = true
		traceIDs[i] = id
	}

	return traceIDs, nil
}
