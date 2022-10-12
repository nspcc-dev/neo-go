package network

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metric used in monitoring service.
var (
	estimatedNetworkSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Help:      "Estimated network size",
			Name:      "network_size",
			Namespace: "neogo",
		},
	)

	peersConnected = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Help:      "Number of connected peers",
			Name:      "peers_connected",
			Namespace: "neogo",
		},
	)

	servAndNodeVersion = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Help:      "Server and Node versions",
			Name:      "serv_node_version",
			Namespace: "neogo",
		},
		[]string{"description", "value"},
	)

	poolCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Help:      "Number of available node addresses",
			Name:      "pool_count",
			Namespace: "neogo",
		},
	)

	blockQueueLength = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Help:      "Block queue length",
			Name:      "block_queue_length",
			Namespace: "neogo",
		},
	)
)

func init() {
	prometheus.MustRegister(
		estimatedNetworkSize,
		peersConnected,
		servAndNodeVersion,
		poolCount,
		blockQueueLength,
	)
}

func updateNetworkSizeMetric(sz int) {
	estimatedNetworkSize.Set(float64(sz))
}

func updateBlockQueueLenMetric(bqLen int) {
	blockQueueLength.Set(float64(bqLen))
}

func updatePoolCountMetric(pCount int) {
	poolCount.Set(float64(pCount))
}

func updatePeersConnectedMetric(pConnected int) {
	peersConnected.Set(float64(pConnected))
}
func setServerAndNodeVersions(nodeVer string, serverID string) {
	servAndNodeVersion.WithLabelValues("Node version: ", nodeVer).Add(0)
	servAndNodeVersion.WithLabelValues("Server id: ", serverID).Add(0)
}
