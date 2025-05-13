package services

import "context"

// Name is the name of the service.
type Name string

type Service interface {
	// Start the service.
	Start(ctx context.Context) error
	// Stop the service.
	Stop(ctx context.Context) error
	// Check if the service is ready.
	Ready(ctx context.Context) error
	// Register a callback to be called when the service is ready.
	OnReady(ctx context.Context, cb func(ctx context.Context) error)
	// Get the name of the service.
	Name() Name
}
