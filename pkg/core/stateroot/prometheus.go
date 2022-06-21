package stateroot

import (
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/prometheus/client_golang/prometheus"
)

// stateHeight (local and validated) prometheus metric.
var stateHeight = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Help:      "Current local or verified state height",
		Name:      "current_state_height",
		Namespace: "neogo",
	},
	[]string{"description", "value"},
)

func init() {
	prometheus.MustRegister(stateHeight)
}

func updateStateHeightMetric(sHeight uint32, sRoot util.Uint256, verified bool) {
	var label string
	if verified {
		label = "Verified stateroot"
	} else {
		label = "Local stateroot"
	}
	stateHeight.WithLabelValues(label, sRoot.StringLE()).Set(float64(sHeight))
}
