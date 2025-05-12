package crawler_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/ethpandaops/beacon/pkg/beacon/api/types"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/crawler"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/host"
	"github.com/ethpandaops/ethcore/pkg/discovery"
	"github.com/ethpandaops/ethcore/pkg/ethereum"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/services"
	"github.com/kurtosis-tech/kurtosis/api/golang/engine/lib/kurtosis_context"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func Test_RunKurtosisTests(t *testing.T) {
	// if os.Getenv("EXTENDED_TESTS") != "true" {
	// 	t.Skip("Skipping kurtosis tests")
	// }

	// if os.Getenv("KURTOSIS_ENCLAVE") == "" {
	// 	require.Fail(t, "KURTOSIS_ENCLAVE env var must be set")
	// }
	enclaveName := os.Getenv("KURTOSIS_ENCLAVE")
	enclaveName = "dogwater"
	require.NotEmpty(t, enclaveName, "KURTOSIS_ENCLAVE env var must be set")

	ctx := context.Background()
	kurtosisCtx, err := kurtosis_context.NewKurtosisContextFromLocalEngine()
	require.NoError(t, err)

	enclaveCtx, err := kurtosisCtx.GetEnclaveContext(ctx, enclaveName)
	require.NoError(t, err)

	testFoundation := createTestFoundation(t, kurtosisCtx, enclaveCtx)

	// Run all the kurtosis tests in parallel
	t.Run("ListParticipants", func(t *testing.T) {
		ListParticipants(t, testFoundation)
	})
	t.Run("AllDiscoverableNodes", func(t *testing.T) {
		AllDiscoverableNodes(t, testFoundation)
	})
}

func ListParticipants(t *testing.T, tf *TestFoundation) {
	services, err := tf.EnclaveCtx.GetServices()
	require.NoError(t, err)

	t.Logf("Found %d services in enclave %s:", len(services), tf.EnclaveCtx.GetEnclaveName())

	for name := range services {
		t.Logf("%s", name)
	}
}

func AllDiscoverableNodes(t *testing.T, tf *TestFoundation) {
	log := logrus.New()
	log.SetLevel(logrus.TraceLevel)
	// Wait until genesis has happened
	for {
		genesis, err := tf.GenesisHasHappened()
		require.NoError(t, err)

		if genesis {
			break
		}

		time.Sleep(1 * time.Second)
	}

	// Get all our peer IDs
	identities := make(map[services.ServiceName]*types.Identity)
	successful := make(map[services.ServiceName]bool)

	for _, bn := range tf.BeaconNodes {
		identity, err := bn.Node.FetchNodeIdentity(context.Background())
		require.NoError(t, err)

		identities[bn.Service.GetServiceName()] = identity
		successful[bn.Service.GetServiceName()] = false

		log.Infof("Identified peer ID for %s: %s", bn.Service.GetServiceName(), identity.PeerID)
	}

	// Create our discovery instance which we'll use to manually add peers
	manual := &discovery.Manual{}

	// Get the first beacon node
	clServiceNames, err := GetCLServices(tf.EnclaveCtx)
	require.NoError(t, err)
	require.NotEmpty(t, clServiceNames)

	clServiceName := clServiceNames[0]
	logrus.Infof("Using CL service: %s", clServiceName)

	clService, err := tf.EnclaveCtx.GetServiceContext(clServiceName)
	require.NoError(t, err)

	beaconPort := clService.GetPublicPorts()["http"]
	require.NotNil(t, beaconPort)

	cr := crawler.New(log, &crawler.Config{
		DialConcurrency: 10,
		CooloffDuration: 1 * time.Second,
		Node: &host.Config{
			IPAddr: net.ParseIP("127.0.0.1"),
		},
		Beacon: &ethereum.Config{
			BeaconNodeAddress: "http://" + net.JoinHostPort(clService.GetMaybePublicIPAddress(), fmt.Sprintf("%d", beaconPort.GetNumber())),
			Network:           "kurtosis",
		},
	}, "mimicry/crawler", "mimicry", manual)

	go func() {
		require.NoError(t, cr.Start(context.Background()))
	}()

	// Create a sink of the crawler's events
	cr.OnSuccessfulCrawl(func(peerID peer.ID, status *common.Status, metadata *common.MetaData) {
		log.Infof("###### Got status/metadata: %s", peerID)

		// Get the service name
		found := false

		for name, identity := range identities {
			if identity.PeerID == peerID.String() {
				log.Infof("Got a successful crawl for %s", name)

				successful[name] = true

				found = true

				break
			}
		}

		if !found {
			log.Fatalf("Failed to find peer ID in our list of identities. If this has failed, it's likely due to a bug in the crawler. Peer ID: %s", peerID)
		}
	})

	cr.OnFailedCrawl(func(peerID peer.ID, err crawler.CrawlError) {
		log.Errorf("Failed to crawl peer %s: %v", peerID, err)
	})

	// Wait until the crawler is ready
	<-cr.OnReady

	// Start feeding in the ENR's
	go func() {
		// Get all our enrs
		for _, bn := range tf.BeaconNodes {
			identity, err := bn.Node.FetchNodeIdentity(context.Background())
			require.NoError(t, err)

			if complete, ok := successful[bn.Service.GetServiceName()]; ok && complete {
				log.Infof("Already have status/metadata for participant: %s", bn.Service.GetServiceName())

				continue
			}

			log.Infof("Adding node %s's ENR to discovery pool (%s)", bn.Service.GetServiceName(), identity.ENR)

			en, err := discovery.ENRToEnode(identity.ENR)
			require.NoError(t, err)

			if err := manual.AddNode(context.Background(), en); err != nil {
				log.Errorf("Failed to add node: %v", err)
			}
		}

		time.Sleep(30 * time.Second)
	}()

	// Wait until we've discovered all the nodes
	for {
		okPeers := 0

		for _, complete := range successful {
			if complete {
				okPeers++
			}
		}

		log.Infof("Discovered %d/%d peers", okPeers, len(successful))

		if okPeers == len(successful) {
			// Test complete!
			break
		} else {
			missingPeers := []services.ServiceName{}

			for name, complete := range successful {
				if !complete {
					missingPeers = append(missingPeers, name)
				}
			}

			log.Infof("Missing peers: %v", missingPeers)
		}

		time.Sleep(1 * time.Second)
	}
}
