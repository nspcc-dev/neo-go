package rpcsrv

import (
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics used in monitoring service.
var (
	rpcCounter = map[string]prometheus.Counter{}
	rpcTimes   = map[string]prometheus.Histogram{}
)

func addReqTimeMetric(name string, t time.Duration) {
	hist, ok := rpcTimes[name]
	if ok {
		hist.Observe(t.Seconds())
	}
	ctr, ok := rpcCounter[name]
	if ok {
		ctr.Inc()
	}
}

func regCounter(call string) {
	ctr := prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      fmt.Sprintf("Number of calls to %s rpc endpoint (obsolete, to be removed)", call),
			Name:      fmt.Sprintf("%s_called", call),
			Namespace: "neogo",
		},
	)
	prometheus.MustRegister(ctr)
	rpcCounter[call] = ctr
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
}
