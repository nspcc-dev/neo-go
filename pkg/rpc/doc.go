/*
Package rpc implements NEO-specific JSON-RPC 2.0 server.
This package is currently in alpha and is subject to change.

Server

The server is written to support as much of the JSON-RPC 2.0 Spec as possible.
The server is run as part of the node currently.

TODO:
	Implement HTTPS server.
	Add remaining methods (Documented below).
	Add Swagger spec and test using dredd in circleCI.

Example call

An example would be viewing the version of the node:

	$ curl -X POST -d '{"jsonrpc": "2.0", "method": "getversion", "params": [], "id": 1}' http://localhost:20332

which would yield the response:

	{
	  "jsonrpc" : "2.0",
	    "id" : 1,
	    "result" : {
	      "port" : 20333,
	      "useragent" : "/NEO-GO:0.36.0-dev/",
	      "nonce" : 9318417
	    }
	}

Unsupported methods

	getblocksysfee
	getcontractstate (needs to be implemented in pkg/core/blockchain.go)
	getrawmempool (needs to be implemented on in pkg/network/server.go)
	getrawtransaction (needs to be implemented in pkg/core/blockchain.go)
	getstorage (lacks VM functionality)
	gettxout (needs to be implemented in pkg/core/blockchain.go)
	invoke (lacks VM functionality)
	invokefunction (lacks VM functionality)
	invokescript (lacks VM functionality)
	sendrawtransaction (needs to be implemented in pkg/core/blockchain.go)
	submitblock (needs to be implemented in pkg/core/blockchain.go)

*/
package rpc
