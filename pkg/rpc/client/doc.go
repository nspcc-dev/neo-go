/*
Package client implements NEO-specific JSON-RPC 2.0 client.
This package is currently in alpha and is subject to change.

Client

After creating a client instance with or without a ClientConfig
you can interact with the NEO blockchain by its exposed methods.

Some of the methods also allow to pass a verbose bool. This will
return a more pretty printed response from the server instead of
a raw hex string.

TODO:
	Add missing methods to client.
	Allow client to connect using client cert.
	More in-depth examples.

Supported methods

	getaccountstate
	getalltransfertx
	getapplicationlog
	getassetstate
	getbestblockhash
	getblock
	getblockcount
	getblockhash
	getblockheader
	getblocksysfee
	getclaimable
	getconnectioncount
	getcontractstate
	getnep5balances
	getnep5transfers
	getpeers
	getrawmempool
	getrawtransaction
	getstorage
	gettransactionheight
	gettxout
	getunclaimed
	getunspents
	getutxotransfers
	getvalidators
	getversion
	invoke
	invokefunction
	invokescript
	sendrawtransaction
	submitblock
	validateaddress

Unsupported methods

	claimgas
	dumpprivkey
	getbalance
	getmetricblocktimestamp
	getnewaddress
	getunclaimedgas
	getwalletheight
	importprivkey
	listaddress
	listplugins
	sendfrom
	sendmany
	sendtoaddress

*/
package client
