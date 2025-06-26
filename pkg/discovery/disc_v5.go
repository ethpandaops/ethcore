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

const (
	topicNodeRecord = "node_record"
)

type DiscV5 struct {
	log       logrus.FieldLogger
	restart   time.Duration
	bootNodes []*enode.Node
	listener  *ListenerV5
	privKey   *ecdsa.PrivateKey
	broker    *emission.Emitter
	mu        sync.Mutex
	scheduler gocron.Scheduler
	started   bool
}

type ListenerV5 struct {
	conn      *net.UDPConn
	localNode *enode.LocalNode
	discovery *discover.UDPv5
	mu        sync.Mutex
}

func NewDiscV5(_ context.Context, restart time.Duration, log logrus.FieldLogger) *DiscV5 {
	return &DiscV5{
		log:     log.WithField("module", "ethcore/discovery/discV5"),
		restart: restart,
		broker:  emission.NewEmitter(),
		started: false,
	}
}

func (d *DiscV5) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.started {
		return nil
	}

	d.started = true

	if err := d.startCrons(ctx); err != nil {
		return err
	}

	return nil
}

func (d *DiscV5) startListener(ctx context.Context) error {
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

	d.privKey = privKey

	listener, err := d.startDiscovery(ctx, d.privKey)
	if err != nil {
		return err
	}

	d.mu.Lock()
	d.listener = listener
	d.mu.Unlock()

	go d.listenForNewNodes(ctx)

	return nil
}

func (d *DiscV5) Stop(_ context.Context) error {
	d.mu.Lock()
	listener := d.listener
	scheduler := d.scheduler
	d.started = false
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

func (d *DiscV5) startCrons(ctx context.Context) error {
	c, err := gocron.NewScheduler(gocron.WithLocation(time.Local))
	if err != nil {
		return err
	}

	if _, err := c.NewJob(
		gocron.DurationJob(d.restart),
		gocron.NewTask(
			func(ctx context.Context) {
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

func (d *DiscV5) UpdateBootNodes(bootNodes []string) error {
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

func (d *DiscV5) OnNodeRecord(ctx context.Context, handler func(ctx context.Context, reason *enode.Node) error) {
	d.broker.On(topicNodeRecord, func(reason *enode.Node) {
		d.handleSubscriberError(handler(ctx, reason), topicNodeRecord)
	})
}

func (d *DiscV5) listenForNewNodes(ctx context.Context) {
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

func (d *DiscV5) createListener(
	ctx context.Context,
	privKey *ecdsa.PrivateKey,
) (*ListenerV5, error) {
	listener := &ListenerV5{}

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

	dv5Cfg := discover.Config{
		PrivateKey: privKey,
	}

	dv5Cfg.Bootnodes = []*enode.Node{}

	d.mu.Lock()
	defer d.mu.Unlock()

	dv5Cfg.Bootnodes = append(dv5Cfg.Bootnodes, d.bootNodes...)

	discovery, err := discover.ListenV5(conn, localNode, dv5Cfg)
	if err != nil {
		return nil, err
	}

	listener.discovery = discovery

	return listener, nil
}

func (d *DiscV5) createLocalNode(
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

func (d *DiscV5) startDiscovery(
	ctx context.Context,
	privKey *ecdsa.PrivateKey,
) (*ListenerV5, error) {
	listener, err := d.createListener(ctx, privKey)
	if err != nil {
		return nil, err
	}

	record := listener.discovery.Self()
	d.log.WithField("ENR", record.String()).Info("Started discovery v5")

	return listener, nil
}

func (d *DiscV5) filterPeer(node *enode.Node) bool {
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

	if listener != nil && listener.localNode != nil && node.ID() == listener.localNode.ID() {
		return false
	}

	// do not dial nodes with their tcp ports not set
	if err := node.Record().Load(enr.WithEntry("tcp", new(enr.TCP))); err != nil {
		if !enr.IsNotFound(err) {
			d.log.WithError(err).Debug("Could not retrieve tcp port")
		}

		return false
	}

	// Do not bother if the node is private
	if node.IP().IsPrivate() {
		return false
	}

	return true
}

func (d *DiscV5) publishNodeRecord(_ context.Context, record *enode.Node) {
	d.broker.Emit(topicNodeRecord, record)
}

func (d *DiscV5) handleSubscriberError(err error, topic string) {
	if err != nil {
		d.log.WithError(err).WithField("topic", topic).Error("Subscriber error")
	}
}

func (l *ListenerV5) Close() error {
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
