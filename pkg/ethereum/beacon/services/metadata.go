package services

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"time"

	v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/cenkalti/backoff/v5"
	"github.com/ethpandaops/beacon/pkg/beacon"
	"github.com/ethpandaops/beacon/pkg/beacon/api/types"
	"github.com/ethpandaops/beacon/pkg/beacon/state"
	"github.com/ethpandaops/ethcore/pkg/ethereum/clients"
	"github.com/ethpandaops/ethcore/pkg/ethereum/networks"
	"github.com/ethpandaops/ethwallclock"
	"github.com/go-co-op/gocron/v2"
	"github.com/sirupsen/logrus"
)

type MetadataService struct {
	beacon           beacon.Node
	log              logrus.FieldLogger
	Network          *networks.Network
	Genesis          *v1.Genesis
	Spec             *state.Spec
	wallclock        *ethwallclock.EthereumBeaconChain
	onReadyCallbacks []func(context.Context) error
	nodeIdentity     *types.Identity
	nodeID           string
	nodeIDHash       string
	networkOverride  string
	mu               sync.Mutex
}

func NewMetadataService(log logrus.FieldLogger, sbeacon beacon.Node, networkOverride string) MetadataService {
	return MetadataService{
		beacon:           sbeacon,
		log:              log.WithField("module", "ethcore/ethereum/beacon/metadata"),
		Network:          &networks.Network{Name: networks.NetworkNameHoodi},
		onReadyCallbacks: []func(context.Context) error{},
		networkOverride:  networkOverride,
	}
}

func (m *MetadataService) GetNodeVersion(_ context.Context) string {
	version, _ := m.beacon.NodeVersion()

	return version
}

func (m *MetadataService) GetNodeIdentity() (*types.Identity, error) {
	if m.nodeIdentity == nil {
		return nil, errors.New("node identity is not available")
	}

	return m.nodeIdentity, nil
}

func (m *MetadataService) GetNodeID() (string, error) {
	if m.nodeID == "" {
		return "", errors.New("node ID is not available")
	}

	return m.nodeID, nil
}

func (m *MetadataService) GetNodeIDHash() (string, error) {
	if m.nodeIDHash == "" {
		return "", errors.New("node ID hash is not available")
	}

	return m.nodeIDHash, nil
}

func (m *MetadataService) GetGenesis() *v1.Genesis {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.Genesis
}

func (m *MetadataService) GetSpec() *state.Spec {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.Spec
}

func (m *MetadataService) GetNetwork() *networks.Network {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Network == nil {
		return nil
	}

	// Return a copy to prevent external modification
	network := *m.Network

	return &network
}

func (m *MetadataService) GetClient(ctx context.Context) string {
	return string(clients.ClientFromString(m.GetNodeVersion(ctx)))
}

func (m *MetadataService) GetWallclock() *ethwallclock.EthereumBeaconChain {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.wallclock
}

func (m *MetadataService) Name() Name {
	return "metadata"
}

func (m *MetadataService) Start(ctx context.Context) error {
	go func() {
		operation := func() (string, error) {
			if err := m.RefreshAll(ctx); err != nil {
				return "", err
			}

			return "", nil
		}

		retryOpts := []backoff.RetryOption{
			backoff.WithBackOff(backoff.NewExponentialBackOff()),
			backoff.WithNotify(func(err error, duration time.Duration) {
				m.log.WithError(err).Warnf("Failed to refresh metadata, retrying in %s", duration)
			}),
		}

		if _, err := backoff.Retry(ctx, operation, retryOpts...); err != nil {
			m.log.WithError(err).Warn("Failed to refresh metadata")
		}

		if err := m.DeriveNetwork(ctx); err != nil {
			// Fatally panic if we can't derive the network
			m.log.WithFields(logrus.Fields{
				"error": err,
			}).Fatal("Failed to derive network")
		}

		if err := m.Ready(ctx); err != nil {
			m.log.WithError(err).Warn("Failed to check metadata service readiness")
		}

		m.log.Debug("Metadata service is ready")

		for _, cb := range m.onReadyCallbacks {
			if err := cb(ctx); err != nil {
				m.log.WithError(err).Warn("Failed to execute onReady callback")
			}
		}
	}()

	s, err := gocron.NewScheduler(gocron.WithLocation(time.Local))
	if err != nil {
		return err
	}

	if _, err := s.NewJob(
		gocron.DurationJob(5*time.Minute),
		gocron.NewTask(
			func(ctx context.Context) {
				_ = m.RefreshAll(ctx)
			},
			ctx,
		),
	); err != nil {
		return err
	}

	s.Start()

	return nil
}

func (m *MetadataService) Stop(_ context.Context) error {
	return nil
}

func (m *MetadataService) OnReady(_ context.Context, cb func(context.Context) error) {
	m.onReadyCallbacks = append(m.onReadyCallbacks, cb)
}

func (m *MetadataService) Ready(ctx context.Context) error {
	if m.Genesis == nil {
		return errors.New("genesis is not available")
	}

	if m.Spec == nil {
		return errors.New("spec is not available")
	}

	if m.GetNodeVersion(ctx) == "" {
		return errors.New("node version is not available")
	}

	if m.wallclock == nil {
		return errors.New("wallclock is not available")
	}

	if m.nodeIdentity == nil {
		return errors.New("node identity is not available")
	}

	return nil
}

func (m *MetadataService) RefreshAll(ctx context.Context) error {
	if err := m.fetchSpec(ctx); err != nil {
		m.log.WithError(err).Warn("Failed to fetch spec for refresh")
	}

	if err := m.fetchGenesis(ctx); err != nil {
		m.log.WithError(err).Warn("Failed to fetch genesis for refresh")
	}

	if _, err := m.DeriveNodeIdentity(ctx); err != nil {
		m.log.WithError(err).Warn("Failed to derive node identity")
	}

	if m.Genesis != nil && m.Spec != nil && m.wallclock == nil {
		if newWallclock := ethwallclock.NewEthereumBeaconChain(m.Genesis.GenesisTime, m.Spec.SecondsPerSlot.AsDuration(), uint64(m.Spec.SlotsPerEpoch)); newWallclock != nil {
			m.mu.Lock()

			m.wallclock = newWallclock

			m.mu.Unlock()
		}
	}

	return nil
}

func (m *MetadataService) DeriveNodeIdentity(ctx context.Context) (*types.Identity, error) {
	if m.beacon == nil {
		return nil, errors.New("beacon is not available")
	}

	if !m.beacon.Healthy() {
		return nil, errors.New("beacon is not healthy")
	}

	identity, err := m.beacon.FetchNodeIdentity(ctx)
	if err != nil {
		return nil, err
	}

	m.nodeIdentity = identity

	enr, err := identity.GetEnode()
	if err != nil {
		return nil, err
	}

	m.nodeID = enr.ID().String()

	// Hash the node ID so we obfuscate the actual node ID, and trim it to 10 characters
	m.nodeIDHash = fmt.Sprintf("%x", sha256.Sum256([]byte(m.nodeID)))[:10]

	return identity, nil
}

func (m *MetadataService) DeriveNetwork(_ context.Context) error {
	if m.Spec == nil {
		return errors.New("spec is not available")
	}

	m.log.WithFields(logrus.Fields{
		"deposit_contract_address": m.Spec.DepositContractAddress,
		"deposit_chain_id":         m.Spec.DepositChainID,
		"config_name":              m.Spec.ConfigName,
	}).Info("Deriving ethereum network")

	network, err := networks.DeriveFromSpec(m.Spec)
	if err != nil {
		return err
	}

	// Apply network override if specified and the derived network is "testnet".
	// This allows us to override the network name for testnets that are not
	// in our list of known networks.
	if m.networkOverride != "" && (network.Name == "testnet") {
		m.log.WithFields(logrus.Fields{
			"derived_network":          network.Name,
			"override_network":         m.networkOverride,
			"deposit_contract_address": m.Spec.DepositContractAddress,
			"deposit_chain_id":         m.Spec.DepositChainID,
		}).Info("Applying testnet network name override")

		// Create a new network with the override name but keep other properties.
		network = &networks.Network{
			Name:                   networks.NetworkName(m.networkOverride),
			ID:                     network.ID,
			DepositContractAddress: network.DepositContractAddress,
			DepositChainID:         network.DepositChainID,
		}
	}

	m.log.WithFields(logrus.Fields{
		"name":                     network.Name,
		"id":                       network.ID,
		"deposit_contract_address": m.Spec.DepositContractAddress,
		"deposit_chain_id":         m.Spec.DepositChainID,
		"config_name":              m.Spec.ConfigName,
	}).Debug("Detected ethereum network")

	m.Network = network

	return nil
}

func (m *MetadataService) fetchSpec(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	spec, err := m.beacon.FetchSpec(ctx)
	if err != nil {
		return err
	}

	m.Spec = spec

	return nil
}

func (m *MetadataService) fetchGenesis(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	genesis, err := m.beacon.FetchGenesis(ctx)
	if err != nil {
		return err
	}

	m.Genesis = genesis

	return nil
}
