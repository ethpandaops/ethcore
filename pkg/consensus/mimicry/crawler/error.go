package crawler

type CrawlError string

const (
	ErrCrawlTooSoon                 = CrawlError("too soon")
	ErrCrawlFailedToConnect         = CrawlError("failed to connect")
	ErrCrawlENRForkDigest           = CrawlError("wrong enr fork digest")
	ErrCrawlStatusForkDigest        = CrawlError("wrong fork digest in status message")
	ErrCrawlFailedToRequestStatus   = CrawlError("failed to request status")
	ErrCrawlFailedToRequestMetadata = CrawlError("failed to request metadata")
	ErrCrawlFailedToCrawl           = CrawlError("failed to crawl")
)

func (e CrawlError) Error() string {
	return string(e)
}

type StatusError string

func (e StatusError) Error() string {
	return string(e)
}

const (
	ErrStatusForkDigest = StatusError("wrong fork digest in status message")
)
