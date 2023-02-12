package mimicry

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/chuckpreslar/emission"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/rlpx"
	"github.com/sirupsen/logrus"
)

type Mimicry struct {
	log logrus.FieldLogger

	nodeRecord *enode.Node
	name       string
	broker     *emission.Emitter

	privateKey *ecdsa.PrivateKey

	peer    *p2p.Peer
	msgPipe *p2p.MsgPipeRW
	ethPeer *eth.Peer

	conn     net.Conn
	rlpxConn *rlpx.Conn

	pooledTransactionsMap map[uint64]chan *PooledTransactions

	ethCapVersion uint
}

func parseNodeRecord(record string) (*enode.Node, error) {
	if strings.HasPrefix(record, "enode://") {
		return enode.ParseV4(record)
	}

	return enode.Parse(enode.ValidSchemes, record)
}

func New(ctx context.Context, log logrus.FieldLogger, record, name string) (*Mimicry, error) {
	nodeRecord, err := parseNodeRecord(record)
	if err != nil {
		return nil, err
	}

	return &Mimicry{
		log:                   log.WithField("node_record", nodeRecord.String()),
		nodeRecord:            nodeRecord,
		name:                  name,
		broker:                emission.NewEmitter(),
		pooledTransactionsMap: map[uint64]chan *PooledTransactions{},
	}, nil
}

func (m *Mimicry) Start(ctx context.Context) error {
	m.log.Debug("starting execution mimicry client")
	m.peer = p2p.NewPeer(m.nodeRecord.ID(), m.name, SupportedEthCaps())

	_, msgPipe := p2p.MsgPipe()
	m.msgPipe = msgPipe

	m.ethPeer = eth.NewPeer(maxETHProtocolVersion, m.peer, m.msgPipe, nil)

	address := m.nodeRecord.IP().String() + ":" + strconv.Itoa(m.nodeRecord.TCP())
	m.log.WithField("address", address).Debug("dialing peer")

	var err error

	// set a deadline for dialing
	d := net.Dialer{Deadline: time.Now().Add(5 * time.Second)}

	m.conn, err = d.Dial("tcp", address)
	if err != nil {
		return err
	}

	m.rlpxConn = rlpx.NewConn(m.conn, m.nodeRecord.Pubkey())

	// update deadline for handshake
	err = m.conn.SetDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		return err
	}

	m.privateKey, _ = crypto.GenerateKey()

	peerPublicKey, err := m.rlpxConn.Handshake(m.privateKey)
	if err != nil {
		return err
	}

	// clear deadline
	err = m.conn.SetDeadline(time.Time{})
	if err != nil {
		return err
	}

	m.log.WithFields(logrus.Fields{
		"peer_key": peerPublicKey,
		"address":  address,
	}).Debug("successfully handshaked")

	go func() {
		m.startSession(ctx)
	}()

	return nil
}

func (m *Mimicry) Stop(ctx context.Context) error {
	if m.peer != nil {
		m.peer.Disconnect(p2p.DiscQuitting)
	}

	if m.ethPeer != nil {
		m.ethPeer.Close()
	}

	if m.rlpxConn != nil {
		if err := m.rlpxConn.Close(); err != nil {
			m.log.WithError(err).Error("error closing rlpx connection")
		}
	}

	if m.msgPipe != nil {
		if err := m.msgPipe.Close(); err != nil {
			m.log.WithError(err).Error("error closing msg pipe")
		}
	}

	return nil
}

func (m *Mimicry) handleSessionError(ctx context.Context, err error) {
	m.log.WithError(err).Debug("error handling session")
	m.disconnect(ctx, nil)
}

func (m *Mimicry) disconnect(ctx context.Context, reason *Disconnect) {
	m.log.Debug("disconnecting from peer")
	m.publishDisconnect(ctx, reason)
}

func (m *Mimicry) startSession(ctx context.Context) {
	if err := m.sendHello(ctx); err != nil {
		m.handleSessionError(ctx, err)
		return
	}

	for {
		// set read deadline
		err := m.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		if err != nil {
			m.handleSessionError(ctx, fmt.Errorf("error setting rlpx read deadline: %w", err))
			return
		}

		code, data, _, err := m.rlpxConn.Read()
		if err != nil {
			m.handleSessionError(ctx, fmt.Errorf("error reading rlpx connection: %w", err))
			return
		}

		switch int(code) {
		case HelloCode:
			if err := m.handleHello(ctx, code, data); err != nil {
				m.handleSessionError(ctx, err)
				return
			}
		case DisconnectCode:
			m.handleDisconnect(ctx, code, data)
			return
		case PingCode:
			if err := m.handlePing(ctx, code, data); err != nil {
				m.handleSessionError(ctx, err)
				return
			}
		case StatusCode:
			if err := m.handleStatus(ctx, code, data); err != nil {
				m.handleSessionError(ctx, err)
				return
			}
		case TransactionsCode:
			if err := m.handleTransactions(ctx, code, data); err != nil {
				m.handleSessionError(ctx, err)
				return
			}
		case GetBlockHeadersCode:
			if err := m.handleGetBlockHeaders(ctx, code, data); err != nil {
				m.handleSessionError(ctx, err)
				return
			}
		case BlockHeadersCode:
			if err := m.handleBlockHeaders(ctx, code, data); err != nil {
				m.handleSessionError(ctx, err)
				return
			}
		case GetBlockBodiesCode:
			if err := m.handleGetBlockBodies(ctx, code, data); err != nil {
				m.handleSessionError(ctx, err)
				return
			}
		case NewPooledTransactionHashesCode:
			if err := m.handleNewPooledTransactionHashes(ctx, code, data); err != nil {
				m.handleSessionError(ctx, err)
				return
			}
		case PooledTransactionsCode:
			if err := m.handlePooledTransactions(ctx, code, data); err != nil {
				m.handleSessionError(ctx, err)
				return
			}
		case GetReceiptsCode:
			if err := m.handleGetReceipts(ctx, code, data); err != nil {
				m.handleSessionError(ctx, err)
				return
			}
		default:
			m.log.WithField("code", code).Debug("received unhandled message code")
		}
	}
}
