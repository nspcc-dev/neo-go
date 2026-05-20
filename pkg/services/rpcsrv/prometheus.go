package rpcsrv

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics used in monitoring service.
var (
	rpcRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Help:      "RPC requests count",
			Name:      "rpc_requests_total",
			Namespace: "neogo",
		},
		[]string{"api"},
	)
	wsConnectionsCnt = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Help:      "WS connections count (local and remote clients)",
			Name:      "wsconnections_count",
			Namespace: "neogo",
		},
	)
)

func addReqMetric(name string) {
	rpcRequestsTotal.WithLabelValues(strings.ToLower(name)).Inc()
}

func init() {
	prometheus.MustRegister(rpcRequestsTotal)
	prometheus.MustRegister(wsConnectionsCnt)
}

func updateWSConnectionsCnt(cnt int) {
	wsConnectionsCnt.Set(float64(cnt))
}
