package mimicry

import (
	"context"
)

const (
	topicDisconnect                   = "disconnect"
	topicHello                        = "hello"
	topicStatus                       = "status"
	topicTransactions                 = "transactions"
	topicNewPooledTransactionHashes   = "new_pooled_transaction_hashes"
	topicNewPooledTransactionHashes68 = "new_pooled_transaction_hashes_68"
)

func (m *Mimicry) publishDisconnect(ctx context.Context, reason *Disconnect) {
	m.broker.Emit(topicDisconnect, reason)
}

func (m *Mimicry) publishHello(ctx context.Context, status *Hello) {
	m.broker.Emit(topicHello, status)
}

func (m *Mimicry) publishStatus(ctx context.Context, status *Status) {
	m.broker.Emit(topicStatus, status)
}

func (m *Mimicry) publishTransactions(ctx context.Context, transactions *Transactions) {
	m.broker.Emit(topicTransactions, transactions)
}

func (m *Mimicry) publishNewPooledTransactionHashes(ctx context.Context, hashes *NewPooledTransactionHashes) {
	m.broker.Emit(topicNewPooledTransactionHashes, hashes)
}

func (m *Mimicry) publishNewPooledTransactionHashes68(ctx context.Context, hashes *NewPooledTransactionHashes68) {
	m.broker.Emit(topicNewPooledTransactionHashes68, hashes)
}

func (m *Mimicry) handleSubscriberError(err error, topic string) {
	if err != nil {
		m.log.WithError(err).WithField("topic", topic).Error("Subscriber error")
	}
}

func (m *Mimicry) OnDisconnect(ctx context.Context, handler func(ctx context.Context, reason *Disconnect) error) {
	m.broker.On(topicDisconnect, func(reason *Disconnect) {
		m.handleSubscriberError(handler(ctx, reason), topicDisconnect)
	})
}

func (m *Mimicry) OnHello(ctx context.Context, handler func(ctx context.Context, status *Hello) error) {
	m.broker.On(topicHello, func(status *Hello) {
		m.handleSubscriberError(handler(ctx, status), topicHello)
	})
}

func (m *Mimicry) OnStatus(ctx context.Context, handler func(ctx context.Context, status *Status) error) {
	m.broker.On(topicStatus, func(status *Status) {
		m.handleSubscriberError(handler(ctx, status), topicStatus)
	})
}

func (m *Mimicry) OnTransactions(ctx context.Context, handler func(ctx context.Context, transactions *Transactions) error) {
	m.broker.On(topicTransactions, func(transactions *Transactions) {
		m.handleSubscriberError(handler(ctx, transactions), topicTransactions)
	})
}

func (m *Mimicry) OnNewPooledTransactionHashes(ctx context.Context, handler func(ctx context.Context, hashes *NewPooledTransactionHashes) error) {
	m.broker.On(topicNewPooledTransactionHashes, func(hashes *NewPooledTransactionHashes) {
		m.handleSubscriberError(handler(ctx, hashes), topicNewPooledTransactionHashes)
	})
}

func (m *Mimicry) OnNewPooledTransactionHashes68(ctx context.Context, handler func(ctx context.Context, hashes *NewPooledTransactionHashes68) error) {
	m.broker.On(topicNewPooledTransactionHashes68, func(hashes *NewPooledTransactionHashes68) {
		m.handleSubscriberError(handler(ctx, hashes), topicNewPooledTransactionHashes68)
	})
}
