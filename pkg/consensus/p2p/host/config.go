package host

// Config represents host configuration.
type Config struct {
	// DataStream is the data stream to use for events.
	DataStream DataStream

	// OutputType specifies the output format for events.
	OutputType DataStreamOutputType

	// MaxPeers is the maximum number of peers to connect to.
	MaxPeers int

	// UserAgent is the user agent string to use.
	UserAgent string
}

// NewConfig creates a new Config with default values.
func NewConfig() *Config {
	return &Config{
		MaxPeers: 50,
	}
}

// WithDataStream sets the data stream for the config.
func (c *Config) WithDataStream(ds DataStream) *Config {
	c.DataStream = ds

	return c
}

// WithOutputType sets the output type for the config.
func (c *Config) WithOutputType(ot DataStreamOutputType) *Config {
	c.OutputType = ot

	return c
}

// WithMaxPeers sets the maximum number of peers for the config.
func (c *Config) WithMaxPeers(maxPeers int) *Config {
	c.MaxPeers = maxPeers

	return c
}

// WithUserAgent sets the user agent for the config.
func (c *Config) WithUserAgent(userAgent string) *Config {
	c.UserAgent = userAgent

	return c
}
