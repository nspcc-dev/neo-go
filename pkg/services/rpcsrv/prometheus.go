package rpcsrv

import (
	"strings"
	"time"

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
	rpcRequestsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Help:      "RPC request handling time in seconds",
			Name:      "rpc_request_duration_seconds",
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
	api := strings.ToLower(name)
	rpcRequestsTotal.WithLabelValues(api).Inc()
	rpcRequestsDuration.WithLabelValues(api).Observe(t.Seconds())
}

func init() {
	prometheus.MustRegister(rpcRequestsTotal)
	prometheus.MustRegister(rpcRequestsDuration)
	prometheus.MustRegister(wsConnectionsCnt)
}

func updateWSConnectionsCnt(cnt int) {
	wsConnectionsCnt.Set(float64(cnt))
}
