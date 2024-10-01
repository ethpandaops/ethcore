package crawler

import (
	"context"
	"fmt"

	v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth"
	"github.com/ethpandaops/ethcore/pkg/discovery"
	"github.com/ethpandaops/ethcore/pkg/ethereum/beacon"
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
	c.beacon.OnReady(ctx, c.handleBeaconNodeReady)

	// Wire up the discovery
	c.discovery.OnNodeRecord(ctx, c.handleNewDiscoveryNode)

	// Wire up the req/resp
	if err := c.reqResp.RegisterHandler(ctx, eth.StatusProtocolID, c.handleStatus); err != nil {
		return fmt.Errorf("failed to register status handler: %w", err)
	}

	if err := c.reqResp.RegisterHandler(ctx, eth.GoodbyeProtocolID, c.handleGoodbye); err != nil {
		return fmt.Errorf("failed to register goodbye handler: %w", err)
	}

	if err := c.reqResp.RegisterHandler(ctx, eth.PingProtocolID, c.handlePing); err != nil {
		return fmt.Errorf("failed to register ping handler: %w", err)
	}

	if err := c.reqResp.RegisterHandler(ctx, eth.MetaDataProtocolID, c.handleMetadata); err != nil {
		return fmt.Errorf("failed to register metadata handler: %w", err)
	}

	c.OnFailedCrawl(func(peerID peer.ID, reason CrawlError) {
		c.metrics.RecordFailedCrawl(reason.Error())
	})

	c.OnSuccessfulCrawl(func(peerID peer.ID, status *common.Status, metadata *common.MetaData) {
		c.metrics.RecordSuccessfulCrawl(string(beacon.ClientFromString(c.GetPeerAgentVersion(peerID))))
	})

	return nil
}

func (c *Crawler) handlePeerConnected(net network.Network, conn network.Conn) {
	logCtx := c.log.WithFields(logrus.Fields{
		"peer":          conn.RemotePeer(),
		"agent_version": c.GetPeerAgentVersion(conn.RemotePeer()),
	})

	defer func() {
		// Disconnect them regardless of what happens
		if err := c.DisconnectFromPeer(context.Background(), conn.RemotePeer(), eth.GoodbyeReasonClientShutdown); err != nil {
			logCtx.WithError(err).Debug("Failed to disconnect from peer")
		}
	}()

	status, err := c.RequestStatusFromPeer(context.Background(), conn.RemotePeer())
	if err != nil {
		logCtx.WithError(err).Debug("Failed to request status from peer")
		c.emitFailedCrawl(conn.RemotePeer(), ErrCrawlFailedToRequestStatus)

		return
	}

	ourStatus := c.GetStatus()

	if status != nil && status.ForkDigest != ourStatus.ForkDigest {
		if errr := c.DisconnectFromPeer(context.Background(), conn.RemotePeer(), eth.GoodbyeReasonIrrelevantNetwork); errr != nil {
			logCtx.WithError(errr).Debug("Failed to disconnect from peer for incorrect fork digest")
		}

		c.emitFailedCrawl(conn.RemotePeer(), ErrCrawlStatusForkDigest)

		return
	}

	logCtx.WithFields(logrus.Fields{
		"fork_digest":     status.ForkDigest,
		"finalized_root":  status.FinalizedRoot.String(),
		"finalized_epoch": status.FinalizedEpoch,
		"head_root":       status.HeadRoot.String(),
		"head_slot":       status.HeadSlot,
	}).Debug("Received status from peer")

	// Request metadata from the peer
	metadata, err := c.RequestMetadataFromPeer(context.Background(), conn.RemotePeer())
	if err != nil {
		logCtx.WithError(err).Warn("Failed to request metadata from peer")

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

		go func() {
			for {
				node, ok := <-c.peersToDial
				if !ok {
					return
				}

				c.log.WithFields(logrus.Fields{
					"peer":      node.Enode.String(),
					"worker_id": workerID,
				}).Debug("Dialing new peer")

				c.metrics.RecordNodeProcessed()
				c.metrics.RecordPendingDials(len(c.peersToDial))

				if err := c.node.ConnectToPeer(ctx, node.AddrInfo); err != nil {
					c.log.WithError(err).Trace("Failed to connect to peer")
				}
			}
		}()
	}

	return nil
}

func (c *Crawler) handleNewDiscoveryNode(ctx context.Context, node *enode.Node) error {
	c.log.WithFields(logrus.Fields{
		"node": node.String(),
	}).Trace("Enode received")

	n, err := discovery.DeriveDetailsFromNode(node)
	if err != nil {
		c.log.WithError(err).Error("Failed to derive peer details from node")

		return nil
	}

	// Check if they're on our network
	if !c.nodeIsOnOurNetwork(n.Enode) {
		c.log.WithFields(logrus.Fields{
			"node": n.Enode.String(),
		}).Trace("Node is not on our network")
	}

	// If the channel is full, we drop the peer.
	c.metrics.RecordPendingDials(len(c.peersToDial))

	select {
	case c.peersToDial <- n:
	default:
		c.log.WithFields(logrus.Fields{
			"node": n.Enode.String(),
		}).Warn("Dropping potential peer: pending peers channel is full. Consider increasing the dial concurrency")
	}

	return nil
}
