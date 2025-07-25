package discovery

import (
	"context"
	"crypto/ecdsa"
	"net"
	"sync"
	"time"

	"github.com/chuckpreslar/emission"
	gcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/go-co-op/gocron/v2"
	"github.com/sirupsen/logrus"
)

// DiscV4 implements the discovery v4 protocol for finding Ethereum nodes.
type DiscV4 struct {
	log       logrus.FieldLogger
	restart   time.Duration
	bootNodes []*enode.Node
	listener  *ListenerV4
	privKey   *ecdsa.PrivateKey
	broker    *emission.Emitter
	mu        sync.Mutex
	scheduler gocron.Scheduler
	started   bool
}

// ListenerV4 manages the UDP connection and discovery protocol instance.
type ListenerV4 struct {
	conn      *net.UDPConn
	localNode *enode.LocalNode
	discovery *discover.UDPv4
	mu        sync.Mutex
}

// NewDiscV4 creates a new discovery v4 instance.
func NewDiscV4(_ context.Context, restart time.Duration, log logrus.FieldLogger) *DiscV4 {
	return &DiscV4{
		log:     log.WithField("module", "ethcore/discovery/discV5"),
		restart: restart,
		broker:  emission.NewEmitter(),
		started: false,
	}
}

// Start initializes and starts the discovery v4 protocol.
func (d *DiscV4) Start(ctx context.Context) error {
	d.mu.Lock()

	if d.started {
		d.mu.Unlock()

		return nil
	}

	d.started = true

	d.mu.Unlock()

	if err := d.startCrons(ctx); err != nil {
		return err
	}

	return nil
}

func (d *DiscV4) startListener(ctx context.Context) error {
	d.mu.Lock()
	if d.listener != nil {
		d.listener.Close()
		d.listener = nil
	}
	d.mu.Unlock()

	privKey, err := gcrypto.GenerateKey()
	if err != nil {
		return err
	}

	d.mu.Lock()
	d.privKey = privKey
	d.mu.Unlock()

	listener, err := d.startDiscovery(ctx, privKey)
	if err != nil {
		return err
	}

	d.mu.Lock()
	d.listener = listener
	d.mu.Unlock()

	go d.listenForNewNodes(ctx)

	return nil
}

// Stop gracefully shuts down the discovery v4 protocol.
func (d *DiscV4) Stop(_ context.Context) error {
	d.mu.Lock()
	listener := d.listener
	scheduler := d.scheduler
	d.started = false
	d.listener = nil
	d.scheduler = nil
	d.mu.Unlock()

	if listener != nil {
		listener.Close()
	}

	if scheduler != nil {
		if err := scheduler.Shutdown(); err != nil {
			return err
		}
	}

	return nil
}

// UpdateBootNodes updates the list of bootstrap nodes used for discovery.
func (d *DiscV4) UpdateBootNodes(bootNodes []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	bn := []*enode.Node{}

	for _, addr := range bootNodes {
		bootNode, parseErr := enode.Parse(enode.ValidSchemes, addr)
		if parseErr != nil {
			return parseErr
		}

		bn = append(bn, bootNode)
	}

	d.bootNodes = bn

	return nil
}

// OnNodeRecord registers a handler to be called when new nodes are discovered.
func (d *DiscV4) OnNodeRecord(ctx context.Context, handler func(ctx context.Context, reason *enode.Node) error) {
	d.broker.On(topicNodeRecord, func(reason *enode.Node) {
		d.handleSubscriberError(handler(ctx, reason), topicNodeRecord)
	})
}

// Close shuts down the listener and releases all resources.
func (l *ListenerV4) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.discovery != nil {
		l.discovery.Close()
	}

	if l.localNode != nil && l.localNode.Database() != nil {
		l.localNode.Database().Close()
		l.localNode = nil
	}

	if l.conn != nil {
		return l.conn.Close()
	}

	return nil
}

// GetLocalNodeID returns the local node ID in a thread-safe manner.
func (l *ListenerV4) GetLocalNodeID() *enode.ID {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.localNode == nil {
		return nil
	}

	id := l.localNode.ID()

	return &id
}

func (d *DiscV4) startCrons(ctx context.Context) error {
	d.mu.Lock()
	if d.scheduler != nil {
		d.mu.Unlock()

		return nil
	}
	d.mu.Unlock()

	c, err := gocron.NewScheduler(gocron.WithLocation(time.Local))
	if err != nil {
		return err
	}

	if _, err := c.NewJob(
		gocron.DurationJob(d.restart),
		gocron.NewTask(
			func(ctx context.Context) {
				d.mu.Lock()
				started := d.started
				d.mu.Unlock()

				if !started {
					return
				}

				if err := d.startListener(ctx); err != nil {
					d.log.WithError(err).Error("Failed to restart new node discovery")
				}
			},
			ctx,
		),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	); err != nil {
		return err
	}

	d.mu.Lock()
	d.scheduler = c
	d.mu.Unlock()

	c.Start()

	return nil
}

func (d *DiscV4) listenForNewNodes(ctx context.Context) {
	d.mu.Lock()
	listener := d.listener
	d.mu.Unlock()

	if listener == nil || listener.discovery == nil {
		return
	}

	iterator := listener.discovery.RandomNodes()
	iterator = enode.Filter(iterator, d.filterPeer)

	defer iterator.Close()

	for {
		exists := iterator.Next()
		if !exists {
			break
		}

		node := iterator.Node()

		d.publishNodeRecord(ctx, node)
	}
}

func (d *DiscV4) createListener(
	ctx context.Context,
	privKey *ecdsa.PrivateKey,
) (*ListenerV4, error) {
	listener := &ListenerV4{}

	var bindIP net.IP

	ipAddr := net.IPv4zero

	bindIP = ipAddr
	udpAddr := &net.UDPAddr{
		IP:   bindIP,
		Port: 0,
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}

	listener.conn = conn

	localNode, err := d.createLocalNode(
		ctx,
		privKey,
		ipAddr,
		0,
		0,
	)
	if err != nil {
		return nil, err
	}

	listener.localNode = localNode

	dv4Cfg := discover.Config{
		PrivateKey: privKey,
	}

	dv4Cfg.Bootnodes = []*enode.Node{}

	d.mu.Lock()
	defer d.mu.Unlock()

	dv4Cfg.Bootnodes = append(dv4Cfg.Bootnodes, d.bootNodes...)

	discovery, err := discover.ListenV4(conn, localNode, dv4Cfg)
	if err != nil {
		return nil, err
	}

	listener.discovery = discovery

	return listener, nil
}

func (d *DiscV4) createLocalNode(
	_ context.Context,
	privKey *ecdsa.PrivateKey,
	ipAddr net.IP,
	udpPort, tcpPort int,
) (*enode.LocalNode, error) {
	db, err := enode.OpenDB("")
	if err != nil {
		return nil, err
	}

	localNode := enode.NewLocalNode(db, privKey)

	ipEntry := enr.IP(ipAddr)
	udpEntry := enr.UDP(udpPort) //nolint:gosec // not concerned about overflow here.
	tcpEntry := enr.TCP(tcpPort) //nolint:gosec // not concerned about overflow here.

	localNode.Set(ipEntry)
	localNode.Set(udpEntry)
	localNode.Set(tcpEntry)
	localNode.SetFallbackIP(ipAddr)
	localNode.SetFallbackUDP(udpPort)

	return localNode, nil
}

func (d *DiscV4) startDiscovery(
	ctx context.Context,
	privKey *ecdsa.PrivateKey,
) (*ListenerV4, error) {
	listener, err := d.createListener(ctx, privKey)
	if err != nil {
		return nil, err
	}

	record := listener.discovery.Self()
	d.log.WithField("ENR", record.String()).Info("Started discovery v4")

	return listener, nil
}

func (d *DiscV4) filterPeer(node *enode.Node) bool {
	// Ignore nil node entries passed in.
	if node == nil {
		return false
	}

	// ignore nodes with no ip address stored.
	if node.IP() == nil {
		return false
	}

	// Ignore ourself
	d.mu.Lock()
	listener := d.listener
	d.mu.Unlock()

	if listener != nil {
		localID := listener.GetLocalNodeID()
		if localID != nil && node.ID() == *localID {
			return false
		}
	}

	// do not dial nodes with their tcp ports not set
	if err := node.Record().Load(enr.WithEntry("tcp", new(enr.TCP))); err != nil {
		if !enr.IsNotFound(err) {
			d.log.WithError(err).Debug("Could not retrieve tcp port")
		}

		return false
	}

	return true
}

func (d *DiscV4) publishNodeRecord(_ context.Context, record *enode.Node) {
	d.broker.Emit(topicNodeRecord, record)
}

func (d *DiscV4) handleSubscriberError(err error, topic string) {
	if err != nil {
		d.log.WithError(err).WithField("topic", topic).Error("Subscriber error")
	}
}
