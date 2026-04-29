package stateroot

import "github.com/prometheus/client_golang/prometheus"

// stateHeight prometheus metric.
var stateHeight = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Help:      "Verified state height",
		Name:      "verified_state_height",
		Namespace: "neogo",
	},
)

func init() {
	prometheus.MustRegister(stateHeight)
}

func updateStateHeightMetric(sHeight uint32) {
	stateHeight.Set(float64(sHeight))
}
