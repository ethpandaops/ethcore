package crawler_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ethpandaops/beacon/pkg/beacon"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/enclaves"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/services"
	"github.com/kurtosis-tech/kurtosis/api/golang/engine/lib/kurtosis_context"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"
)

type BeaconNode struct {
	Node    beacon.Node
	Service *services.ServiceContext
}

type TestFoundation struct {
	BeaconNodes []*BeaconNode
	KurtosisCtx *kurtosis_context.KurtosisContext
	EnclaveCtx  *enclaves.EnclaveContext
}

func createTestFoundation(t *testing.T, kurtosisCtx *kurtosis_context.KurtosisContext, enclaveCtx *enclaves.EnclaveContext) *TestFoundation {
	// Get all the CL services
	services, err := enclaveCtx.GetServices()
	require.NoError(t, err)

	clServiceNames := []string{}

	for name := range services {
		if strings.HasPrefix(string(name), "cl-") {
			clServiceNames = append(clServiceNames, string(name))
		}
	}

	t.Logf("Found %d CL services:", len(clServiceNames))

	for _, name := range clServiceNames {
		t.Logf("%s", name)
	}

	if len(clServiceNames) == 0 {
		t.Skip("No CL services found")
	}

	// Pick a random CL service that we will use to back our crawler with
	clServiceName := clServiceNames[rand.Intn(len(clServiceNames))]
	// Grab the CL service's Beacon API URL
	clService, err := enclaveCtx.GetServiceContext(clServiceName)
	require.NoError(t, err)

	beaconPort := clService.GetPublicPorts()["http"]
	require.NotNil(t, beaconPort)

	// Create all our beacon api instances
	beaconNodes := []*BeaconNode{}

	for _, clServiceName := range clServiceNames {
		b, err := getBeaconAPIInstance(enclaveCtx, clServiceName)
		require.NoError(t, err)

		beaconNodes = append(beaconNodes, b)
	}

	tf := &TestFoundation{
		BeaconNodes: beaconNodes,
		KurtosisCtx: kurtosisCtx,
		EnclaveCtx:  enclaveCtx,
	}

	for _, bn := range tf.BeaconNodes {
		go func(bn *BeaconNode) {
			require.NoError(t, bn.Node.Start(context.Background()))
		}(bn)
	}

	// Wait for all the beacon nodes to be healthy
	for !tf.AllBeaconNodesHealthy() {
		time.Sleep(1 * time.Second)
	}

	return tf
}

func GetCLServices(enclaveCtx *enclaves.EnclaveContext) ([]string, error) {
	services, err := enclaveCtx.GetServices()
	if err != nil {
		return nil, err
	}

	clServiceNames := []string{}

	for name := range services {
		if strings.HasPrefix(string(name), "cl-") {
			clServiceNames = append(clServiceNames, string(name))
		}
	}

	return clServiceNames, nil
}

func getBeaconAPIInstance(enclaveCtx *enclaves.EnclaveContext, clServiceName string) (*BeaconNode, error) {
	clService, err := enclaveCtx.GetServiceContext(clServiceName)
	if err != nil {
		return nil, err
	}

	beaconPort := clService.GetPublicPorts()["http"]
	if beaconPort == nil {
		return nil, fmt.Errorf("beacon port not found")
	}

	opts := beacon.DefaultOptions()
	opts.HealthCheck.Interval.Duration = 250 * time.Millisecond

	b := beacon.NewNode(
		logrus.New(),
		&beacon.Config{
			Name: clServiceName,
			Addr: "http://" + net.JoinHostPort(clService.GetMaybePublicIPAddress(), fmt.Sprintf("%d", beaconPort.GetNumber())),
		},
		"testing",
		*opts,
	)

	return &BeaconNode{
		Node:    b,
		Service: clService,
	}, nil
}

func (t *TestFoundation) AllBeaconNodesHealthy() bool {
	for _, bn := range t.BeaconNodes {
		if !bn.Node.Healthy() {
			return false
		}
	}

	return true
}

func (t *TestFoundation) RandomBeaconNode() *BeaconNode {
	return t.BeaconNodes[rand.Intn(len(t.BeaconNodes))]
}

func (t *TestFoundation) GenesisHasHappened() (bool, error) {
	bn := t.RandomBeaconNode()

	wallclock := bn.Node.Wallclock()
	if wallclock == nil {
		return false, fmt.Errorf("wallclock not found")
	}

	slot, _, err := wallclock.Now()
	if err != nil {
		return false, err
	}

	return slot.Number() > 0, nil
}
