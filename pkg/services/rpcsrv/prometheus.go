package rpcsrv

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	localClientLabel  string = "local"
	remoteClientLabel string = "remote"
)

// Metrics used in monitoring service.
var (
	rpcTimes         = map[string]prometheus.Histogram{}
	wsConnectionsCnt = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Help:      "WS connections count (the number of local and remote WS clients)",
			Name:      "wsconnections_count",
			Namespace: "neogo",
		},
		[]string{"client_type"},
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

func incWSConnectionsCnt(isLocal bool) {
	var label = remoteClientLabel
	if isLocal {
		label = localClientLabel
	}
	wsConnectionsCnt.WithLabelValues(label).Inc()
}

func decWSConnectionsCnt(isLocal bool) {
	var label = remoteClientLabel
	if isLocal {
		label = localClientLabel
	}
	wsConnectionsCnt.WithLabelValues(label).Dec()
}
