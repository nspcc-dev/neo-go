package mempool

import "github.com/prometheus/client_golang/prometheus"

var (
	//mempoolUnsortedTx prometheus metric.
	mempoolUnsortedTx = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Help:      "Mempool Unsorted TXs",
			Name:      "mempool_unsorted_tx",
			Namespace: "neogo",
		},
	)
	//mempoolUnverifiedTx prometheus metric.
	mempoolUnverifiedTx = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Help:      "Mempool Unverified TXs",
			Name:      "mempool_unverified_tx",
			Namespace: "neogo",
		},
	)
)

func init() {
	prometheus.MustRegister(
		mempoolUnsortedTx,
		mempoolUnverifiedTx,
	)
}

func updateMempoolMetrics(unsortedTxnLen int, unverifiedTxnLen int) {
	mempoolUnsortedTx.Set(float64(unsortedTxnLen))
	mempoolUnverifiedTx.Set(float64(unverifiedTxnLen))
}
