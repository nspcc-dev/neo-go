package server

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics used in monitoring service.
var rpcCounter = map[string]prometheus.Counter{}

func incCounter(name string) {
	ctr, ok := rpcCounter[name]
	if ok {
		ctr.Inc()
	}
}

func init() {
	for call := range rpcHandlers {
		ctr := prometheus.NewCounter(
			prometheus.CounterOpts{
				Help:      fmt.Sprintf("Number of calls to %s rpc endpoint", call),
				Name:      fmt.Sprintf("%s_called", call),
				Namespace: "neogo",
			},
		)
		prometheus.MustRegister(ctr)
		rpcCounter[call] = ctr
	}
}
