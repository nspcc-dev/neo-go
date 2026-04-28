package rpcsrv

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics used in monitoring service.
var (
	rpcTimes         = map[string]prometheus.Histogram{}
	wsConnectionsCnt = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Help:      "WS connections count (local and remote clients)",
			Name:      "wsconnections_count",
			Namespace: "neogo",
		},
	)
)

func addReqTimeMetric(name string, t time.Duration) {
	hist, ok := rpcTimes[name]
	if ok {
		hist.Observe(t.Seconds())
	}
}

func regCounter(call string) {
	rpcTimes[call] = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Help:      "RPC " + call + " call handling time",
			Name:      "rpc_" + strings.ToLower(call) + "_time",
			Namespace: "neogo",
		},
	)
	prometheus.MustRegister(rpcTimes[call])
}

func init() {
	for call := range rpcHandlers {
		regCounter(call)
	}
	for call := range rpcWsHandlers {
		regCounter(call)
	}
	prometheus.MustRegister(wsConnectionsCnt)
}

func updateWSConnectionsCnt(cnt int) {
	wsConnectionsCnt.Set(float64(cnt))
}
