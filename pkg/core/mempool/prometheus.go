package mempool

import "github.com/prometheus/client_golang/prometheus"

var (
	// mempoolUnsortedTx prometheus metric.
	mempoolUnsortedTx = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Help:      "Mempool Unsorted TXs",
			Name:      "mempool_unsorted_tx",
			Namespace: "neogo",
		},
	)
)

func init() {
	prometheus.MustRegister(
		mempoolUnsortedTx,
	)
}

func updateMempoolMetrics(unsortedTxnLen int) {
	mempoolUnsortedTx.Set(float64(unsortedTxnLen))
}
