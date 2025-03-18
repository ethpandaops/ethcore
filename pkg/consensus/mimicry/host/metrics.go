package host

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	Peers                *prometheus.GaugeVec
	PeerConnectsTotal    prometheus.Counter
	PeerDisconnectsTotal prometheus.Counter
}

func NewMetrics() *Metrics {
	m := &Metrics{
		Peers: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "libp2p_peers",
			Help: "Total number of peers connected",
		}, []string{"direction", "state"}),
		PeerConnectsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "peer_connects_success_total",
			Help: "Total number of successful peer connections",
		}),
		PeerDisconnectsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "peer_disconnects_success_total",
			Help: "Total number of successful peer disconnections",
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
