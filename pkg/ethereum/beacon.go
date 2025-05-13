package ethereum

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/beacon/pkg/beacon"
	"github.com/ethpandaops/ethcore/pkg/ethereum/beacon/services"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type BeaconNode struct {
	config *Config
	log    logrus.FieldLogger

	beacon beacon.Node

	services []services.Service

	onReadyCallbacks []func(ctx context.Context) error
}

func NewBeaconNode(ctx context.Context, name string, config *Config, log logrus.FieldLogger, namespace string, opts beacon.Options) (*BeaconNode, error) {
	node := beacon.NewNode(log, &beacon.Config{
		Name:    name,
		Addr:    config.BeaconNodeAddress,
		Headers: config.BeaconNodeHeaders,
	}, namespace, opts)

	metadata := services.NewMetadataService(log, node, config.Network)

	svcs := []services.Service{
		&metadata,
	}

	return &BeaconNode{
		config:   config,
		log:      log.WithField("module", "ethcore/ethereum/beacon"),
		beacon:   node,
		services: svcs,
	}, nil
}

func (b *BeaconNode) Start(ctx context.Context) error {
	errs := make(chan error, 1)

	go func() {
		wg := sync.WaitGroup{}

		for _, service := range b.services {
			wg.Add(1)

			service.OnReady(ctx, func(ctx context.Context) error {
				b.log.WithField("service", service.Name()).Info("Service is ready")

				wg.Done()

				return nil
			})

			b.log.WithField("service", service.Name()).Info("Starting service")

			if err := service.Start(ctx); err != nil {
				errs <- fmt.Errorf("failed to start service: %w", err)
			}

			wg.Wait()
		}

		b.log.Info("All services are ready")

		for _, callback := range b.onReadyCallbacks {
			if err := callback(ctx); err != nil {
				errs <- fmt.Errorf("failed to run on ready callback: %w", err)
			}
		}
	}()

	if err := b.beacon.Start(ctx); err != nil {
		return err
	}

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *BeaconNode) Node() beacon.Node {
	return b.beacon
}

func (b *BeaconNode) getServiceByName(name services.Name) (services.Service, error) {
	for _, service := range b.services {
		if service.Name() == name {
			return service, nil
		}
	}

	return nil, errors.New("service not found")
}

func (b *BeaconNode) Metadata() *services.MetadataService {
	service, err := b.getServiceByName("metadata")
	if err != nil {
		// This should never happen. If it does, good luck.
		return nil
	}

	metadata, ok := service.(*services.MetadataService)
	if !ok {
		return nil
	}

	return metadata
}

func (b *BeaconNode) OnReady(_ context.Context, callback func(ctx context.Context) error) {
	b.onReadyCallbacks = append(b.onReadyCallbacks, callback)
}

func (b *BeaconNode) Synced(ctx context.Context) error {
	status := b.beacon.Status()
	if status == nil {
		return errors.New("missing beacon status")
	}

	syncState := status.SyncState()
	if syncState == nil {
		return errors.New("missing beacon node status sync state")
	}

	if syncState.SyncDistance > 3 {
		return errors.New("beacon node is not synced")
	}

	wallclock := b.Metadata().Wallclock()
	if wallclock == nil {
		return errors.New("missing wallclock")
	}

	currentSlot := wallclock.Slots().Current()

	if currentSlot.Number()-uint64(syncState.HeadSlot) > 32 {
		return fmt.Errorf("beacon node is too far behind head, head slot is %d, current slot is %d", syncState.HeadSlot, currentSlot.Number())
	}

	for _, service := range b.services {
		if err := service.Ready(ctx); err != nil {
			return errors.Wrapf(err, "service %s is not ready", service.Name())
		}
	}

	return nil
}

func (b *BeaconNode) ForkDigest() (phase0.ForkDigest, error) {
	genesis := b.Metadata().Genesis
	if genesis == nil {
		return phase0.ForkDigest{}, errors.New("missing genesis")
	}

	spec, err := b.beacon.Spec()
	if err != nil {
		return phase0.ForkDigest{}, errors.Wrap(err, "failed to get spec")
	}

	_, epoch, err := b.beacon.Wallclock().Now()
	if err != nil {
		return phase0.ForkDigest{}, errors.Wrap(err, "failed to get wallclock")
	}

	current, err := spec.ForkEpochs.CurrentFork(phase0.Epoch(epoch.Number()))
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

	forkDigest, err := ComputeForkDigest(genesis.GenesisValidatorsRoot, forkVersion)
	if err != nil {
		return phase0.ForkDigest{}, errors.Wrap(err, "failed to compute fork digest")
	}

	return forkDigest, nil
}
