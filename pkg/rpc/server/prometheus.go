package server

import "github.com/prometheus/client_golang/prometheus"

// Metrics used in monitoring service.
var (
	getapplicationlogCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getapplicationlog rpc endpoint",
			Name:      "getapplicationlog_called",
			Namespace: "neogo",
		},
	)
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

	getblockheaderCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getblockheader rpc endpoint",
			Name:      "getblockheader_called",
			Namespace: "neogo",
		},
	)

	getblocksysfeeCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getblocksysfee rpc endpoint",
			Name:      "getblocksysfee_called",
			Namespace: "neogo",
		},
	)

	getclaimableCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getclaimable rpc endpoint",
			Name:      "getclaimable_called",
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

	getcontractstateCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getcontractstate rpc endpoint",
			Name:      "getcontractstate_called",
			Namespace: "neogo",
		},
	)

	getnep5balancesCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getnep5balances rpc endpoint",
			Name:      "getnep5balances_called",
			Namespace: "neogo",
		},
	)

	getnep5transfersCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getnep5transfers rpc endpoint",
			Name:      "getnep5transfers_called",
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

	getrawmempoolCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getrawmempool rpc endpoint",
			Name:      "getrawmempool_called",
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

	gettxoutCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to gettxout rpc endpoint",
			Name:      "gettxout_called",
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

	getunspentsCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getunspents rpc endpoint",
			Name:      "getunspents_called",
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

	submitblockCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to submitblock rpc endpoint",
			Name:      "submitblock_called",
			Namespace: "neogo",
		},
	)

	getstorageCalled = prometheus.NewCounter(
		prometheus.CounterOpts{
			Help:      "Number of calls to getstorage rpc endpoint",
			Name:      "getstorage_called",
			Namespace: "neogo",
		},
	)
)

func init() {
	prometheus.MustRegister(
		getapplicationlogCalled,
		getbestblockhashCalled,
		getbestblockCalled,
		getblockcountCalled,
		getblockHashCalled,
		getblockheaderCalled,
		getblocksysfeeCalled,
		getconnectioncountCalled,
		getcontractstateCalled,
		getversionCalled,
		getpeersCalled,
		getrawmempoolCalled,
		validateaddressCalled,
		getassetstateCalled,
		getaccountstateCalled,
		getunspentsCalled,
		gettxoutCalled,
		getrawtransactionCalled,
		sendrawtransactionCalled,
		submitblockCalled,
		getstorageCalled,
	)
}
