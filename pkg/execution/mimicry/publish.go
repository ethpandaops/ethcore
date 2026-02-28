package mimicry

import (
	"context"
)

// Client event topics.
const (
	topicDisconnect                 = "disconnect"
	topicHello                      = "hello"
	topicStatus                     = "status"
	topicTransactions               = "transactions"
	topicNewPooledTransactionHashes = "new_pooled_transaction_hashes"
)

// Server event topics.
const (
	topicServerPeerConnected              = "server:peer:connected"
	topicServerPeerDisconnected           = "server:peer:disconnected"
	topicServerDisconnect                 = "server:disconnect"
	topicServerHello                      = "server:hello"
	topicServerStatus                     = "server:status"
	topicServerTransactions               = "server:transactions"
	topicServerNewPooledTransactionHashes = "server:new_pooled_transaction_hashes"
)

func (c *Client) publishDisconnect(ctx context.Context, reason *Disconnect) {
	c.broker.Emit(topicDisconnect, reason)
}

func (c *Client) publishHello(ctx context.Context, status *Hello) {
	c.broker.Emit(topicHello, status)
}

func (c *Client) publishStatus(ctx context.Context, status Status) {
	c.broker.Emit(topicStatus, status)
}

func (c *Client) publishTransactions(ctx context.Context, transactions *Transactions) {
	c.broker.Emit(topicTransactions, transactions)
}

func (c *Client) publishNewPooledTransactionHashes(ctx context.Context, hashes *NewPooledTransactionHashes) {
	c.broker.Emit(topicNewPooledTransactionHashes, hashes)
}

func (c *Client) handleSubscriberError(err error, topic string) {
	if err != nil {
		c.log.WithError(err).WithField("topic", topic).Error("Subscriber error")
	}
}

func (c *Client) OnDisconnect(ctx context.Context, handler func(ctx context.Context, reason *Disconnect) error) {
	c.broker.On(topicDisconnect, func(reason *Disconnect) {
		c.handleSubscriberError(handler(ctx, reason), topicDisconnect)
	})
}

func (c *Client) OnHello(ctx context.Context, handler func(ctx context.Context, status *Hello) error) {
	c.broker.On(topicHello, func(status *Hello) {
		c.handleSubscriberError(handler(ctx, status), topicHello)
	})
}

func (c *Client) OnStatus(ctx context.Context, handler func(ctx context.Context, status Status) error) {
	c.broker.On(topicStatus, func(status Status) {
		c.handleSubscriberError(handler(ctx, status), topicStatus)
	})
}

func (c *Client) OnTransactions(ctx context.Context, handler func(ctx context.Context, transactions *Transactions) error) {
	c.broker.On(topicTransactions, func(transactions *Transactions) {
		c.handleSubscriberError(handler(ctx, transactions), topicTransactions)
	})
}

func (c *Client) OnNewPooledTransactionHashes(ctx context.Context, handler func(ctx context.Context, hashes *NewPooledTransactionHashes) error) {
	c.broker.On(topicNewPooledTransactionHashes, func(hashes *NewPooledTransactionHashes) {
		c.handleSubscriberError(handler(ctx, hashes), topicNewPooledTransactionHashes)
	})
}

// Server publish methods

func (s *Server) publishPeerConnected(peer *ServerPeer) {
	s.broker.Emit(topicServerPeerConnected, peer)
}

func (s *Server) publishPeerDisconnected(peer *ServerPeer) {
	s.broker.Emit(topicServerPeerDisconnected, peer)
}

func (s *Server) publishDisconnect(ctx context.Context, peer *ServerPeer, reason *Disconnect) {
	s.broker.Emit(topicServerDisconnect, peer, reason)
}

func (s *Server) publishHello(ctx context.Context, peer *ServerPeer, hello *Hello) {
	s.broker.Emit(topicServerHello, peer, hello)
}

func (s *Server) publishStatus(ctx context.Context, peer *ServerPeer, status Status) {
	s.broker.Emit(topicServerStatus, peer, status)
}

func (s *Server) publishTransactions(ctx context.Context, peer *ServerPeer, transactions *Transactions) {
	s.broker.Emit(topicServerTransactions, peer, transactions)
}

func (s *Server) publishNewPooledTransactionHashes(ctx context.Context, peer *ServerPeer, hashes *NewPooledTransactionHashes) {
	s.broker.Emit(topicServerNewPooledTransactionHashes, peer, hashes)
}

func (s *Server) handleSubscriberError(err error, topic string) {
	if err != nil {
		s.log.WithError(err).WithField("topic", topic).Error("Subscriber error")
	}
}

// Server subscription methods

// OnPeerConnected registers a handler for when a peer connects.
func (s *Server) OnPeerConnected(handler func(peer *ServerPeer)) {
	s.broker.On(topicServerPeerConnected, handler)
}

// OnPeerDisconnected registers a handler for when a peer disconnects.
func (s *Server) OnPeerDisconnected(handler func(peer *ServerPeer)) {
	s.broker.On(topicServerPeerDisconnected, handler)
}

// OnDisconnect registers a handler for disconnect messages from peers.
func (s *Server) OnDisconnect(ctx context.Context, handler func(ctx context.Context, peer *ServerPeer, reason *Disconnect) error) {
	s.broker.On(topicServerDisconnect, func(peer *ServerPeer, reason *Disconnect) {
		s.handleSubscriberError(handler(ctx, peer, reason), topicServerDisconnect)
	})
}

// OnHello registers a handler for hello messages from peers.
func (s *Server) OnHello(ctx context.Context, handler func(ctx context.Context, peer *ServerPeer, hello *Hello) error) {
	s.broker.On(topicServerHello, func(peer *ServerPeer, hello *Hello) {
		s.handleSubscriberError(handler(ctx, peer, hello), topicServerHello)
	})
}

// OnStatus registers a handler for status messages from peers.
func (s *Server) OnStatus(ctx context.Context, handler func(ctx context.Context, peer *ServerPeer, status Status) error) {
	s.broker.On(topicServerStatus, func(peer *ServerPeer, status Status) {
		s.handleSubscriberError(handler(ctx, peer, status), topicServerStatus)
	})
}

// OnTransactions registers a handler for transaction messages from peers.
func (s *Server) OnTransactions(ctx context.Context, handler func(ctx context.Context, peer *ServerPeer, transactions *Transactions) error) {
	s.broker.On(topicServerTransactions, func(peer *ServerPeer, transactions *Transactions) {
		s.handleSubscriberError(handler(ctx, peer, transactions), topicServerTransactions)
	})
}

// OnNewPooledTransactionHashes registers a handler for new pooled transaction hash messages from peers.
func (s *Server) OnNewPooledTransactionHashes(ctx context.Context, handler func(ctx context.Context, peer *ServerPeer, hashes *NewPooledTransactionHashes) error) {
	s.broker.On(topicServerNewPooledTransactionHashes, func(peer *ServerPeer, hashes *NewPooledTransactionHashes) {
		s.handleSubscriberError(handler(ctx, peer, hashes), topicServerNewPooledTransactionHashes)
	})
}
