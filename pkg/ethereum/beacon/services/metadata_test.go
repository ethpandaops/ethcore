package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ethpandaops/beacon/pkg/beacon"
	"github.com/ethpandaops/beacon/pkg/beacon/api/types"
	"github.com/ethpandaops/beacon/pkg/beacon/state"
	"github.com/ethpandaops/ethcore/pkg/ethereum/networks"
	v1 "github.com/ethpandaops/go-eth2-client/api/v1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// stubBeacon embeds the (nil) beacon.Node interface so any method this test
// doesn't explicitly override panics if actually called. MetadataService
// only ever calls NodeVersion, Healthy, FetchNodeIdentity, Spec and
// Genesis, so those are the only ones that need real behavior here.
type stubBeacon struct {
	beacon.Node
	spec *state.Spec
}

func (s *stubBeacon) Healthy() bool                 { return true }
func (s *stubBeacon) NodeVersion() (string, error)  { return "test/1.0", nil }
func (s *stubBeacon) Spec() (*state.Spec, error)    { return s.spec, nil }
func (s *stubBeacon) Genesis() (*v1.Genesis, error) { return &v1.Genesis{}, nil }
func (s *stubBeacon) FetchNodeIdentity(context.Context) (*types.Identity, error) {
	return nil, errors.New("no identity in this test")
}

func testLogger() logrus.FieldLogger {
	l := logrus.New()
	l.SetLevel(logrus.ErrorLevel)

	return l
}

// unresolvableSpec has an empty CONFIG_NAME and a deposit contract that
// matches no KnownNetworks entry -- the exact condition that makes
// networks.DeriveFromSpec return ErrNetworkNotFound.
func unresolvableSpec() *state.Spec {
	return &state.Spec{
		ConfigName:             "",
		DepositContractAddress: "0x000000000000000000000000000000000000ff",
		DepositChainID:         999999999,
		SecondsPerSlot:         state.StringerDuration(12 * time.Second),
		SlotsPerEpoch:          32,
	}
}

// TestDeriveNetwork_UnresolvableSpecReturnsError is a direct unit check
// that DeriveNetwork surfaces ErrNetworkNotFound as a plain error rather
// than doing anything fatal itself.
func TestDeriveNetwork_UnresolvableSpecReturnsError(t *testing.T) {
	m := NewMetadataService(testLogger(), &stubBeacon{spec: unresolvableSpec()}, "")
	m.Spec = unresolvableSpec()

	err := m.DeriveNetwork(context.Background())
	require.Error(t, err)
	require.ErrorIs(t, err, networks.ErrNetworkNotFound)
}

// TestReady_ReportsNotReadyWhenNetworkNotDerived guards the other half of
// ETHEREUM-02's fix: once DeriveNetwork fails and Network is left at its
// NetworkNameNone default, Ready() must say so instead of silently
// reporting ready with a meaningless network.
func TestReady_ReportsNotReadyWhenNetworkNotDerived(t *testing.T) {
	m := NewMetadataService(testLogger(), &stubBeacon{spec: unresolvableSpec()}, "")
	m.Genesis = &v1.Genesis{}
	m.Spec = unresolvableSpec()
	// Network is left at its NewMetadataService default (NetworkNameNone) --
	// deliberately not set, mirroring a failed DeriveNetwork call.

	err := m.Ready(context.Background())
	require.Error(t, err)
}

// TestStart_UnresolvableSpecDoesNotCrash guards against ETHEREUM-02: a
// beacon node reporting a spec that can't be resolved to a known network
// used to call logrus.Fatal, which calls os.Exit(1) and kills the entire
// embedding process. Start() must complete without crashing the test
// binary, and the service must end up reporting not-ready rather than
// silently claiming readiness with an undetermined network.
func TestStart_UnresolvableSpecDoesNotCrash(t *testing.T) {
	m := NewMetadataService(testLogger(), &stubBeacon{spec: unresolvableSpec()}, "")

	ready := make(chan struct{})
	m.OnReady(context.Background(), func(ctx context.Context) error {
		close(ready)

		return nil
	})

	require.NoError(t, m.Start(context.Background()))
	t.Cleanup(func() { _ = m.Stop(context.Background()) })

	// If this test process is still running to observe this, ETHEREUM-02
	// did not crash it -- the old code would have called os.Exit(1) from
	// the goroutine Start() spawns, well before this select could fire.
	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("onReady callback never fired")
	}

	require.Equal(t, networks.NetworkNameNone, m.GetNetwork().Name, "network must stay at its default when it could not be derived")
	require.Error(t, m.Ready(context.Background()), "service must report not-ready when the network could not be derived")
}
