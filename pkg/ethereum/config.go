package ethereum

import "errors"

// Config defines the configuration for the Ethereum beacon node.
type Config struct {
	// The address of the Beacon node to connect to
	BeaconNodeAddress string `yaml:"beaconNodeAddress"`
	// BeaconNodeHeaders is a map of headers to send to the beacon node.
	BeaconNodeHeaders map[string]string `yaml:"beaconNodeHeaders"`
	// BeaconSubscriptions is a list of beacon subscriptions to subscribe to.
	BeaconSubscriptions *[]string `yaml:"beaconSubscriptions"`
	// NetworkOverride is an optional network name to use instead of what's reported by the beacon node
	NetworkOverride string `yaml:"networkOverride,omitempty"`
}

// Validate checks the configuration for the beacon node.
func (c *Config) Validate() error {
	if c.BeaconNodeAddress == "" {
		return errors.New("beaconNodeAddress is required")
	}

	return nil
}
