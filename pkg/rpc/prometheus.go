package rpc

import "github.com/prometheus/client_golang/prometheus"

// Metrics used in monitoring service.
var (
	getbestblockhashCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getbestblockhash rpc endpoint",
			Name:      "getbestblockhash_called",
			Namespace: "neogo",
		},
	)

	getbestblockCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getbestblock rpc endpoint",
			Name:      "getbestblock_called",
			Namespace: "neogo",
		},
	)

	getblockcountCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getblockcount rpc endpoint",
			Name:      "getblockcount_called",
			Namespace: "neogo",
		},
	)

	getblockHashCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getblockhash rpc endpoint",
			Name:      "getblockhash_called",
			Namespace: "neogo",
		},
	)

	getconnectioncountCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getconnectioncount rpc endpoint",
			Name:      "getconnectioncount_called",
			Namespace: "neogo",
		},
	)

	getversionCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getversion rpc endpoint",
			Name:      "getversion_called",
			Namespace: "neogo",
		},
	)

	getpeersCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getpeers rpc endpoint",
			Name:      "getpeers_called",
			Namespace: "neogo",
		},
	)

	validateaddressCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to validateaddress rpc endpoint",
			Name:      "validateaddress_called",
			Namespace: "neogo",
		},
	)

	getassetstateCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getassetstate rpc endpoint",
			Name:      "getassetstate_called",
			Namespace: "neogo",
		},
	)

	getaccountstateCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getaccountstate rpc endpoint",
			Name:      "getaccountstate_called",
			Namespace: "neogo",
		},
	)

	getrawtransactionCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getrawtransaction rpc endpoint",
			Name:      "getrawtransaction_called",
			Namespace: "neogo",
		},
	)

	sendrawtransactionCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to sendrawtransaction rpc endpoint",
			Name:      "sendrawtransaction_called",
			Namespace: "neogo",
		},
	)
)

func init() {
	prometheus.MustRegister(
		getbestblockhashCalled,
		getbestblockCalled,
		getblockcountCalled,
		getblockHashCalled,
		getconnectioncountCalled,
		getversionCalled,
		getpeersCalled,
		validateaddressCalled,
		getassetstateCalled,
		getaccountstateCalled,
		getrawtransactionCalled,
		sendrawtransactionCalled,
	)
}
