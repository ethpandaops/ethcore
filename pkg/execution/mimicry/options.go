package mimicry

import (
	"context"
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
)

type StatusProvider func(ctx context.Context, protocolVersion uint, peerStatus Status) (Status, error)

type HeaderProvider func(ctx context.Context, request *eth.GetBlockHeadersRequest) ([]*types.Header, error)

type BodyProvider func(ctx context.Context, hashes []common.Hash) ([]eth.BlockBody, error)

type ReceiptRequest struct {
	Hashes                 []common.Hash
	FirstBlockReceiptIndex uint64
}

type ReceiptProvider func(ctx context.Context, request ReceiptRequest) ([]*eth.ReceiptList, error)

type Option func(*Client)

func WithStatusProvider(provider StatusProvider) Option {
	return func(c *Client) {
		c.statusProvider = provider
	}
}

func WithHeaderProvider(provider HeaderProvider) Option {
	return func(c *Client) {
		c.headerProvider = provider
	}
}

func WithBodyProvider(provider BodyProvider) Option {
	return func(c *Client) {
		c.bodyProvider = provider
	}
}

func WithReceiptProvider(provider ReceiptProvider) Option {
	return func(c *Client) {
		c.receiptProvider = provider
	}
}

func WithPrivateKey(privateKey *ecdsa.PrivateKey) Option {
	return func(c *Client) {
		c.privateKey = privateKey
	}
}
