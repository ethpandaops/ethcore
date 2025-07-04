package v1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatus_String(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected string
	}{
		{
			name:     "success",
			status:   StatusSuccess,
			expected: "success",
		},
		{
			name:     "invalid_request",
			status:   StatusInvalidRequest,
			expected: "invalid_request",
		},
		{
			name:     "server_error",
			status:   StatusServerError,
			expected: "server_error",
		},
		{
			name:     "resource_unavailable",
			status:   StatusResourceUnavailable,
			expected: "resource_unavailable",
		},
		{
			name:     "rate_limited",
			status:   StatusRateLimited,
			expected: "rate_limited",
		},
		{
			name:     "unknown_status",
			status:   Status(99),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStatus_IsError(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected bool
	}{
		{
			name:     "success_is_not_error",
			status:   StatusSuccess,
			expected: false,
		},
		{
			name:     "invalid_request_is_error",
			status:   StatusInvalidRequest,
			expected: true,
		},
		{
			name:     "server_error_is_error",
			status:   StatusServerError,
			expected: true,
		},
		{
			name:     "resource_unavailable_is_error",
			status:   StatusResourceUnavailable,
			expected: true,
		},
		{
			name:     "rate_limited_is_error",
			status:   StatusRateLimited,
			expected: true,
		},
		{
			name:     "unknown_status_is_error",
			status:   Status(99),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.IsError()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultServiceConfig(t *testing.T) {
	config := DefaultServiceConfig()

	// Check HandlerOptions
	assert.Equal(t, 30*time.Second, config.HandlerOptions.RequestTimeout)
	assert.False(t, config.HandlerOptions.EnableMetrics)
	assert.Nil(t, config.HandlerOptions.Encoder)
	assert.Nil(t, config.HandlerOptions.Compressor)

	// Check ClientConfig
	assert.Equal(t, 30*time.Second, config.ClientConfig.DefaultTimeout)
	assert.Equal(t, 3, config.ClientConfig.MaxRetries)
	assert.Equal(t, 1*time.Second, config.ClientConfig.RetryDelay)
	assert.False(t, config.ClientConfig.EnableMetrics)
}

func TestProtocolConfig(t *testing.T) {
	config := ProtocolConfig{
		ID:              "/test/1.0.0",
		Version:         "1.0.0",
		MaxRequestSize:  1024,
		MaxResponseSize: 2048,
		Timeout:         5 * time.Second,
	}

	assert.Equal(t, "/test/1.0.0", string(config.ID))
	assert.Equal(t, "1.0.0", config.Version)
	assert.Equal(t, uint64(1024), config.MaxRequestSize)
	assert.Equal(t, uint64(2048), config.MaxResponseSize)
	assert.Equal(t, 5*time.Second, config.Timeout)
}

func TestRequestMetadata(t *testing.T) {
	now := time.Now()
	meta := RequestMetadata{
		Protocol:    "/test/1.0.0",
		PeerID:      "peer123",
		RequestedAt: now,
		Size:        256,
	}

	assert.Equal(t, "/test/1.0.0", string(meta.Protocol))
	assert.Equal(t, "peer123", meta.PeerID)
	assert.Equal(t, now, meta.RequestedAt)
	assert.Equal(t, 256, meta.Size)
}

func TestResponseMetadata(t *testing.T) {
	now := time.Now()
	meta := ResponseMetadata{
		Protocol:    "/test/1.0.0",
		PeerID:      "peer123",
		Status:      StatusSuccess,
		RespondedAt: now,
		Size:        512,
		Duration:    100 * time.Millisecond,
	}

	assert.Equal(t, "/test/1.0.0", string(meta.Protocol))
	assert.Equal(t, "peer123", meta.PeerID)
	assert.Equal(t, StatusSuccess, meta.Status)
	assert.Equal(t, now, meta.RespondedAt)
	assert.Equal(t, 512, meta.Size)
	assert.Equal(t, 100*time.Millisecond, meta.Duration)
}

func TestHandlerOptions(t *testing.T) {
	encoder := &mockEncoder{}
	compressor := &mockCompressor{}

	opts := HandlerOptions{
		Encoder:        encoder,
		Compressor:     compressor,
		RequestTimeout: 10 * time.Second,
		EnableMetrics:  true,
	}

	assert.Equal(t, encoder, opts.Encoder)
	assert.Equal(t, compressor, opts.Compressor)
	assert.Equal(t, 10*time.Second, opts.RequestTimeout)
	assert.True(t, opts.EnableMetrics)
}

func TestClientConfig(t *testing.T) {
	config := ClientConfig{
		DefaultTimeout: 20 * time.Second,
		MaxRetries:     5,
		RetryDelay:     2 * time.Second,
		EnableMetrics:  true,
	}

	assert.Equal(t, 20*time.Second, config.DefaultTimeout)
	assert.Equal(t, 5, config.MaxRetries)
	assert.Equal(t, 2*time.Second, config.RetryDelay)
	assert.True(t, config.EnableMetrics)
}

func TestRequestOptions(t *testing.T) {
	encoder := &mockEncoder{}
	compressor := &mockCompressor{}

	opts := RequestOptions{
		Encoder:    encoder,
		Compressor: compressor,
		Timeout:    15 * time.Second,
	}

	assert.Equal(t, encoder, opts.Encoder)
	assert.Equal(t, compressor, opts.Compressor)
	assert.Equal(t, 15*time.Second, opts.Timeout)
}

func TestServiceConfig(t *testing.T) {
	encoder := &mockEncoder{}
	compressor := &mockCompressor{}

	config := ServiceConfig{
		HandlerOptions: HandlerOptions{
			Encoder:        encoder,
			Compressor:     compressor,
			RequestTimeout: 30 * time.Second,
			EnableMetrics:  true,
		},
		ClientConfig: ClientConfig{
			DefaultTimeout: 30 * time.Second,
			MaxRetries:     3,
			RetryDelay:     1 * time.Second,
			EnableMetrics:  true,
		},
	}

	// Verify HandlerOptions
	assert.Equal(t, encoder, config.HandlerOptions.Encoder)
	assert.Equal(t, compressor, config.HandlerOptions.Compressor)
	assert.Equal(t, 30*time.Second, config.HandlerOptions.RequestTimeout)
	assert.True(t, config.HandlerOptions.EnableMetrics)

	// Verify ClientConfig
	assert.Equal(t, 30*time.Second, config.ClientConfig.DefaultTimeout)
	assert.Equal(t, 3, config.ClientConfig.MaxRetries)
	assert.Equal(t, 1*time.Second, config.ClientConfig.RetryDelay)
	assert.True(t, config.ClientConfig.EnableMetrics)
}

func TestErrorConstants(t *testing.T) {
	// Test that error constants are not nil
	require.NotNil(t, ErrInvalidRequest)
	require.NotNil(t, ErrInvalidResponse)
	require.NotNil(t, ErrStreamReset)
	require.NotNil(t, ErrTimeout)
	require.NotNil(t, ErrNoHandler)
	require.NotNil(t, ErrHandlerExists)
	require.NotNil(t, ErrServiceStopped)
	require.NotNil(t, ErrMaxSizeExceeded)

	// Test error messages
	assert.Contains(t, ErrInvalidRequest.Error(), "invalid request")
	assert.Contains(t, ErrInvalidResponse.Error(), "invalid response")
	assert.Contains(t, ErrStreamReset.Error(), "stream reset")
	assert.Contains(t, ErrTimeout.Error(), "timed out")
	assert.Contains(t, ErrNoHandler.Error(), "no handler")
	assert.Contains(t, ErrHandlerExists.Error(), "handler already registered")
	assert.Contains(t, ErrServiceStopped.Error(), "service stopped")
	assert.Contains(t, ErrMaxSizeExceeded.Error(), "max size exceeded")
}
