package crawler

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
	pb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/attestantio/go-eth2-client/api"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/cenkalti/backoff/v5"
	"github.com/chuckpreslar/emission"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethpandaops/beacon/pkg/beacon"
	"github.com/ethpandaops/ethcore/pkg/cache"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/host"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth"
	"github.com/ethpandaops/ethcore/pkg/discovery"
	"github.com/ethpandaops/ethcore/pkg/ethereum"
	"github.com/go-co-op/gocron/v2"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/tree"
	"github.com/sirupsen/logrus"
)

var sszNetworkEncoder = encoder.SszNetworkEncoder{}

const unknown = "unknown"

// RetryInfo contains information about a peer to be retried.
type RetryInfo struct {
	Peer         *discovery.ConnectablePeer
	ENR          *enode.Node
	Attempts     int
	LastAttempt  time.Time
	BackoffDelay time.Duration
}

// Crawler is a Mimicry client that connects to Ethereum beacon nodes and
// requests status updates from them. It depends on an upstream beacon node
// to provide it with the current Ethereum network state, and never
// stays connected to any one peer for very long.
// Crawler comes with all tools included to crawl the network,
// including discovery, and outputs events to the ch.
type Crawler struct {
	log       logrus.FieldLogger
	config    *Config
	userAgent string
	broker    *emission.Emitter
	node      *host.Node
	metadata  *common.MetaData
	status    *common.Status

	// Subcomponents
	beacon             *ethereum.BeaconNode
	reqResp            *p2p.ReqResp
	discovery          discovery.NodeFinder
	statusMu           sync.Mutex
	statusFromPeerChan chan eth.PeerStatus
	duplicateCache     cache.DuplicateCache[string, time.Time]
	metrics            *Metrics
	peersToDial        chan *discovery.ConnectablePeer
	OnReady            chan struct{}

	// ENR storage
	peerENRs   map[peer.ID]*enode.Node
	peerENRsMu sync.RWMutex

	// Retry tracking
	retryQueue   chan *RetryInfo
	retryTracker map[peer.ID]*RetryInfo
	retryMu      sync.RWMutex

	// Scheduler for cron jobs
	scheduler gocron.Scheduler

	// Shutdown tracking
	shutdownOnce sync.Once
	isShutdown   bool
	shutdownMu   sync.RWMutex

	// Context for cancellation
	//nolint:containedctx // Would require big refactor into channels, leaving for now.
	ctx    context.Context
	cancel context.CancelFunc

	// Wait group for dialer workers
	dialerWg sync.WaitGroup
}

// New creates a new Crawler.
func New(log logrus.FieldLogger, config *Config, f discovery.NodeFinder) *Crawler {
	return &Crawler{
		log:       log.WithField("module", "ethcore/consensus/crawler"),
		userAgent: config.UserAgent,
		config:    config,
		statusMu:  sync.Mutex{},
		broker:    emission.NewEmitter(),
		status:    &common.Status{},
		metadata: &common.MetaData{
			SeqNumber: 1,
			Attnets:   common.AttnetBits{},
			Syncnets:  common.SyncnetBits{},
		},
		metrics:            NewMetrics(config.Namespace),
		statusFromPeerChan: make(chan eth.PeerStatus, 10000),
		duplicateCache:     cache.NewDuplicateCache[string, time.Time](log, config.CooloffDuration),
		discovery:          f,
		peersToDial:        make(chan *discovery.ConnectablePeer, 10000),
		OnReady:            make(chan struct{}),
		peerENRs:           make(map[peer.ID]*enode.Node, 1000),
		peerENRsMu:         sync.RWMutex{},
		retryQueue:         make(chan *RetryInfo, 1000),
		retryTracker:       make(map[peer.ID]*RetryInfo, 1000),
		retryMu:            sync.RWMutex{},
	}
}

// Start starts the Crawler.
func (c *Crawler) Start(ctx context.Context) error {
	c.log.WithFields(logrus.Fields{
		"user_agent": c.userAgent,
		"cooloff":    c.config.CooloffDuration,
	}).Info("Starting crawler")

	// Create internal context for cancellation
	c.ctx, c.cancel = context.WithCancel(ctx)

	// Start the duplicate cache
	if err := c.duplicateCache.Start(c.ctx); err != nil {
		return fmt.Errorf("failed to start duplicate cache: %w", err)
	}

	// Create the host
	h, err := host.NewNode(c.ctx, c.log, c.config.Node, c.config.Namespace, c.userAgent)
	if err != nil {
		return fmt.Errorf("failed to create host: %w", err)
	}

	c.node = h

	// Create the beacon node
	opts := &ethereum.Options{Options: beacon.DefaultOptions()}
	opts.DisablePrometheusMetrics()
	opts.HealthCheck.Interval.Duration = time.Second * 3
	opts.HealthCheck.SuccessfulResponses = 1
	opts.BeaconSubscription.Enabled = true
	opts.BeaconSubscription.Topics = []string{"head"}

	b, err := ethereum.NewBeaconNode(
		c.log,
		"ethcore/crawler",
		c.config.Beacon,
		opts,
	)
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
	c.beacon.OnReady(func(ctx context.Context) error {
		c.log.Info("Beacon node is ready")

		operation := func() (string, error) {
			if err := c.fetchAndSetStatus(ctx); err != nil {
				return "", err
			}

			return "", nil
		}

		bo := backoff.NewExponentialBackOff()
		bo.MaxInterval = 15 * time.Second

		retryOpts := []backoff.RetryOption{
			backoff.WithBackOff(backoff.NewExponentialBackOff()),
			backoff.WithMaxElapsedTime(3 * time.Minute),
			backoff.WithNotify(func(err error, duration time.Duration) {
				c.log.WithError(err).Warnf("Failed to fetch initial status, retrying in %s", duration)
			}),
		}

		if _, err := backoff.Retry(ctx, operation, retryOpts...); err != nil {
			c.log.WithError(err).Warn("Failed to fetch initial status")
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

		// Start the node dialer
		if err := c.startDialer(ctx); err != nil {
			c.log.WithError(err).Fatal("Failed to start node dialer")
		}

		c.log.Info("Successfully started node dialer")

		// Start the retry worker
		if err := c.startRetryWorker(ctx); err != nil {
			c.log.WithError(err).Fatal("Failed to start retry worker")
		}

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

		// Wait until we have a valid status (with timeout).
		waitStart := time.Now()
		for c.GetStatus().HeadSlot == 0 {
			if time.Since(waitStart) > 30*time.Second {
				c.log.Warn("Timeout waiting for valid status, marking crawler as ready anyway")

				break
			}

			c.log.Info("Waiting for valid status, current HeadSlot is 0")

			time.Sleep(time.Second)
		}

		c.log.WithField("head_slot", c.GetStatus().HeadSlot).Info("Crawler is ready")

		close(c.OnReady)

		return nil
	})

	// Start the beacon node
	if err := c.beacon.Start(c.ctx); err != nil {
		return fmt.Errorf("failed to start beacon node: %w", err)
	}

	return nil
}

func (c *Crawler) Stop(ctx context.Context) error {
	var finalErr error

	c.shutdownOnce.Do(func() {
		c.log.Info("Stopping crawler...")

		// Mark as shutting down
		c.shutdownMu.Lock()
		c.isShutdown = true
		c.shutdownMu.Unlock()

		// Cancel the context to stop all operations
		if c.cancel != nil {
			c.cancel()
		}

		var errs []error

		// Stop the beacon node first to prevent any more head events
		if c.beacon != nil {
			c.log.Info("Stopping beacon node...")

			if err := c.beacon.Stop(ctx); err != nil {
				errs = append(errs, fmt.Errorf("failed to stop beacon node: %w", err))
			}
		}

		// Stop the cron scheduler
		if c.scheduler != nil {
			c.log.Info("Stopping cron scheduler...")

			if err := c.scheduler.StopJobs(); err != nil {
				errs = append(errs, fmt.Errorf("failed to stop scheduler jobs: %w", err))
			}

			// Shutdown waits for running jobs to complete
			if err := c.scheduler.Shutdown(); err != nil {
				errs = append(errs, fmt.Errorf("failed to shutdown scheduler: %w", err))
			}
		}

		// Stop discovery
		if c.discovery != nil {
			c.log.Info("Stopping discovery...")

			if err := c.discovery.Stop(ctx); err != nil {
				errs = append(errs, fmt.Errorf("failed to stop discovery: %w", err))
			}
		}

		// Close the retry queue channel to stop retry worker
		if c.retryQueue != nil {
			c.log.Info("Closing retry queue channel...")
			close(c.retryQueue)
		}

		// Close the dialer channel to stop dialer workers
		if c.peersToDial != nil {
			c.log.Info("Closing dialer channel...")

			close(c.peersToDial)
		}

		// Wait for dialer workers to finish with timeout
		dialerDone := make(chan struct{})
		go func() {
			c.dialerWg.Wait()
			close(dialerDone)
		}()

		select {
		case <-dialerDone:
			c.log.Info("All dialer workers stopped")
		case <-time.After(5 * time.Second):
			c.log.Warn("Timeout waiting for dialer workers to stop")
		}

		// Stop the duplicate cache
		if c.duplicateCache != nil {
			if err := c.duplicateCache.Stop(); err != nil {
				errs = append(errs, fmt.Errorf("failed to stop duplicate cache: %w", err))
			}
		}

		// Tell all our peers we're disconnecting
		if c.node != nil {
			peerstore := c.node.Peerstore()
			if peerstore != nil {
				for _, p := range peerstore.Peers() {
					if err := c.DisconnectFromPeer(ctx, p, eth.GoodbyeReasonClientShutdown); err != nil {
						c.log.WithError(err).Debug("Failed to disconnect from peer on shutdown")
					}
				}
			}
		}

		// Stop the libp2p node
		if c.node != nil {
			if err := c.node.Stop(ctx); err != nil {
				errs = append(errs, fmt.Errorf("failed to stop node: %w", err))
			}
		}

		// Clear ENR storage
		c.peerENRsMu.Lock()
		c.peerENRs = make(map[peer.ID]*enode.Node)
		c.peerENRsMu.Unlock()

		// Clear retry tracker
		c.retryMu.Lock()
		c.retryTracker = make(map[peer.ID]*RetryInfo)
		c.retryMu.Unlock()

		c.log.Info("Crawler stopped")

		if len(errs) > 0 {
			finalErr = fmt.Errorf("errors during shutdown: %v", errs)
		}
	})

	return finalErr
}

func (c *Crawler) startCrons(ctx context.Context) error {
	s, err := gocron.NewScheduler(gocron.WithLocation(time.Local))
	if err != nil {
		return err
	}

	if _, err := s.NewJob(
		gocron.DurationJob(1*time.Minute),
		gocron.NewTask(
			func() {
				// Check if we're shutting down
				c.shutdownMu.RLock()
				if c.isShutdown {
					c.shutdownMu.RUnlock()

					return
				}
				c.shutdownMu.RUnlock()

				// Use a timeout context for the status fetch.
				fetchCtx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
				defer cancel()

				if err := c.fetchAndSetStatus(fetchCtx); err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}

					c.log.WithError(err).Error("Failed to fetch and set status")
				}
			},
		),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	); err != nil {
		return err
	}

	c.scheduler = s
	s.Start()

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

func (c *Crawler) nodeIsOnOurNetwork(node *enode.Node) error {
	sszEncodedForkEntry := make([]byte, 16)

	entry := enr.WithEntry("eth2", &sszEncodedForkEntry)

	if err := node.Record().Load(entry); err != nil {
		return fmt.Errorf("failed to load enr fork entry: %w", err)
	}

	forkEntry := &pb.ENRForkID{}
	if err := forkEntry.UnmarshalSSZ(sszEncodedForkEntry); err != nil {
		return fmt.Errorf("failed to unmarshal enr fork entry: %w", err)
	}

	status := c.GetStatus()

	if !bytes.Equal(forkEntry.CurrentForkDigest, status.ForkDigest[:]) {
		err := ErrCrawlENRForkDigest.WithDetails(fmt.Sprintf("theirs 0x%s != ours %s", hex.EncodeToString(forkEntry.CurrentForkDigest), status.ForkDigest.String()))

		return &err
	}

	return nil
}

func (c *Crawler) ConnectToPeer(ctx context.Context, p peer.AddrInfo, n *enode.Node) error {
	if err := c.nodeIsOnOurNetwork(n); err != nil {
		c.handleCrawlFailure(p.ID, newCrawlError(err.Error()))

		return nil
	}

	if c.duplicateCache.GetCache().Get(p.ID.String()) != nil {
		c.log.WithFields(logrus.Fields{
			"peer": p.ID,
		}).Debug("We've already connected to this peer previously")

		c.handleCrawlFailure(p.ID, ErrCrawlTooSoon)

		return nil
	}

	c.log.WithFields(logrus.Fields{
		"peer_id": p.ID.String(),
		"enr":     n.String(),
		"node_id": n.ID().String(),
	}).Debug("Crawler: Dialing peer")

	// Check if we're already connected to the peer
	if status := c.node.Connectedness(p.ID); status == network.Connected {
		return errors.New("already connected to peer")
	}

	// Store the ENR for this peer
	c.peerENRsMu.Lock()
	c.peerENRs[p.ID] = n
	c.peerENRsMu.Unlock()

	c.log.WithFields(logrus.Fields{
		"peer_id": p.ID.String(),
		"enr":     n.String(),
		"node_id": n.ID().String(),
	}).Debug("Crawler: Stored ENR for peer")

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
		ProtocolID: eth.GoodbyeV1ProtocolID,
		PeerID:     peerID,
		Payload:    &goodbye,
		Timeout:    time.Second * 5,
	}, &resp)

	// Clean up ENR storage
	c.peerENRsMu.Lock()
	delete(c.peerENRs, peerID)
	c.peerENRsMu.Unlock()

	// Always disconnect regardless of goodbye message status
	return c.node.DisconnectFromPeer(ctx, peerID)
}

func (c *Crawler) RequestStatusFromPeer(ctx context.Context, peerID peer.ID) (*common.Status, error) {
	status := c.GetStatus()

	req := &p2p.Request{
		ProtocolID: eth.StatusV1ProtocolID,
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
	c.log.WithField("peer", peerID.String()).Debug("Requesting metadata from peer")

	// NOTE: Per the Ethereum consensus spec, metadata requests have NO payload.
	// We must send nil as the payload to comply with the specification.
	// See: https://github.com/ethereum/consensus-specs/blob/dev/specs/phase0/p2p-interface.md#getmetadata-v1
	req := &p2p.Request{
		ProtocolID: eth.MetaDataV2ProtocolID,
		PeerID:     peerID,
		Payload:    nil, // Metadata requests have no payload per spec
		Timeout:    time.Second * 30,
	}

	rsp := &common.MetaData{}

	if err := c.reqResp.SendRequest(ctx, req, rsp); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	c.log.WithFields(logrus.Fields{
		"peer":       peerID.String(),
		"seq_number": rsp.SeqNumber,
		"attnets":    fmt.Sprintf("%x", rsp.Attnets),
	}).Debug("Successfully received metadata")

	c.emitMetadataReceived(peerID, rsp)

	return rsp, nil
}

func (c *Crawler) GetPeerAgentVersion(peerID peer.ID) string {
	rawAgentVersion, err := c.node.Peerstore().Get(peerID, "AgentVersion")
	if err != nil {
		return unknown
	}

	agentVersion, ok := rawAgentVersion.(string)
	if !ok {
		return unknown
	}

	return agentVersion
}

func (c *Crawler) GetPeerENR(peerID peer.ID) *enode.Node {
	c.peerENRsMu.RLock()
	defer c.peerENRsMu.RUnlock()

	return c.peerENRs[peerID]
}

func (c *Crawler) fetchAndSetStatus(ctx context.Context) error {
	status := &common.Status{}

	// Fetch the status from the upstream beacon node.
	checkpoint, err := c.beacon.Node().FetchFinality(ctx, "head")
	if err != nil {
		return fmt.Errorf("failed to fetch finality: %w", err)
	}

	sp, err := c.beacon.Node().Spec()
	if err != nil {
		return fmt.Errorf("failed to fetch spec: %w", err)
	}

	_, epoch, err := c.beacon.Node().Wallclock().Now()
	if err != nil {
		return fmt.Errorf("failed to fetch wallclock: %w", err)
	}

	currentFork, err := sp.ForkEpochs.CurrentFork(phase0.Epoch(epoch.Number()))
	if err != nil {
		return fmt.Errorf("failed to fetch current fork: %w", err)
	}

	status.FinalizedRoot = tree.Root(checkpoint.Finalized.Root)
	status.FinalizedEpoch = common.Epoch(checkpoint.Finalized.Epoch)

	headers, err := c.beacon.Node().FetchBeaconBlockHeader(ctx, &api.BeaconBlockHeaderOpts{
		Block: "head",
	})
	if err != nil {
		return fmt.Errorf("failed to fetch block headers: %w", err)
	}

	status.HeadRoot = tree.Root(headers.Root)
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
			"current_fork":    currentFork.Name,
		}).Debug("New eth2 status set")
	}

	// Set our status
	c.SetStatus(status)

	return nil
}

// handleCrawlFailure handles a crawl failure by either scheduling a retry or emitting a failure event.
func (c *Crawler) handleCrawlFailure(peerID peer.ID, err CrawlError) {
	// Check if error is retryable, if not, emit failure.
	if !c.isRetryableError(err) {
		c.emitFailedCrawl(peerID, err)

		return
	}

	// If max retry attempts is 0, don't retry at all
	if c.config.MaxRetryAttempts == 0 {
		c.log.WithFields(logrus.Fields{
			"peer":  peerID,
			"error": err,
		}).Debug("Max retry attempts is 0, emitting failure without retry")

		c.emitFailedCrawl(peerID, err)

		return
	}

	// Check existing retry info
	c.retryMu.RLock()
	retryInfo, exists := c.retryTracker[peerID]
	c.retryMu.RUnlock()

	// Check if we've exceeded max attempts
	if exists && retryInfo.Attempts >= c.config.MaxRetryAttempts {
		// Max attempts reached, emit failure
		c.log.WithFields(logrus.Fields{
			"peer":     peerID,
			"attempts": retryInfo.Attempts,
			"error":    err,
		}).Info("Max retry attempts reached, emitting failure")

		// Clean up retry tracking
		c.retryMu.Lock()
		delete(c.retryTracker, peerID)
		c.retryMu.Unlock()

		c.emitFailedCrawl(peerID, err)

		return
	}

	// Try to schedule a retry; if no ENR is stored, can't retry, emit failure
	enr := c.GetPeerENR(peerID)
	if enr == nil {
		c.emitFailedCrawl(peerID, err)

		return
	}

	// Create a ConnectablePeer from the ENR, if we can't derive peer details, emit failure.
	peer, convErr := discovery.DeriveDetailsFromNode(enr)
	if convErr != nil {
		c.emitFailedCrawl(peerID, err)

		return
	}

	// Schedule retry - this will handle the retry logic.
	c.scheduleRetry(peerID, peer, enr, err)

	// Don't emit failure event - as we're retrying.
	c.log.WithFields(logrus.Fields{
		"peer":  peerID,
		"error": err,
	}).Info("Scheduling retry instead of emitting failure")
}

// scheduleRetry schedules a peer for retry if conditions are met.
func (c *Crawler) scheduleRetry(peerID peer.ID, peer *discovery.ConnectablePeer, enr *enode.Node, err error) {
	// Check if error is retryable
	if !c.isRetryableError(err) {
		c.log.WithFields(logrus.Fields{
			"peer":  peerID,
			"error": err,
		}).Info("Error is not retryable, skipping retry")

		return
	}

	// If max retry attempts is 0, don't schedule any retries
	if c.config.MaxRetryAttempts == 0 {
		c.log.WithField("peer", peerID).Debug("Max retry attempts is 0, not scheduling retry")

		return
	}

	// Check existing retry info
	c.retryMu.Lock()
	defer c.retryMu.Unlock()

	retryInfo, exists := c.retryTracker[peerID]
	if !exists {
		// First retry attempt
		retryInfo = &RetryInfo{
			Peer:         peer,
			ENR:          enr,
			Attempts:     0,
			LastAttempt:  time.Now(),
			BackoffDelay: c.config.RetryBackoff,
		}

		c.retryTracker[peerID] = retryInfo
	}

	// Check if we've exceeded max attempts
	if retryInfo.Attempts >= c.config.MaxRetryAttempts {
		c.log.WithFields(logrus.Fields{
			"peer":     peerID,
			"attempts": retryInfo.Attempts,
		}).Debug("Max retry attempts reached, not scheduling retry")

		delete(c.retryTracker, peerID)

		return
	}

	// Update retry info
	retryInfo.Attempts++
	retryInfo.LastAttempt = time.Now()
	retryInfo.BackoffDelay = time.Duration(float64(c.config.RetryBackoff) * float64(retryInfo.Attempts))

	// Remove from duplicate cache to allow immediate retry
	c.duplicateCache.GetCache().Delete(peerID.String())

	// Schedule retry
	select {
	case c.retryQueue <- retryInfo:
		c.log.WithFields(logrus.Fields{
			"peer":     peerID,
			"attempts": retryInfo.Attempts,
			"backoff":  retryInfo.BackoffDelay,
		}).Info("Scheduled peer for retry")
	default:
		c.log.WithField("peer", peerID).Warn("Retry queue full, dropping retry request")
	}
}

// isRetryableError determines if an error should trigger a retry.
func (c *Crawler) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	var crawlErr CrawlError
	if errors.As(err, &crawlErr) {
		switch crawlErr.Type() {
		case ErrCrawlNetworkMismatchType:
			// Different network; don't retry.
			return false
		case ErrCrawlProtocolNotSupportedType:
			// Protocol isn't supported; don't retry.
			return false
		default:
			// Default to retry for ErrCrawlIdentifyTimeoutType, ErrCrawlTimeoutType and unknowns.
			return true
		}
	}

	// Check for context errors.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Default to retry for other errors.
	return true
}
