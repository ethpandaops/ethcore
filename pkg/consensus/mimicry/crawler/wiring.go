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
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/protolambda/zrnt/eth2/beacon/common"
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

	c.OnFailedCrawl(func(peerID peer.ID, reason CrawlError) {
		c.metrics.RecordFailedCrawl(reason.Error())
	})

	c.OnSuccessfulCrawl(func(peerID peer.ID, enr *enode.Node, status *common.Status, metadata *common.MetaData) {
		c.metrics.RecordSuccessfulCrawl(string(clients.ClientFromString(c.GetPeerAgentVersion(peerID))))
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

	goodbyeReason := eth.GoodbyeReasonClientShutdown

	c.log.WithFields(logrus.Fields{
		"peer": conn.RemotePeer().String(),
	}).Info("Peer connected")

	// Wait for libp2p identify protocol to complete.
	// The identify protocol exchanges peer information like agent version, protocols, etc.
	// Without this wait, we may see "unknown" agent versions which makes it hard to crawl/map.
	// We use a generous timeout to accommodate clients that take longer to initialize
	// in resource-constrained environments like test networks.
	identifyTimeout := 120 * time.Second
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
				c.log.WithFields(logrus.Fields{
					"peer":    conn.RemotePeer(),
					"timeout": identifyTimeout,
				}).Warn("Timeout waiting for identify protocol")
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
	// we mark this as a failed crawl since we can't properly identify the client.
	if !identifyCompleted && agentVersion == unknown {
		// Check if it was due to shutdown.
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		logCtx.Error("Failed to complete identify protocol")

		c.emitFailedCrawl(conn.RemotePeer(), ErrCrawlIdentifyTimeout)

		return
	}

	defer func() {
		// Disconnect them regardless of what happens
		// Use a fresh context with timeout for cleanup
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer disconnectCancel()

		if err := c.DisconnectFromPeer(disconnectCtx, conn.RemotePeer(), goodbyeReason); err != nil {
			logCtx.WithError(err).Error("Failed to disconnect from peer")
		}
	}()

	// Request status with retry logic.
	// Different beacon node implementations have varying initialization times after connection.
	// This retry mechanism ensures we don't prematurely fail connections to slower-initializing clients.
	// We use generous delays to accommodate all client types.
	var (
		retryDelay = 5 * time.Second
		status     *common.Status
		err        error
	)

	for retries := 0; retries < 3; retries++ {
		statusCtx, statusCancel := context.WithTimeout(c.ctx, 30*time.Second)
		status, err = c.RequestStatusFromPeer(statusCtx, conn.RemotePeer())

		statusCancel()

		if err == nil {
			break
		}

		// Check if the error is due to context cancellation (shutdown)
		if errors.Is(err, context.Canceled) {
			logCtx.Debug("Status request cancelled due to shutdown")

			return
		}

		if retries < 2 {
			logCtx.WithFields(logrus.Fields{
				"attempt":     retries + 1,
				"error":       err.Error(),
				"retry_delay": retryDelay,
			}).Info("Status request failed, retrying")

			select {
			case <-time.After(retryDelay):
				// Continue with retry
			case <-c.ctx.Done():
				logCtx.Debug("Retry cancelled due to shutdown")

				return
			}
		}
	}

	if err != nil {
		logCtx.WithError(err).Error("Failed to request status from peer after retries")

		c.emitFailedCrawl(conn.RemotePeer(), ErrCrawlFailedToRequestStatus)

		return
	}

	ourStatus := c.GetStatus()

	if status != nil && status.ForkDigest != ourStatus.ForkDigest {
		// They're on a different fork
		goodbyeReason = eth.GoodbyeReasonIrrelevantNetwork

		c.emitFailedCrawl(conn.RemotePeer(), ErrCrawlStatusForkDigest.WithDetails(fmt.Sprintf("ours %s != theirs %s", ourStatus.ForkDigest, status.ForkDigest)))

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

	// Request metadata from the peer with retry logic
	// Metadata contains information about attestation subnet participation and sync committee duties.
	metadataCtx, metadataCancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer metadataCancel()

	// Retry metadata requests with generous delays for all clients
	metadataRetryDelay := 3 * time.Second

	var metadata *common.MetaData
	for retries := 0; retries < 3; retries++ {
		metadata, err = c.RequestMetadataFromPeer(metadataCtx, conn.RemotePeer())
		if err == nil {
			break
		}

		// Check if the error is due to context cancellation (shutdown)
		if errors.Is(err, context.Canceled) {
			logCtx.Debug("Metadata request cancelled due to shutdown")

			return
		}

		if retries < 2 {
			logCtx.WithFields(logrus.Fields{
				"attempt":     retries + 1,
				"error":       err.Error(),
				"retry_delay": metadataRetryDelay,
			}).Error("Metadata request failed, retrying")

			select {
			case <-time.After(metadataRetryDelay):
				// Continue with retry
			case <-c.ctx.Done():
				logCtx.Debug("Metadata retry cancelled due to shutdown")

				return
			}
		}
	}

	if err != nil {
		logCtx.WithError(err).Error("Failed to request metadata from peer after retries")

		c.emitFailedCrawl(conn.RemotePeer(), ErrCrawlFailedToRequestMetadata)

		return
	}

	c.emitSuccessfulCrawl(conn.RemotePeer(), status, metadata)
}

func (c *Crawler) handlePeerDisconnected(net network.Network, conn network.Conn) {
	c.log.WithFields(logrus.Fields{
		"peer":          conn.RemotePeer(),
		"agent_version": c.GetPeerAgentVersion(conn.RemotePeer()),
	}).Debug("Disconnected from peer")
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

func (c *Crawler) handleNewDiscoveryNode(_ context.Context, node *enode.Node) error {
	// Dont proceed further if we're shutting down.
	c.shutdownMu.RLock()
	if c.isShutdown {
		c.shutdownMu.RUnlock()

		return nil
	}
	c.shutdownMu.RUnlock()

	c.log.WithFields(logrus.Fields{
		"node": node.String(),
	}).Trace("Enode received")

	n, err := discovery.DeriveDetailsFromNode(node)
	if err != nil {
		c.log.WithError(err).Error("Failed to derive peer details from node")

		// We don't care about this node
		return nil
	}

	// Check if they're on our network
	if err := c.nodeIsOnOurNetwork(n.Enode); err != nil {
		c.log.WithFields(logrus.Fields{
			"node":  n.Enode.String(),
			"error": err.Error(),
		}).Trace("Node is not on our network")

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
	default:
		c.log.WithFields(logrus.Fields{
			"node": n.Enode.String(),
		}).Warn("Dropping potential peer: pending peers channel is full. Consider increasing the dial concurrency")
	}

	return nil
}
