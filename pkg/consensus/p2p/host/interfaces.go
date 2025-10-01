package host

import (
	"context"

	"github.com/ethpandaops/ethcore/pkg/consensus/p2p/events"
)

// DataStream represents an interface for handling data streams.
type DataStream interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	PutRecord(ctx context.Context, event *events.TraceEvent) error
	Type() DataStreamType
	OutputType() DataStreamOutputType
}

// EventCallback is a function type for handling events.
type EventCallback func(ctx context.Context, event *events.TraceEvent)

// DataStreamType represents the type of data stream.
type DataStreamType int

const (
	// DataStreamTypeKinesis represents a Kinesis data stream.
	DataStreamTypeKinesis DataStreamType = iota
	// DataStreamTypeCallback represents a callback-based data stream.
	DataStreamTypeCallback
	// DataStreamTypeLogger represents a logger data stream.
	DataStreamTypeLogger
	// DataStreamTypeS3 represents an S3 data stream.
	DataStreamTypeS3
	// DataStreamTypeNoop represents a no-op data stream.
	DataStreamTypeNoop
)

// String returns the string representation of the DataStreamType.
func (ds DataStreamType) String() string {
	switch ds {
	case DataStreamTypeLogger:
		return "logger"
	case DataStreamTypeKinesis:
		return "kinesis"
	case DataStreamTypeCallback:
		return "callback"
	case DataStreamTypeS3:
		return "s3"
	case DataStreamTypeNoop:
		return "noop"
	default:
		return "logger"
	}
}

// DataStreamOutputType is the output type of the data stream.
type DataStreamOutputType int

const (
	// DataStreamOutputTypeKinesis outputs the data stream decorated with metadata and in a format ingested by Kinesis.
	DataStreamOutputTypeKinesis DataStreamOutputType = iota
	// DataStreamOutputTypeFull outputs the data stream decorated with metadata and containing the raw/full event data.
	DataStreamOutputTypeFull
	// DataStreamOutputTypeParquet outputs the trace events formatted into a simplified parquet columns style.
	DataStreamOutputTypeParquet
)
