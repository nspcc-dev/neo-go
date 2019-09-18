/*
Package rpc implements NEO-specific JSON-RPC 2.0 client and server.
This package is currently in alpha and is subject to change.

Client

After creating a client instance with or without a ClientConfig
you can interact with the NEO blockchain by its exposed methods.

Some of the methods also allow to pass a verbose bool. This will
return a more pretty printed response from the server instead of
a raw hex string.

An example:
  endpoint := "http://seed5.bridgeprotocol.io:10332"
  opts := rpc.ClientOptions{}

  client, err := rpc.NewClient(context.TODO(), endpoint, opts)
  if err != nil {
	  log.Fatal(err)
  }

  if err := client.Ping(); err != nil {
	  log.Fatal(err)
  }

  resp, err := client.GetAccountState("ATySFJAbLW7QHsZGHScLhxq6EyNBxx3eFP")
  if err != nil {
	  log.Fatal(err)
  }
  log.Println(resp.Result.ScriptHash)
  log.Println(resp.Result.Balances)

TODO:
	Merge structs so can be used by both server and client.
	Add missing methods to client.
	Allow client to connect using client cert.
	More in-depth examples.

Supported methods

	getblock
	getaccountstate
	invokescript
	invokefunction
	sendrawtransaction
	invoke
	getrawtransaction

Unsupported methods

	validateaddress
	getblocksysfee
	getcontractstate
	getrawmempool
	getstorage
	submitblock
	gettxout
	getassetstate
	getpeers
	getversion
	getconnectioncount
	getblockhash
	getblockcount
	getbestblockhash

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
