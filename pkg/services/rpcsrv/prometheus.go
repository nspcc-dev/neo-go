package rpcsrv

import (
	"strings"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	localClientLabel  string = "local"
	remoteClientLabel string = "remote"
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
	wsConnectionsCnt = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Help:      "WS connections count (the number of local and remote WS clients)",
			Name:      "wsconnections_count",
			Namespace: "neogo",
		},
		[]string{"client_type"},
	)
	wsNtfSubscribersCnt = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Help:      "The number of WS RPC notification subsystem subscribers",
			Name:      "rpc_notification_subscriber_cnt",
			Namespace: "neogo",
		},
		[]string{"client_address", "event_id"},
	)
)

func addReqMetric(name string, t time.Duration) {
	rpcRequests.WithLabelValues(strings.ToLower(name)).Observe(t.Seconds())
}

func init() {
	prometheus.MustRegister(rpcRequests)
	prometheus.MustRegister(wsConnectionsCnt)
	prometheus.MustRegister(wsNtfSubscribersCnt)
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

func incWSNtfSubscribersCnt(clientAddr string, typ neorpc.EventID) {
	wsNtfSubscribersCnt.WithLabelValues(clientAddr, typ.String()).Inc()
}

func decWSNtfSubscribersCnt(clientAddr string, typ neorpc.EventID) {
	wsNtfSubscribersCnt.WithLabelValues(clientAddr, typ.String()).Dec()
}

func dropWSNtfSubscriber(clientAddr string) {
	for _, id := range neorpc.EventIDs {
		wsNtfSubscribersCnt.DeleteLabelValues(clientAddr, id.String())
	}
}
