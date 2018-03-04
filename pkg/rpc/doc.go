package rpc

/*
Package rpc provides interaction with a NEO node over JSON-RPC.
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

To be continued with more in depth examples.
*/
