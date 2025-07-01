package crawler

import (
	"fmt"
)

// Error type constants for categorization.
const (
	ErrCrawlNetworkMismatchType      = "network_mismatch"
	ErrCrawlIdentifyTimeoutType      = "identify_timeout"
	ErrCrawlTimeoutType              = "timeout"
	ErrCrawlProtocolNotSupportedType = "protocol_not_supported"
)

// CrawlError represents an error that occurred during the crawling process.
// It provides both a human-readable message and a machine-readable type for metrics.
type CrawlError struct {
	// Msg is the human-readable error message.
	Msg string
	// ErrorType is a machine-readable error type used for categorization and metrics.
	ErrorType string
}

// Pre-defined crawl errors for common failure scenarios.
var (
	// ErrCrawlTooSoon indicates that we attempted to crawl a peer too soon after the last attempt.
	ErrCrawlTooSoon = CrawlError{Msg: "too soon", ErrorType: "too_soon"}
	// ErrCrawlENRForkDigest indicates that the peer's ENR contains a fork digest that doesn't match our network.
	ErrCrawlENRForkDigest = CrawlError{Msg: "wrong enr fork digest", ErrorType: ErrCrawlNetworkMismatchType}
	// ErrCrawlStatusForkDigest indicates that the peer's status message contains a fork digest that doesn't match our network.
	ErrCrawlStatusForkDigest = CrawlError{Msg: "wrong fork digest in status message", ErrorType: ErrCrawlNetworkMismatchType}
	// ErrCrawlFailedToRequestStatus indicates that we failed to request status from the peer.
	ErrCrawlFailedToRequestStatus = CrawlError{Msg: "failed to request status", ErrorType: ErrCrawlTimeoutType}
	// ErrCrawlFailedToRequestMetadata indicates that we failed to request metadata from the peer.
	ErrCrawlFailedToRequestMetadata = CrawlError{Msg: "failed to request metadata", ErrorType: ErrCrawlTimeoutType}
	// ErrCrawlIdentifyTimeout indicates that the libp2p identify protocol timed out.
	ErrCrawlIdentifyTimeout = CrawlError{Msg: "identify protocol timeout", ErrorType: ErrCrawlIdentifyTimeoutType}
)

// Error implements the error interface.
func (e CrawlError) Error() string {
	return e.Msg
}

// WithDetails creates a new CrawlError with additional details appended to the message.
// This method returns a new error instance to avoid mutation and potential race conditions.
func (e CrawlError) WithDetails(details string) CrawlError {
	return CrawlError{
		Msg:       fmt.Sprintf("%s: %s", e.Msg, details),
		ErrorType: e.ErrorType,
	}
}

// Type returns the error type for metrics and categorization.
func (e CrawlError) Type() string {
	return e.ErrorType
}

// newCrawlError creates a new CrawlError with the given message.
// The type is derived from the message by replacing spaces with underscores.
func newCrawlError(msg string) CrawlError {
	return CrawlError{Msg: msg, ErrorType: msg}
}
