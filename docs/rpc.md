# RPC

## Client

Client is provided as a Go package, so please refer to the
[relevant godocs page](https://godoc.org/github.com/nspcc-dev/neo-go/pkg/rpc).

## Server

The server is written to support as much of the [JSON-RPC 2.0 Spec](http://www.jsonrpc.org/specification) as possible. The server is run as part of the node currently.

### Example call

An example would be viewing the version of the node:

```bash
$ curl -X POST -d '{"jsonrpc": "2.0", "method": "getversion", "params": [], "id": 1}' http://localhost:20332
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

| Method  | Implemented |
| ------- | ------------|
| `getaccountstate` | Yes |
| `getapplicationlog` | No (#500) |
| `getassetstate` | Yes |
| `getbestblockhash` | Yes |
| `getblock` | Yes |
| `getblockcount` | Yes |
| `getblockhash` | Yes |
| `getblocksysfee` | No (#341) |
| `getconnectioncount` | Yes |
| `getcontractstate` | No (#342) |
| `getnep5balances` | No (#498) |
| `getnep5transfers` | No (#498) |
| `getpeers` | Yes |
| `getrawmempool` | No (#175) |
| `getrawtransaction` | Yes |
| `getstorage` | No (#343) |
| `gettxout` | No (#345) |
| `getunspents` | Yes |
| `getversion` | Yes |
| `invoke` | Yes |
| `invokefunction` | Yes |
| `invokescript` | Yes |
| `sendrawtransaction` | Yes |
| `submitblock` | No (#344) |
| `validateaddress` | Yes |

#### Implementation notices

##### `invokefunction` and `invoke`

neo-go's implementation of `invokefunction` and `invoke` does not return `tx`
field in the answer because that requires signing the transaction with some
key in the server which doesn't fit the model of our node-client interactions.
Lacking this signature the transaction is almost useless, so there is no point
in returning it.

Both methods also don't currently support arrays in function parameters.

## Reference

* [JSON-RPC 2.0 Specification](http://www.jsonrpc.org/specification)
* [NEO JSON-RPC 2.0 docs](https://docs.neo.org/en-us/node/cli/apigen.html)
