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
	ErrCrawlTooSoon                 = newCrawlError("too soon")
	ErrCrawlFailedToConnect         = newCrawlError("failed to connect")
	ErrCrawlENRForkDigest           = newCrawlError("wrong enr fork digest")
	ErrCrawlStatusForkDigest        = newCrawlError("wrong fork digest in status message")
	ErrCrawlFailedToRequestStatus   = newCrawlError("failed to request status")
	ErrCrawlFailedToRequestMetadata = newCrawlError("failed to request metadata")
	ErrCrawlFailedToCrawl           = newCrawlError("failed to crawl")
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

type StatusError struct {
	Msg  string
	Type string
}

var (
	ErrStatusForkDigest = newStatusError("wrong fork digest in status message")
)

func newStatusError(msg string) *StatusError {
	return &StatusError{Msg: msg, Type: msg}
}

func (e *StatusError) Error() string {
	return e.Msg
}

func (e *StatusError) Add(msg string) *StatusError {
	e.Msg = fmt.Sprintf("%s: %s", e.Msg, msg)

	return e
}

func (e *StatusError) Unwrap() error {
	return errors.New(e.Msg)
}
