package server

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics used in monitoring service.
var (
	rpcCalls = []string{
		"getaccountstate",
		"getapplicationlog",
		"getassetstate",
		"getbestblock",
		"getbestblockhash",
		"getblockcount",
		"getblockhash",
		"getblockheader",
		"getblocksysfee",
		"getclaimable",
		"getconnectioncount",
		"getcontractstate",
		"getnep5balances",
		"getnep5transfers",
		"getpeers",
		"getrawmempool",
		"getrawtransaction",
		"getstorage",
		"gettransactionheight",
		"gettxout",
		"getunclaimed",
		"getunspents",
		"getvalidators",
		"getversion",
		"sendrawtransaction",
		"submitblock",
		"validateaddress",
	}

	rpcCounter = map[string]prometheus.Counter{}
)

func incCounter(name string) {
	ctr, ok := rpcCounter[name]
	if ok {
		ctr.Inc()
	}
}

func init() {
	for i := range rpcCalls {
		ctr := prometheus.NewCounter(
			prometheus.CounterOpts{
				Help:      fmt.Sprintf("Number of calls to %s rpc endpoint", rpcCalls[i]),
				Name:      fmt.Sprintf("%s_called", rpcCalls[i]),
				Namespace: "neogo",
			},
		)
		prometheus.MustRegister(ctr)
		rpcCounter[rpcCalls[i]] = ctr
	}
}
