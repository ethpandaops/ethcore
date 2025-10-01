package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Tracer provides OpenTelemetry tracing capabilities.
type Tracer interface {
	// Start creates a new span and returns a context containing the span.
	Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span)

	// TracerProvider returns the underlying tracer provider.
	TracerProvider() trace.TracerProvider
}

// Meter provides OpenTelemetry metrics capabilities.
type Meter interface {
	// Int64Counter creates a new int64 counter instrument.
	Int64Counter(name string, opts ...metric.Int64CounterOption) (metric.Int64Counter, error)

	// Int64Histogram creates a new int64 histogram instrument.
	Int64Histogram(name string, opts ...metric.Int64HistogramOption) (metric.Int64Histogram, error)

	// Int64UpDownCounter creates a new int64 up-down counter instrument.
	Int64UpDownCounter(name string, opts ...metric.Int64UpDownCounterOption) (metric.Int64UpDownCounter, error)

	// Float64Counter creates a new float64 counter instrument.
	Float64Counter(name string, opts ...metric.Float64CounterOption) (metric.Float64Counter, error)

	// Float64Histogram creates a new float64 histogram instrument.
	Float64Histogram(name string, opts ...metric.Float64HistogramOption) (metric.Float64Histogram, error)

	// Float64UpDownCounter creates a new float64 up-down counter instrument.
	Float64UpDownCounter(name string, opts ...metric.Float64UpDownCounterOption) (metric.Float64UpDownCounter, error)

	// RegisterCallback registers a callback that will be called when the meter is read.
	RegisterCallback(callback metric.Callback, instruments ...metric.Observable) (metric.Registration, error)

	// MeterProvider returns the underlying meter provider.
	MeterProvider() metric.MeterProvider
}

// TelemetryProvider provides both tracing and metrics capabilities.
type TelemetryProvider interface {
	// Tracer returns a tracer with the given name.
	Tracer(name string, opts ...trace.TracerOption) Tracer

	// Meter returns a meter with the given name.
	Meter(name string, opts ...metric.MeterOption) Meter

	// Shutdown gracefully shuts down the telemetry provider.
	Shutdown(ctx context.Context) error
}

// Attributes provides common attribute key-value pairs for telemetry.
type Attributes struct {
	// Network represents the network name (e.g., "mainnet", "goerli").
	Network string

	// NodeID represents the node identifier.
	NodeID string

	// Version represents the software version.
	Version string

	// Additional custom attributes.
	Custom []attribute.KeyValue
}

// ToOTEL converts Attributes to OpenTelemetry attributes.
func (a Attributes) ToOTEL() []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("network", a.Network),
		attribute.String("node_id", a.NodeID),
		attribute.String("version", a.Version),
	}

	attrs = append(attrs, a.Custom...)

	return attrs
}
