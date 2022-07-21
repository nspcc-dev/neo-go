/*
Package rpcclient implements NEO-specific JSON-RPC 2.0 client.
This package is currently in beta and is subject to change.

Client

After creating a client instance with or without a ClientConfig
you can interact with the NEO blockchain by its exposed methods.

Some of the methods also allow to pass a verbose bool. This will
return a more pretty printed response from the server instead of
a raw hex string.

TODO:
	Allow client to connect using client cert.
	More in-depth examples.

Supported methods

	calculatenetworkfee
	findstates
	getapplicationlog
	getbestblockhash
	getblock
	getblockcount
	getblockhash
	getblockheader
	getblockheadercount
	getcommittee
	getconnectioncount
	getcontractstate
	getnativecontracts
	getnep11balances
	getnep11properties
	getnep11transfers
	getnep17balances
	getnep17transfers
	getpeers
	getrawmempool
	getrawtransaction
	getstate
	getstateheight
	getstateroot
	getstorage
	gettransactionheight
	getunclaimedgas
	getnextblockvalidators
	getversion
	invokefunction
	invokescript
	invokecontractverify
	sendrawtransaction
	submitblock
	submitoracleresponse
	terminatesession
	traverseiterator
	validateaddress

Extensions:

	getblocksysfee
	submitnotaryrequest

Unsupported methods

	claimgas
	dumpprivkey
	getbalance
	getmetricblocktimestamp
	getnewaddress
	getwalletheight
	importprivkey
	listaddress
	listplugins
	sendfrom
	sendmany
	sendtoaddress

*/
package rpcclient
