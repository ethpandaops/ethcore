package host

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	Peers                *prometheus.GaugeVec
	PeerConnectsTotal    prometheus.Counter
	PeerDisconnectsTotal prometheus.Counter
}

func NewMetrics(namespace string) *Metrics {
	m := &Metrics{
		Peers: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "libp2p_peers",
			Help:      "Total number of peers connected",
			Namespace: namespace,
		}, []string{"direction", "state"}),
		PeerConnectsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "peer_connects_success_total",
			Help:      "Total number of successful peer connections",
			Namespace: namespace,
		}),
		PeerDisconnectsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "peer_disconnects_success_total",
			Help:      "Total number of successful peer disconnections",
			Namespace: namespace,
		}),
	}

	prometheus.MustRegister(
		m.Peers,
		m.PeerConnectsTotal,
		m.PeerDisconnectsTotal,
	)

	return m
}

func (m *Metrics) SetPeers(value int, direction, state string) {
	m.Peers.WithLabelValues(direction, state).Set(float64(value))
}
