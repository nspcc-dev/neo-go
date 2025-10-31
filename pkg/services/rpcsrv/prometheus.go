package rpcsrv

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics used in monitoring service.
var (
	rpcTimes    = map[string]prometheus.Histogram{}
	rpcCounters = map[string]prometheus.Counter{}
)

func addReqTimeMetric(name string, t time.Duration) {
	hist, ok := rpcTimes[name]
	if ok {
		hist.Observe(t.Seconds())
	}
	cnt, ok := rpcCounters[name]
	if ok {
		cnt.Inc()
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
	rpcCounters[call] = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "RPC " + call + " call count",
			Name:      "rpc_" + strings.ToLower(call) + "_counter",
			Namespace: "neogo",
		},
	)
	prometheus.MustRegister(rpcCounters[call])
}

func init() {
	for call := range rpcHandlers {
		regCounter(call)
	}
	for call := range rpcWsHandlers {
		regCounter(call)
	}
}
