package crawler

import (
	"errors"
	"fmt"
)

type CrawlError struct {
	Msg  string
	Type string
}

var (
	ErrCrawlTooSoon               = newCrawlError("too soon")
	ErrCrawlENRForkDigest         = newCrawlError("wrong enr fork digest")
	ErrCrawlStatusForkDigest      = newCrawlError("wrong fork digest in status message")
	ErrCrawlFailedToRequestStatus = newCrawlError("failed to request status")
	ErrCrawlIdentifyTimeout       = newCrawlError("identify protocol timeout")
)

func newCrawlError(msg string) *CrawlError {
	return &CrawlError{Msg: msg, Type: msg}
}

func (e *CrawlError) Error() string {
	return e.Msg
}

func (e *CrawlError) Add(msg string) *CrawlError {
	e.Msg = fmt.Sprintf("%s: %s", e.Msg, msg)

	return e
}

func (e *CrawlError) Unwrap() error {
	return errors.New(e.Msg)
}
