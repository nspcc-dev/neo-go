package consensus

import "github.com/prometheus/client_golang/prometheus"

// Metric used in monitoring service.
var consensusReset = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Help:      "Height of dBFT service restart",
		Name:      "dbft_restart_height",
		Namespace: "neogo",
	},
)

// InitializeDBFTRestartedMetric adds consensus restart metric to Prometheus.
func initializeConsensusResetMetric() {
	prometheus.MustRegister(
		consensusReset,
	)
}

func updateConsensusResetMetric(height uint32) {
	consensusReset.Set(float64(height))
}
