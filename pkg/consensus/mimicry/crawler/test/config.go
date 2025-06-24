package crawler_test

import "time"

const (
	// Network configuration.
	DefaultEnclaveName        = "ethcore-test-enclave"
	DefaultNetworkTimeout     = 60 * time.Second
	GenesisWaitTimeout        = 2 * time.Minute
	BeaconHealthCheckInterval = 250 * time.Millisecond

	// Crawler configuration.
	CrawlerDialConcurrency = 10
	CrawlerCooloffDuration = 1 * time.Second
	CrawlerTimeout         = 30 * time.Second
)
