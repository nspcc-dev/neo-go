package rpcsrv

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics used in monitoring service.
var (
	rpcRequests = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Help:      "RPC request handling time",
			Name:      "rpc_requests_time",
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

func addReqMetric(name string, t time.Duration) {
	rpcRequests.WithLabelValues(strings.ToLower(name)).Observe(t.Seconds())
}

func init() {
	prometheus.MustRegister(rpcRequests)
	prometheus.MustRegister(wsConnectionsCnt)
}

func updateWSConnectionsCnt(cnt int) {
	wsConnectionsCnt.Set(float64(cnt))
}
