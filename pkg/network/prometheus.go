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

	// Deprecated: please, use neogoVersion and serverID instead.
	servAndNodeVersion = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Help:      "Server and Node versions",
			Name:      "serv_node_version",
			Namespace: "neogo",
		},
		[]string{"description", "value"},
	)

	neogoVersion = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Help:      "NeoGo version",
			Name:      "version",
			Namespace: "neogo",
		},
		[]string{"version"})

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
	p2pCmds = make(map[CommandType]prometheus.Histogram)

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
		servAndNodeVersion,
		neogoVersion,
		serverID,
		poolCount,
		blockQueueLength,
		notarypoolUnsortedTx,
	)
	for _, cmd := range []CommandType{CMDVersion, CMDVerack, CMDGetAddr,
		CMDAddr, CMDPing, CMDPong, CMDGetHeaders, CMDHeaders, CMDGetBlocks,
		CMDMempool, CMDInv, CMDGetData, CMDGetBlockByIndex, CMDNotFound,
		CMDTX, CMDBlock, CMDExtensible, CMDP2PNotaryRequest, CMDGetMPTData,
		CMDMPTData, CMDReject, CMDFilterLoad, CMDFilterAdd, CMDFilterClear,
		CMDMerkleBlock, CMDAlert} {
		p2pCmds[cmd] = prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Help:      "P2P " + cmd.String() + " handling time",
				Name:      "p2p_" + strings.ToLower(cmd.String()) + "_time",
				Namespace: "neogo",
			},
		)
		prometheus.MustRegister(p2pCmds[cmd])
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

// Deprecated: please, use setNeoGoVersion and setSeverID instead.
func setServerAndNodeVersions(nodeVer string, serverID string) {
	servAndNodeVersion.WithLabelValues("Node version: ", nodeVer).Add(0)
	servAndNodeVersion.WithLabelValues("Server id: ", serverID).Add(0)
}

func setNeoGoVersion(nodeVer string) {
	neogoVersion.WithLabelValues(nodeVer).Add(1)
}

func setSeverID(id string) {
	serverID.WithLabelValues(id).Add(1)
}

func addCmdTimeMetric(cmd CommandType, t time.Duration) {
	// Shouldn't happen, message decoder checks the type, but better safe than sorry.
	if p2pCmds[cmd] == nil {
		return
	}
	p2pCmds[cmd].Observe(t.Seconds())
}

// updateNotarypoolMetrics updates metric of the number of fallback txs inside
// the notary request pool.
func updateNotarypoolMetrics(unsortedTxnLen int) {
	notarypoolUnsortedTx.Set(float64(unsortedTxnLen))
}
