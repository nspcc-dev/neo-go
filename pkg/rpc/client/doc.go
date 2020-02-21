/*
Package client implements NEO-specific JSON-RPC 2.0 client.
This package is currently in alpha and is subject to change.

Client

After creating a client instance with or without a ClientConfig
you can interact with the NEO blockchain by its exposed methods.

Some of the methods also allow to pass a verbose bool. This will
return a more pretty printed response from the server instead of
a raw hex string.

An example:
  endpoint := "http://seed5.bridgeprotocol.io:10332"
  opts := client.Options{}

  c, err := client.New(context.TODO(), endpoint, opts)
  if err != nil {
	  log.Fatal(err)
  }

  if err := c.Ping(); err != nil {
	  log.Fatal(err)
  }

  resp, err := c.GetAccountState("ATySFJAbLW7QHsZGHScLhxq6EyNBxx3eFP")
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
	getunspents
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

*/
package client
