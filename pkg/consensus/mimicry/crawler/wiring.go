package crawler

import (
	"context"
	"errors"
	"fmt"
	"time"

	v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth"
	"github.com/ethpandaops/ethcore/pkg/discovery"
	"github.com/ethpandaops/ethcore/pkg/ethereum/clients"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/sirupsen/logrus"
)

// wireUpComponents wires up the components of the crawler so it can function.
func (c *Crawler) wireUpComponents(ctx context.Context) error {
	// Wire up the libp2p host
	c.node.AfterPeerConnect(c.handlePeerConnected)
	c.node.AfterPeerDisconnect(c.handlePeerDisconnected)

	// Wire up the beacon node
	c.beacon.Node().OnHead(ctx, func(ctx context.Context, event *v1.HeadEvent) error {
		// If we're shutting down, we're done. Dont process any further.
		c.shutdownMu.RLock()
		if c.isShutdown {
			c.shutdownMu.RUnlock()

			return nil
		}

		c.shutdownMu.RUnlock()

		return c.fetchAndSetStatus(ctx)
	})

	// Wire up the discovery
	c.discovery.OnNodeRecord(ctx, c.handleNewDiscoveryNode)

	// Wire up the req/resp
	if err := c.reqResp.RegisterHandler(ctx, eth.StatusV1ProtocolID, c.handleStatus); err != nil {
		return fmt.Errorf("failed to register status handler: %w", err)
	}

	if err := c.reqResp.RegisterHandler(ctx, eth.GoodbyeV1ProtocolID, c.handleGoodbye); err != nil {
		return fmt.Errorf("failed to register goodbye handler: %w", err)
	}

	if err := c.reqResp.RegisterHandler(ctx, eth.PingV1ProtocolID, c.handlePing); err != nil {
		return fmt.Errorf("failed to register ping handler: %w", err)
	}

	if err := c.reqResp.RegisterHandler(ctx, eth.MetaDataV2ProtocolID, c.handleMetadata); err != nil {
		return fmt.Errorf("failed to register metadata handler: %w", err)
	}

	// Register dummy RPC handlers for the ones we don't implement yet
	// Beacon blocks
	if err := c.reqResp.RegisterHandler(ctx, eth.BeaconBlocksByRangeV1ProtocolID, c.handleDummyRPC); err != nil {
		return fmt.Errorf("failed to register dummy RPC handler: %w", err)
	}

	if err := c.reqResp.RegisterHandler(ctx, eth.BeaconBlocksByRootV1ProtocolID, c.handleDummyRPC); err != nil {
		return fmt.Errorf("failed to register dummy RPC handler: %w", err)
	}

	if err := c.reqResp.RegisterHandler(ctx, eth.BeaconBlocksByRangeV2ProtocolID, c.handleDummyRPC); err != nil {
		return fmt.Errorf("failed to register dummy RPC handler: %w", err)
	}

	if err := c.reqResp.RegisterHandler(ctx, eth.BeaconBlocksByRootV2ProtocolID, c.handleDummyRPC); err != nil {
		return fmt.Errorf("failed to register dummy RPC handler: %w", err)
	}

	// Beacon blobs
	if err := c.reqResp.RegisterHandler(ctx, eth.BlobSidecarsByRangeV1ProtocolID, c.handleDummyRPC); err != nil {
		return fmt.Errorf("failed to register dummy RPC handler: %w", err)
	}

	if err := c.reqResp.RegisterHandler(ctx, eth.BlobSidecarsByRootV1ProtocolID, c.handleDummyRPC); err != nil {
		return fmt.Errorf("failed to register dummy RPC handler: %w", err)
	}

	c.OnFailedCrawl(func(crawl *FailedCrawl) {
		c.metrics.RecordFailedCrawl(crawl.Error.Error())
	})

	c.OnSuccessfulCrawl(func(crawl *SuccessfulCrawl) {
		c.metrics.RecordSuccessfulCrawl(string(clients.ClientFromString(c.GetPeerAgentVersion(crawl.PeerID))))
	})

	return nil
}

func (c *Crawler) handlePeerConnected(net network.Network, conn network.Conn) {
	// Don't process the peer any further if we're shutting down.
	c.shutdownMu.RLock()
	if c.isShutdown {
		c.shutdownMu.RUnlock()

		_ = conn.Close()

		return
	}
	c.shutdownMu.RUnlock()

	peerID := conn.RemotePeer()

	// Check if we've recently processed this peer to prevent duplicate handling
	if c.duplicateCache.GetCache().Get(peerID.String()) != nil {
		c.log.WithFields(logrus.Fields{
			"peer": peerID.String(),
		}).Debug("Skipping duplicate connection event - peer recently processed")

		_ = conn.Close()

		return
	}

	c.log.WithFields(logrus.Fields{
		"peer": peerID.String(),
	}).Info("Peer connected")

	// Wait for libp2p identify protocol to complete.
	// The identify protocol exchanges peer information like agent version, protocols, etc.
	// Without this wait, we may see "unknown" agent versions which makes it hard to crawl/map.
	// If it fails, we'll retry via handleCrawlFailure.
	identifyTimeout := 15 * time.Second
	identifyCtx, identifyCancel := context.WithTimeout(c.ctx, identifyTimeout)

	defer identifyCancel()

	// Poll for agent version to become available, which indicates identify has completed.
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	identifyCompleted := false

	for {
		select {
		case <-identifyCtx.Done():
			// Check if it's due to shutdown or actual timeout and log appropriately.
			select {
			case <-c.ctx.Done():
				c.log.WithFields(logrus.Fields{
					"peer": conn.RemotePeer(),
				}).Debug("Identify protocol cancelled due to shutdown")
			default:
			}

			break
		case <-ticker.C:
			if agentVersion := c.GetPeerAgentVersion(conn.RemotePeer()); agentVersion != unknown {
				c.log.WithFields(logrus.Fields{
					"peer":          conn.RemotePeer(),
					"agent_version": agentVersion,
				}).Info("Identify protocol completed")

				identifyCompleted = true

				break
			}
		}

		// Check if we should exit the loop.
		if identifyCtx.Err() != nil || identifyCompleted {
			break
		}
	}

	agentVersion := c.GetPeerAgentVersion(conn.RemotePeer())
	logCtx := c.log.WithFields(logrus.Fields{
		"peer":          conn.RemotePeer(),
		"agent_version": agentVersion,
	})

	// If we couldn't get the agent version through identify protocol,
	// we'll let this retry via handleCrawlFailure.
	if !identifyCompleted && agentVersion == unknown {
		// Check if it was due to shutdown.
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		c.handleCrawlFailure(conn.RemotePeer(), ErrCrawlIdentifyTimeout)

		return
	}

	// Add peer to duplicate cache to prevent processing duplicate connection events
	c.duplicateCache.GetCache().Set(peerID.String(), time.Now(), c.config.CooloffDuration)
	logCtx.WithField("cooloff_duration", c.config.CooloffDuration).Debug("Added peer to duplicate cache")

	// Request status from peer.
	statusCtx, statusCancel := context.WithTimeout(c.ctx, 30*time.Second)

	status, err := c.RequestStatusFromPeer(statusCtx, conn.RemotePeer())

	statusCancel()

	if err != nil {
		// Check if the error is due to context cancellation (shutdown)
		if errors.Is(err, context.Canceled) {
			logCtx.Debug("Status request cancelled due to shutdown")

			return
		}

		logCtx.WithError(err).Debug("Failed to request status from peer")

		c.handleCrawlFailure(conn.RemotePeer(), ErrCrawlFailedToRequestStatus)

		return
	}

	ourStatus := c.GetStatus()

	if status != nil && status.ForkDigest != ourStatus.ForkDigest {
		// They're on a different fork
		c.handleCrawlFailure(conn.RemotePeer(), ErrCrawlStatusForkDigest.WithDetails(fmt.Sprintf("ours %s != theirs %s", ourStatus.ForkDigest, status.ForkDigest)))

		return
	}

	logCtx.WithFields(logrus.Fields{
		"fork_digest":     status.ForkDigest,
		"finalized_root":  status.FinalizedRoot.String(),
		"finalized_epoch": status.FinalizedEpoch,
		"head_root":       status.HeadRoot.String(),
		"head_slot":       status.HeadSlot,
		"connected":       c.node.Connectedness(conn.RemotePeer()),
	}).Debug("Received status from peer")

	// Request metadata from the peer.
	// Metadata contains information about attestation subnet participation and sync committee duties.
	metadataCtx, metadataCancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer metadataCancel()

	metadata, err := c.RequestMetadataFromPeer(metadataCtx, conn.RemotePeer())
	if err != nil {
		// Check if the error is due to context cancellation (shutdown)
		if errors.Is(err, context.Canceled) {
			logCtx.Debug("Metadata request cancelled due to shutdown")

			return
		}

		logCtx.WithError(err).Debug("Failed to request metadata from peer")

		c.handleCrawlFailure(conn.RemotePeer(), ErrCrawlFailedToRequestMetadata)

		return
	}

	// Clean up any retry tracking for this peer
	c.retryMu.Lock()
	delete(c.retryTracker, conn.RemotePeer())
	c.retryMu.Unlock()

	// Check if peer is still connected before proceeding. This prevents a race condition where
	// libp2p callbacks can execute after peer disconnection has begun but before network state
	// fully reflects the disconnection. Without this check, we could attempt to emit successful
	// crawl events for peers that are in the process of being cleaned up.
	if c.node.Connectedness(conn.RemotePeer()) != network.Connected {
		c.log.WithField("peer", conn.RemotePeer()).Debug("Peer no longer connected, skipping successful crawl emit")

		return
	}

	enr := c.GetPeerENR(conn.RemotePeer())
	if enr == nil {
		c.log.WithField("peer", conn.RemotePeer()).Debug("ENR not available for peer, skipping successful crawl emit")

		return
	}

	// Calculate datetime values for finalized epoch and head slot
	var (
		finalizedEpochStartDateTime *time.Time
		headSlotStartDateTime       *time.Time
	)

	if status != nil {
		wallclock := c.beacon.Node().Wallclock()

		// Calculate finalized epoch start datetime
		finalizedEpoch := wallclock.Epochs().FromNumber(uint64(status.FinalizedEpoch))
		startTime := finalizedEpoch.TimeWindow().Start()
		finalizedEpochStartDateTime = &startTime

		// Calculate head slot start datetime
		headSlot := wallclock.Slots().FromNumber(uint64(status.HeadSlot))
		startTime = headSlot.TimeWindow().Start()
		headSlotStartDateTime = &startTime
	}

	c.emitSuccessfulCrawl(&SuccessfulCrawl{
		PeerID:                      conn.RemotePeer(),
		NodeID:                      enr.ID().String(),
		AgentVersion:                agentVersion,
		NetworkID:                   c.beacon.Metadata().GetNetwork().ID,
		ENR:                         enr,
		Metadata:                    metadata,
		Status:                      status,
		FinalizedEpochStartDateTime: finalizedEpochStartDateTime,
		HeadSlotStartDateTime:       headSlotStartDateTime,
	})
}

func (c *Crawler) handlePeerDisconnected(net network.Network, conn network.Conn) {
	c.log.WithFields(logrus.Fields{
		"peer":          conn.RemotePeer(),
		"agent_version": c.GetPeerAgentVersion(conn.RemotePeer()),
	}).Debug("Disconnected from peer")

	// Clean up ENR storage for naturally disconnected peers
	c.peerENRsMu.Lock()
	delete(c.peerENRs, conn.RemotePeer())
	c.peerENRsMu.Unlock()
}

//nolint:unused // Will revisit if not-needed.
func (c *Crawler) handleBeaconNodeReady(ctx context.Context) error {
	c.log.Info("Upstream beacon node is ready!")

	c.beacon.Node().OnHead(ctx, func(ctx context.Context, event *v1.HeadEvent) error {
		logctx := c.log.WithFields(logrus.Fields{
			"slot":  event.Slot,
			"event": "head",
		})

		logctx.Info("Beacon head event received")

		if err := c.fetchAndSetStatus(ctx); err != nil {
			logctx.WithError(err).Error("Failed to fetch and set status")
		}

		return nil
	})

	// Start crons
	if err := c.startCrons(ctx); err != nil {
		return err
	}

	return nil
}

func (c *Crawler) startDialer(ctx context.Context) error {
	c.log.WithField("concurrency", c.config.DialConcurrency).Info("Starting peer dialer")

	for i := 0; i < c.config.DialConcurrency; i++ {
		workerID := i

		c.dialerWg.Add(1)

		go func() {
			defer c.dialerWg.Done()

			for {
				select {
				case node, ok := <-c.peersToDial:
					if !ok {
						return
					}

					c.log.WithFields(logrus.Fields{
						"peer":      node.Enode.String(),
						"worker_id": workerID,
					}).Debug("Dialing new peer")

					c.metrics.RecordNodeProcessed()
					c.metrics.RecordPendingDials(len(c.peersToDial))

					// Check if we're shutting down before connecting.
					c.shutdownMu.RLock()
					if c.isShutdown {
						c.shutdownMu.RUnlock()

						return
					}
					c.shutdownMu.RUnlock()

					if err := c.ConnectToPeer(c.ctx, node.AddrInfo, node.Enode); err != nil {
						if errors.Is(err, context.Canceled) {
							return
						}

						c.log.WithError(err).Trace("Failed to connect to peer")
					}
				case <-c.ctx.Done():
					return
				}
			}
		}()
	}

	return nil
}

// startRetryWorker starts a worker that processes retry requests.
func (c *Crawler) startRetryWorker(ctx context.Context) error {
	c.log.WithFields(logrus.Fields{
		"max_attempts": c.config.MaxRetryAttempts,
		"backoff":      c.config.RetryBackoff,
	}).Info("Starting retry worker")

	c.dialerWg.Add(1)

	go func() {
		defer c.dialerWg.Done()

		for {
			select {
			case retryInfo, ok := <-c.retryQueue:
				if !ok {
					return
				}

				// Check if we're shutting down
				c.shutdownMu.RLock()
				if c.isShutdown {
					c.shutdownMu.RUnlock()

					return
				}
				c.shutdownMu.RUnlock()

				// Wait for backoff period
				backoffDuration := retryInfo.BackoffDelay
				c.log.WithFields(logrus.Fields{
					"peer":     retryInfo.Peer.AddrInfo.ID,
					"attempts": retryInfo.Attempts,
					"backoff":  backoffDuration,
				}).Debug("Waiting before retry attempt")

				select {
				case <-time.After(backoffDuration):
					// Continue with retry
				case <-ctx.Done():
					return
				}

				// Re-add to dialer queue
				select {
				case c.peersToDial <- retryInfo.Peer:
					c.log.WithFields(logrus.Fields{
						"peer":     retryInfo.Peer.AddrInfo.ID,
						"attempts": retryInfo.Attempts,
					}).Debug("Re-queued peer for retry")
				case <-ctx.Done():
					return
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (c *Crawler) handleNewDiscoveryNode(_ context.Context, node *enode.Node) error {
	// Dont proceed further if we're shutting down.
	c.shutdownMu.RLock()
	if c.isShutdown {
		c.shutdownMu.RUnlock()

		return nil
	}
	c.shutdownMu.RUnlock()

	enr := ""
	if node != nil {
		enr = node.String()
	}

	c.log.WithFields(logrus.Fields{
		"enr":     enr,
		"node_id": node.ID().String(),
	}).Debug("Crawler: Received node from discovery")

	n, err := discovery.DeriveDetailsFromNode(node)
	if err != nil {
		c.log.WithError(err).Error("Failed to derive peer details from node")

		// We don't care about this node
		return nil
	}

	// Check if they're on our network
	if err := c.nodeIsOnOurNetwork(n.Enode); err != nil {
		c.log.WithFields(logrus.Fields{
			"enr":     enr,
			"node_id": node.ID().String(),
			"error":   err.Error(),
		}).Debug("Crawler: Node is not on our network, skipping")

		//nolint:nilerr // We don't care about this node
		return nil
	}

	// If the channel is full, we drop the peer.
	c.metrics.RecordPendingDials(len(c.peersToDial))

	// Check again if we're shutting down before sending.
	c.shutdownMu.RLock()
	if c.isShutdown {
		c.shutdownMu.RUnlock()

		return nil
	}
	c.shutdownMu.RUnlock()

	select {
	case c.peersToDial <- n:
		c.log.WithFields(logrus.Fields{
			"enr":       n.Enode.String(),
			"node_id":   n.Enode.ID().String(),
			"peer_id":   n.AddrInfo.ID.String(),
			"num_addrs": len(n.AddrInfo.Addrs),
		}).Debug("Crawler: Added node to dial queue")
	default:
		c.log.WithFields(logrus.Fields{
			"enr":     n.Enode.String(),
			"node_id": n.Enode.ID().String(),
		}).Warn("Dropping potential peer: pending peers channel is full. Consider increasing the dial concurrency")
	}

	return nil
}
