package crawler

import (
	"errors"
	"time"

	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/host"
	"github.com/ethpandaops/ethcore/pkg/ethereum"

	perrors "github.com/pkg/errors"
)

// Config is the configuration for the Crawler.
type Config struct {
	Node             *host.Config `yaml:"node"`
	UserAgent        string
	Namespace        string
	Beacon           *ethereum.Config `yaml:"ethereum"`
	DialConcurrency  int              `yaml:"dialConcurrency" default:"10"`
	DialTimeout      time.Duration    `yaml:"dialTimeout" default:"5s"`
	CooloffDuration  time.Duration    `yaml:"cooloffDuration" default:"600s"`
	MaxRetryAttempts int              `yaml:"maxRetryAttempts" default:"3"`
	RetryBackoff     time.Duration    `yaml:"retryBackoff" default:"10s"`
}

// Validate validates the CrawlerConfig.
func (c *Config) Validate() error {
	if c.Node == nil {
		return errors.New("node is required")
	}

	if c.Beacon == nil {
		return errors.New("beacon config is required")
	}

	if c.Namespace == "" {
		return errors.New("namespace is required")
	}

	if err := c.Node.Validate(); err != nil {
		return perrors.Wrap(err, "node config is invalid")
	}

	if err := c.Beacon.Validate(); err != nil {
		return perrors.Wrap(err, "beacon config is invalid")
	}

	if c.DialConcurrency <= 0 {
		return errors.New("dial concurrency must be greater than 0")
	}

	return nil
}
