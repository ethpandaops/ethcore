package crawler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/attestantio/go-eth2-client/api"
	"github.com/cenkalti/backoff"
	"github.com/chuckpreslar/emission"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethpandaops/beacon/pkg/beacon"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/cache"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/host"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth"
	"github.com/ethpandaops/ethcore/pkg/discovery"
	"github.com/ethpandaops/ethcore/pkg/ethereum"
	"github.com/go-co-op/gocron"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/tree"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/p2p/encoder"
	pb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
	"github.com/sirupsen/logrus"
)

var sszNetworkEncoder = encoder.SszNetworkEncoder{}

// Crawler is a Mimicry client that connects to Ethereum beacon nodes and
// requests status updates from them. It depends on an upstream beacon node
// to provide it with the current Ethereum network state, and never
// stays connected to any one peer for very long.
// Crawler comes with all tools included to crawl the network,
// including discovery, and outputs events to the ch
type Crawler struct {
	log       logrus.FieldLogger
	config    *Config
	userAgent string

	broker *emission.Emitter
	node   *host.Node

	ctx      context.Context
	metadata *common.MetaData
	status   *common.Status

	// Subcomponents
	beacon    *ethereum.BeaconNode
	reqResp   *p2p.ReqResp
	discovery discovery.NodeFinder

	statusMu sync.Mutex

	statusFromPeerChan chan eth.PeerStatus

	duplicateCache *cache.DuplicateCache

	metrics *Metrics

	peersToDial chan *discovery.ConnectablePeer
}

// New creates a new Crawler
func New(log logrus.FieldLogger, config *Config, userAgent, namespace string, f discovery.NodeFinder) *Crawler {
	return &Crawler{
		log:       log,
		userAgent: userAgent,
		config:    config,
		statusMu:  sync.Mutex{},
		broker:    emission.NewEmitter(),
		status:    &common.Status{},
		metadata: &common.MetaData{
			SeqNumber: 0,
			Attnets:   common.AttnetBits{},
			Syncnets:  common.SyncnetBits{},
		},
		metrics:            NewMetrics(),
		statusFromPeerChan: make(chan eth.PeerStatus, 10000),
		duplicateCache:     cache.NewDuplicateCache(),
		discovery:          f,
		peersToDial:        make(chan *discovery.ConnectablePeer, 10000),
	}
}

// Start starts the Crawler
func (c *Crawler) Start(ctx context.Context) error {
	c.log.WithFields(logrus.Fields{
		"user_agent": c.userAgent,
	}).Info("Starting Ethereum Mimicry crawler")

	// Start the duplicate cache
	if err := c.duplicateCache.Start(ctx); err != nil {
		return fmt.Errorf("failed to start duplicate cache: %w", err)
	}

	// Create the host
	h, err := host.NewNode(ctx, c.log, c.config.Node, c.userAgent)
	if err != nil {
		return fmt.Errorf("failed to create host: %w", err)
	}

	c.node = h

	// Create the beacon node
	opts := beacon.DefaultOptions()

	opts.HealthCheck.Interval.Duration = time.Second * 3
	opts.HealthCheck.SuccessfulResponses = 1
	opts.PrometheusMetrics = false
	opts.BeaconSubscription.Enabled = true
	opts.BeaconSubscription.Topics = []string{"head"}

	b, err := ethereum.NewBeaconNode(ctx, "beacon_node", c.config.Beacon, c.log, "crawler", *opts)
	if err != nil {
		return fmt.Errorf("failed to create upstream beacon node: %w", err)
	}

	c.beacon = b

	// Once the beacon node is ready:
	// - Fetch the initial status
	// - Start the libp2p node
	// - Register the req/resp handlers
	// - Wire up the components
	// - Start node discovery
	// - Start our node dialer
	// - Start the crons
	c.beacon.OnReady(ctx, func(ctx context.Context) error {
		c.log.Info("Beacon node is ready")

		bo := backoff.NewExponentialBackOff()
		bo.MaxInterval = 15 * time.Second
		bo.MaxElapsedTime = 3 * time.Minute

		if err := backoff.RetryNotify(func() error {
			return c.fetchAndSetStatus(ctx)
		}, bo, func(err error, timer time.Duration) {
			c.log.WithError(err).WithField("next_attempt", timer).Warn("Failed to fetch initial status")
		}); err != nil {
			c.log.WithError(err).Fatal("Failed to fetch initial status")
		}

		c.log.Info("Successfully fetched initial status")

		// Start the libp2p host
		libp2pHost, err := c.node.Start(ctx)
		if err != nil {
			c.log.WithError(err).Fatal("Failed to start libp2p node")
		}

		c.log.Info("Successfully started libp2p node")

		// Register our req/resp handler
		c.reqResp = p2p.NewReqResp(c.log, libp2pHost, sszNetworkEncoder, &p2p.ReqRespConfig{
			WriteTimeout:    time.Second * 5,
			ReadTimeout:     time.Second * 15,
			TimeToFirstByte: time.Second * 5,
		})

		c.log.Info("Successfully created req/resp handler")

		if err := c.wireUpComponents(ctx); err != nil {
			c.log.WithError(err).Fatal("Failed to wire up components")
		}

		c.log.Info("Successfully wired up components")

		// Start the node dialer
		if err := c.startDialer(ctx); err != nil {
			c.log.WithError(err).Fatal("Failed to start node dialer")
		}

		c.log.Info("Successfully started node dialer")

		// We now have a valid status and can start discovering peers.
		if err := c.discovery.Start(ctx); err != nil {
			c.log.WithError(err).Fatal("Failed to start discovery")
		}

		c.log.Info("Successfully started discovery")

		if err := c.startCrons(ctx); err != nil {
			c.log.WithError(err).Fatal("Failed to start crons")
		}

		c.log.Info("Successfully started crons")

		c.log.Info("Successfully started Mimicry crawler")

		return nil
	})

	// Start the beacon node
	if err := c.beacon.Start(ctx); err != nil {
		return fmt.Errorf("failed to start beacon node: %w", err)
	}

	return nil
}

func (c *Crawler) Stop(ctx context.Context) error {
	c.duplicateCache.Stop()

	// Tell all our peers we're disconnecting
	for _, p := range c.node.Peerstore().Peers() {
		if err := c.DisconnectFromPeer(ctx, p, eth.GoodbyeReasonClientShutdown); err != nil {
			c.log.WithError(err).Error("Failed to disconnect from peer on shutdown")
		}
	}

	if err := c.node.Stop(ctx); err != nil {
		return fmt.Errorf("failed to close host: %w", err)
	}

	return nil
}

func (c *Crawler) startCrons(ctx context.Context) error {
	cr := gocron.NewScheduler(time.Local)

	if _, err := cr.Every("60s").Do(func() {
		if err := c.fetchAndSetStatus(ctx); err != nil {
			c.log.WithError(err).Error("Failed to fetch and set status")
		}
	}); err != nil {
		return err
	}

	cr.StartAsync()

	return nil
}

func (c *Crawler) GetNode() *host.Node {
	return c.node
}

func (c *Crawler) GetBeacon() *ethereum.BeaconNode {
	return c.beacon
}

func (c *Crawler) GetReqResp() *p2p.ReqResp {
	return c.reqResp
}

func (c *Crawler) SetStatus(status *common.Status) {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()

	c.status = status
}

func (c *Crawler) GetStatus() common.Status {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()

	return *c.status
}

func (c *Crawler) StatusFromPeerChan() <-chan eth.PeerStatus {
	return c.statusFromPeerChan
}

func (c *Crawler) nodeIsOnOurNetwork(node *enode.Node) bool {
	sszEncodedForkEntry := make([]byte, 16)

	entry := enr.WithEntry("eth2", &sszEncodedForkEntry)

	if err := node.Record().Load(entry); err != nil {
		return false
	}

	forkEntry := &pb.ENRForkID{}
	if err := forkEntry.UnmarshalSSZ(sszEncodedForkEntry); err != nil {
		return false
	}

	status := c.GetStatus()

	return bytes.Equal(forkEntry.CurrentForkDigest, status.ForkDigest[:])
}

func (c *Crawler) ConnectToPeer(ctx context.Context, p peer.AddrInfo, n *enode.Node) error {
	if !c.nodeIsOnOurNetwork(n) {
		c.emitFailedCrawl(p.ID, ErrCrawlENRForkDigest)

		return nil
	}

	if c.duplicateCache.Nodes.Get(p.ID.String()) != nil {
		c.log.WithFields(logrus.Fields{
			"peer": p.ID,
		}).Debug("We've already connected to this peer previously")

		c.emitFailedCrawl(p.ID, ErrCrawlTooSoon)

		return nil
	}

	c.log.WithFields(logrus.Fields{
		"peer": p.ID,
	}).Debug("Connecting to peer")

	// Check if we're already connected to the peer
	if status := c.node.Connectedness(p.ID); status == network.Connected {
		return errors.New("already connected to peer")
	}

	// Connect to the peer
	timeoutCtx, cancel := context.WithTimeout(ctx, c.config.DialTimeout)
	defer cancel()

	if err := c.node.ConnectToPeer(timeoutCtx, p); err != nil {
		return fmt.Errorf("failed to connect to peer: %w", err)
	}

	// Add the supported protocols to the peer
	if err := c.node.AddProtocols(p.ID, c.reqResp.SupportedProtocols()...); err != nil {
		return fmt.Errorf("failed to add protocols to peer: %w", err)
	}

	return nil
}

func (c *Crawler) DisconnectFromPeer(ctx context.Context, peerID peer.ID, reason eth.GoodbyeReason) error {
	// If they're already disconnected, do nothing
	if c.node.Connectedness(peerID) != network.Connected {
		return nil
	}

	logCtx := c.log.WithFields(logrus.Fields{
		"peer": peerID,
	})

	logCtx.Debug("Disconnecting from peer")

	// Send a goodbye message with a short timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	// Send a goodbye message but don't wait for response
	resp := common.Goodbye(0)
	goodbye := common.Goodbye(reason)

	// We don't care about the goodbye message response
	_ = c.reqResp.SendRequest(timeoutCtx, &p2p.Request{
		ProtocolID: eth.GoodbyeProtocolID,
		PeerID:     peerID,
		Payload:    &goodbye,
		Timeout:    time.Second * 5,
	}, &resp)

	// Always disconnect regardless of goodbye message status
	return c.node.DisconnectFromPeer(ctx, peerID)
}

func (c *Crawler) RequestStatusFromPeer(ctx context.Context, peerID peer.ID) (*common.Status, error) {
	status := c.GetStatus()

	req := &p2p.Request{
		ProtocolID: eth.StatusProtocolID,
		PeerID:     peerID,
		Payload:    &status,
		Timeout:    time.Second * 30,
	}

	rsp := &common.Status{}

	if err := c.reqResp.SendRequest(ctx, req, rsp); err != nil {
		errType := &p2p.RequestError{}

		if errors.As(err, &errType) {
			c.metrics.RecordFailedRequest(string(req.ProtocolID), errType.Type)

			return nil, fmt.Errorf("failed to send request: %w", err)
		}

		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if status.ForkDigest != rsp.ForkDigest {
		c.emitPeerStatusUpdated(peerID, rsp)
	}

	return rsp, nil
}

func (c *Crawler) RequestMetadataFromPeer(ctx context.Context, peerID peer.ID) (*common.MetaData, error) {
	metadata := c.metadata

	req := &p2p.Request{
		ProtocolID: eth.MetaDataProtocolID,
		PeerID:     peerID,
		Payload:    metadata,
		Timeout:    time.Second * 30,
	}

	rsp := &common.MetaData{}

	if err := c.reqResp.SendRequest(ctx, req, rsp); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	c.emitMetadataReceived(peerID, rsp)

	return rsp, nil
}

func (c *Crawler) GetPeerAgentVersion(peerID peer.ID) string {
	rawAgentVersion, err := c.node.Peerstore().Get(peerID, "AgentVersion")
	if err != nil {
		return "unknown"
	}

	return rawAgentVersion.(string)
}

func (c *Crawler) fetchAndSetStatus(ctx context.Context) error {
	status := &common.Status{}

	// Fetch the status from the upstream beacon node.
	checkpoint, err := c.beacon.Node().FetchFinality(ctx, "head")
	if err != nil {
		return fmt.Errorf("failed to fetch finality: %w", err)
	}

	status.FinalizedRoot = tree.Root(checkpoint.Finalized.Root)
	status.FinalizedEpoch = common.Epoch(checkpoint.Finalized.Epoch)

	headers, err := c.beacon.Node().FetchBeaconBlockHeader(ctx, &api.BeaconBlockHeaderOpts{
		Block: "head",
	})
	if err != nil {
		return fmt.Errorf("failed to fetch block headers: %w", err)
	}

	status.HeadRoot = tree.Root(headers.Header.Message.StateRoot)
	status.HeadSlot = common.Slot(headers.Header.Message.Slot)

	forkDigest, err := c.beacon.ForkDigest()
	if err != nil {
		return fmt.Errorf("failed to fetch fork digest: %w", err)
	}

	status.ForkDigest = common.ForkDigest(forkDigest)

	before := c.GetStatus()

	if before.FinalizedEpoch != status.FinalizedEpoch ||
		before.FinalizedRoot != status.FinalizedRoot ||
		before.HeadSlot != status.HeadSlot ||
		before.HeadRoot != status.HeadRoot ||
		before.ForkDigest != status.ForkDigest {
		c.log.WithFields(logrus.Fields{
			"finalized_epoch": status.FinalizedEpoch,
			"finalized_root":  status.FinalizedRoot,
			"head_slot":       status.HeadSlot,
			"head_root":       status.HeadRoot,
			"fork_digest":     status.ForkDigest,
		}).Info("New eth2 status set")
	}

	// Set our status
	c.SetStatus(status)

	return nil
}
