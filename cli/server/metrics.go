package server

import (
	"github.com/prometheus/client_golang/prometheus"
)

var neogoVersion = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Help:      "NeoGo version",
		Name:      "version",
		Namespace: "neogo",
	},
	[]string{"version"})

func setNeoGoVersion(nodeVer string) {
	neogoVersion.WithLabelValues(nodeVer).Add(1)
}

func init() {
	prometheus.MustRegister(
		neogoVersion,
	)
}
