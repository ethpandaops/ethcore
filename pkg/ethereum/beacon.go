// Package ethereum provides Ethereum beacon node functionality
package ethereum

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/attestantio/go-eth2-client/spec"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/beacon/pkg/beacon"
	"github.com/ethpandaops/ethcore/pkg/ethereum/beacon/services"
	"github.com/ethpandaops/ethwallclock"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// BeaconNode represents a connection to an Ethereum beacon node and manages any associated services (eg: metadata, etc).
type BeaconNode struct {
	config           *Config
	log              logrus.FieldLogger
	beacon           beacon.Node
	metadataSvc      *services.MetadataService
	healthy          atomic.Bool
	onReadyCallbacks []func(ctx context.Context) error
}

// NewBeaconNode creates a new beacon node instance with the given configuration. It initializes any services and
// configures the beacon subscriptions.
func NewBeaconNode(
	log logrus.FieldLogger,
	namespace string,
	config *Config,
	opts *Options,
) (*BeaconNode, error) {
	// Use provided options or defaults.
	var beaconOpts *beacon.Options
	if opts != nil && opts.Options != nil {
		beaconOpts = opts.Options
	} else {
		beaconOpts = beacon.DefaultOptions()
	}

	// Create the beacon node.
	node := beacon.NewNode(log, &beacon.Config{
		Addr:    config.BeaconNodeAddress,
		Headers: config.BeaconNodeHeaders,
	}, namespace, *beaconOpts)

	// Initialize services.
	metadata := services.NewMetadataService(log, node, config.NetworkOverride)

	return &BeaconNode{
		log:         log.WithField("module", "ethcore/ethereum/beacon"),
		config:      config,
		beacon:      node,
		metadataSvc: &metadata,
	}, nil
}

// Start starts the beacon node and waits for it to be ready.
func (b *BeaconNode) Start(ctx context.Context) error {
	errs := make(chan error, 1)
	ready := make(chan struct{})

	// Register callback for when the node becomes healthy
	b.beacon.OnFirstTimeHealthy(ctx, func(ctx context.Context, event *beacon.FirstTimeHealthyEvent) error {
		b.log.Debug("Upstream beacon node is healthy")

		b.healthy.Store(true)

		// Ensure the beacon is actually reporting as healthy before proceeding
		// This handles any timing issues between the event and the health status
		b.log.Info("Awaiting healthy status from beacon node")

		for !b.beacon.Healthy() {
			select {
			case <-time.After(100 * time.Millisecond):
				b.log.Debug("Awaiting healthy status from beacon node")
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Use the existing startServices method
		if err := b.startServices(ctx, errs); err != nil {
			return err
		}

		// Services are now ready, run onReady callbacks
		go func() {
			b.log.Info("All services are ready")

			// Run onReady callbacks
			for _, callback := range b.onReadyCallbacks {
				if err := callback(ctx); err != nil {
					errs <- fmt.Errorf("failed to run on ready callback: %w", err)

					return
				}
			}

			close(ready)
		}()

		return nil
	})

	// Start the beacon node asynchronously
	b.beacon.StartAsync(ctx)

	// Wait for either completion, error, or context cancellation
	select {
	case <-ready:
		return nil
	case err := <-errs:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop gracefully shuts down the beacon node and its services.
func (b *BeaconNode) Stop(ctx context.Context) error {
	b.log.Info("Stopping beacon node")
	b.healthy.Store(false)

	b.log.WithField("service", b.metadataSvc.Name()).Info("Stopping service")

	if err := b.metadataSvc.Stop(ctx); err != nil {
		b.log.WithError(err).WithField("service", b.metadataSvc.Name()).Error("Failed to stop service")
	}

	if err := b.beacon.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop beacon node: %w", err)
	}

	return nil
}

// Synced checks if the beacon node is synced and ready
// It verifies sync state, wallclock, and service readiness.
func (b *BeaconNode) Synced(ctx context.Context) error {
	status := b.beacon.Status()
	if status == nil {
		return errors.New("missing beacon status")
	}

	wallclock := b.metadataSvc.GetWallclock()
	if wallclock == nil {
		return errors.New("missing wallclock")
	}

	if err := b.metadataSvc.Ready(ctx); err != nil {
		return errors.Wrapf(err, "service %s is not ready", b.metadataSvc.Name())
	}

	return nil
}

// Node returns the underlying beacon node instance.
func (b *BeaconNode) Node() beacon.Node {
	return b.beacon
}

// Metadata returns the metadata service instance.
func (b *BeaconNode) Metadata() *services.MetadataService {
	return b.metadataSvc
}

// OnReady registers a callback to be executed when the beacon node is ready.
func (b *BeaconNode) OnReady(callback func(ctx context.Context) error) {
	b.onReadyCallbacks = append(b.onReadyCallbacks, callback)
}

// GetWallclock returns the wallclock for the beacon chain.
func (b *BeaconNode) GetWallclock() *ethwallclock.EthereumBeaconChain {
	return b.metadataSvc.GetWallclock()
}

// GetSlot returns the wallclock slot for a given slot number.
func (b *BeaconNode) GetSlot(slot uint64) ethwallclock.Slot {
	return b.metadataSvc.GetWallclock().Slots().FromNumber(slot)
}

// GetEpoch returns the wallclock epoch for a given slot number.
func (b *BeaconNode) GetEpoch(epoch uint64) ethwallclock.Epoch {
	return b.metadataSvc.GetWallclock().Epochs().FromNumber(epoch)
}

// GetEpochFromSlot returns the wallclock epoch for a given slot.
func (b *BeaconNode) GetEpochFromSlot(slot uint64) ethwallclock.Epoch {
	return b.metadataSvc.GetWallclock().Epochs().FromSlot(slot)
}

// IsHealthy returns whether the node is healthy.
func (b *BeaconNode) IsHealthy() bool {
	return b.healthy.Load()
}

// GetCurrentBlobSchedule returns the current blob schedule for the current epoch.
func (b *BeaconNode) GetCurrentBlobSchedule() (*BlobScheduleEntry, error) {
	spec, err := b.beacon.Spec()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get spec")
	}

	_, epoch, err := b.beacon.Wallclock().Now()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get wallclock")
	}

	currentEpoch := epoch.Number()

	// Use the spec's built-in blob schedule logic.
	maxBlobs := spec.GetMaxBlobsPerBlock(phase0.Epoch(currentEpoch))

	// Find the most recent epoch from blob schedule that is <= currentEpoch.
	var scheduleEpoch uint64

	for _, entry := range spec.BlobSchedule {
		if uint64(entry.Epoch) <= currentEpoch {
			// Keep the highest epoch that is still <= currentEpoch.
			if uint64(entry.Epoch) > scheduleEpoch {
				scheduleEpoch = uint64(entry.Epoch)
			}
		}
	}

	return &BlobScheduleEntry{
		Epoch:            scheduleEpoch,
		MaxBlobsPerBlock: maxBlobs,
	}, nil
}

func (b *BeaconNode) ForkDigest() (phase0.ForkDigest, error) {
	genesis := b.Metadata().GetGenesis()
	if genesis == nil {
		return phase0.ForkDigest{}, errors.New("missing genesis")
	}

	beaconSpec, err := b.beacon.Spec()
	if err != nil {
		return phase0.ForkDigest{}, errors.Wrap(err, "failed to get spec")
	}

	_, epoch, err := b.beacon.Wallclock().Now()
	if err != nil {
		return phase0.ForkDigest{}, errors.Wrap(err, "failed to get wallclock")
	}

	current, err := beaconSpec.ForkEpochs.CurrentFork(phase0.Epoch(epoch.Number()))
	if err != nil {
		return phase0.ForkDigest{}, errors.Wrap(err, "failed to get current fork")
	}

	forkVersion := [4]byte{}
	version := current.Version

	if len(version) >= 2 && version[:2] == "0x" {
		decoded, errr := hex.DecodeString(version[2:])
		if errr != nil {
			return phase0.ForkDigest{}, errors.Wrap(errr, "failed to decode fork version")
		}

		copy(forkVersion[:], decoded)
	} else {
		copy(forkVersion[:], version)
	}

	// Get blob parameters for Fulu fork and later, nil for earlier forks.
	var blobParams *BlobScheduleEntry

	if current.Name >= spec.DataVersionFulu {
		var blobErr error

		blobParams, blobErr = b.GetCurrentBlobSchedule()
		if blobErr != nil {
			return phase0.ForkDigest{}, errors.Wrap(blobErr, "failed to get blob schedule entry")
		}
	}

	// Compute fork digest with optional blob parameters.
	forkDigest := ComputeForkDigest(genesis.GenesisValidatorsRoot, forkVersion, blobParams)

	return forkDigest, nil
}

func (b *BeaconNode) startServices(ctx context.Context, errs chan error) error {
	serviceReady := make(chan struct{})

	b.metadataSvc.OnReady(ctx, func(ctx context.Context) error {
		b.log.WithField("service", b.metadataSvc.Name()).Debug("Service is ready")

		hashed, err := b.metadataSvc.GetNodeIDHash()
		if err != nil {
			return err
		}

		b.log.WithFields(logrus.Fields{
			"node_id":    hashed,
			"network_id": b.metadataSvc.Network.ID,
			"network":    b.metadataSvc.Network.Name,
			"hash":       hashed,
		}).Info("Detected network and node ID hash")

		close(serviceReady)

		return nil
	})

	b.log.WithField("service", b.metadataSvc.Name()).Debug("Starting service")

	if err := b.metadataSvc.Start(ctx); err != nil {
		errs <- fmt.Errorf("failed to start service: %w", err)

		return err
	}

	b.log.WithField("service", b.metadataSvc.Name()).Debug("Waiting for service to be ready")

	// Wait for the service to actually be ready before returning
	select {
	case <-serviceReady:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
