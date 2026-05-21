package network

import (
	"strings"
	"time"

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
	serverID = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Help:      "network server ID",
			Name:      "server_id",
			Namespace: "neogo",
		},
		[]string{"server_id"})

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
	p2pCmds     = make(map[CommandType]struct{})
	p2pCmdsTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Help:      "P2P command handling time",
			Name:      "p2p_commands_time",
			Namespace: "neogo",
		},
		[]string{"command"},
	)

	// notarypoolUnsortedTx prometheus metric.
	notarypoolUnsortedTx = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Help:      "Notary request pool fallback txs",
			Name:      "notarypool_unsorted_tx",
			Namespace: "neogo",
		},
	)
)

func init() {
	prometheus.MustRegister(
		estimatedNetworkSize,
		peersConnected,
		serverID,
		poolCount,
		blockQueueLength,
		notarypoolUnsortedTx,
		p2pCmdsTime,
	)
	for _, cmd := range []CommandType{CMDVersion, CMDVerack, CMDGetAddr,
		CMDAddr, CMDPing, CMDPong, CMDGetHeaders, CMDHeaders, CMDGetBlocks,
		CMDMempool, CMDInv, CMDGetData, CMDGetBlockByIndex, CMDNotFound,
		CMDTX, CMDBlock, CMDExtensible, CMDP2PNotaryRequest, CMDGetMPTData,
		CMDMPTData, CMDReject, CMDFilterLoad, CMDFilterAdd, CMDFilterClear,
		CMDMerkleBlock, CMDAlert} {
		p2pCmds[cmd] = struct{}{}
	}
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

func setSeverID(id string) {
	serverID.WithLabelValues(id).Add(1)
}

func addCmdTimeMetric(cmd CommandType, t time.Duration) {
	// Shouldn't happen, message decoder checks the type, but better safe than sorry.
	if _, ok := p2pCmds[cmd]; !ok {
		return
	}
	p2pCmdsTime.WithLabelValues(strings.ToLower(cmd.String())).Observe(t.Seconds())
}

// updateNotarypoolMetrics updates metric of the number of fallback txs inside
// the notary request pool.
func updateNotarypoolMetrics(unsortedTxnLen int) {
	notarypoolUnsortedTx.Set(float64(unsortedTxnLen))
}
