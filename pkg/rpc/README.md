# RPC

## What

* Structs used by `JSON-RPC` server and for interacting with a `JSON-RPC` endpoint.
* Server for running the `JSON-RPC` protocol based on port in configuration.

> This package is currently in `alpha` and is subject to change.

## Reference

* [JSON-RPC 2.0 Specification](http://www.jsonrpc.org/specification)
* [NEO JSON-RPC 2.0 docs](http://docs.neo.org/en-us/node/api.html)

## Client

You can create a new client and start interacting with any NEO node that exposes their
`JSON-RPC` endpoint. See [godocs](https://godoc.org/github.com/CityOfZion/neo-go/pkg/rpc) for example.

> Not all methods are currently supported in the client, please see table below for supported methods.

### TODO

* Merge structs so can be used by both server and client.
* Add missing methods to client.
* Allow client to connect using client cert. 

### Supported methods

| Method  | Implemented | Required to implement |
| ------- | ------------| --------------------- | 
| `getblock` | Yes | - |
| `getaccountstate` | Yes | - |
| `invokescript` | Yes | - |
| `invokefunction` | Yes | - |
| `sendrawtransaction` | Yes | - |
| `validateaddress` | No | Handler and result struct |
| `getblocksysfee` | No | Handler and result struct |
| `getcontractstate` | No | Handler and result struct |
| `getrawmempool` | No | Handler and result struct |
| `getrawtransaction` | No | Handler and result struct |
| `getstorage` | No | Handler and result struct |
| `submitblock` | No | Handler and result struct |
| `gettxout` | No | Handler and result struct |
| `invoke` | No | Handler and result struct |
| `getassetstate` | No | Handler and result struct |
| `getpeers` | No | Handler and result struct |
| `getversion` | No | Handler and result struct |
| `getconnectioncount` | No | Handler and result struct |
| `getblockhash` | No | Handler and result struct |
| `getblockcount` | No | Handler and result struct |
| `getbestblockhash` | No | Handler and result struct |

## Server

The server is written to support as much of the [JSON-RPC 2.0 Spec](http://www.jsonrpc.org/specification) as possible. The server is run as part of the node currently.

### TODO

* Implement HTTPS server.
* Add remaining methods (Documented below).
* Add Swagger spec and test using dredd in circleCI.

### Example call

An example would be viewing the version of the node:

```bash
curl -X POST -d '{"jsonrpc": "2.0", "method": "getversion", "params": [], "id": 1}" http://localhost:20332
```

which would yield the response:

```json
{
  "jsonrpc" : "2.0",
    "id" : 1,
    "result" : {
      "port" : 20333,
      "useragent" : "/NEO-GO:0.36.0-dev/",
      "nonce" : 9318417
    }
}
```

### Supported methods

| Method  | Implemented | Required to implement |
| ------- | ------------| --------------------- | 
| `getblock` | Yes | - |
| `getaccountstate` | No | Result struct & wallet functionality |
| `invokescript` | No | VM |
| `invokefunction` | No | VM |
| `sendrawtransaction` | No | Needs to be implemented in `pkg/core/blockchain.go` |
| `validateaddress` | No | Needs to be implemented in `pkg/core/blockchain.go` |
| `getblocksysfee` | No | N/A |
| `getcontractstate` | No | Needs to be implemented in `pkg/core/blockchain.go` |
| `getrawmempool` | No | Needs to be implemented on in `pkg/network/server.go` |
| `getrawtransaction` | No | Needs to be implemented in `pkg/core/blockchain.go` |
| `getstorage` | No | VM |
| `submitblock` | No | Needs to be implemented in `pkg/core/blockchain.go` |
| `gettxout` | No | Needs to be implemented in `pkg/core/blockchain.go` |
| `invoke` | No | VM |
| `getassetstate` | No | Needs to be implemented in `pkg/core/blockchain.go` |
| `getpeers` | Yes | - |
| `getversion` | Yes | - |
| `getconnectioncount` | Yes | - |
| `getblockhash` | Yes | - |
| `getblockcount` | Yes | - |
| `getbestblockhash` | Yes | - |
