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
| `getassetstate` | Yes |
| `getbestblockhash` | Yes |
| `getblock` | Yes |
| `getblockcount` | Yes |
| `getblockhash` | Yes |
| `getblocksysfee` | No |
| `getconnectioncount` | Yes |
| `getcontractstate` | No |
| `getpeers` | Yes |
| `getrawmempool` | No |
| `getrawtransaction` | No |
| `getstorage` | No |
| `gettxout` | No |
| `getversion` | Yes |
| `invoke` | No |
| `invokefunction` | No |
| `invokescript` | No |
| `sendrawtransaction` | No |
| `submitblock` | No |
| `validateaddress` | Yes |

## Reference

* [JSON-RPC 2.0 Specification](http://www.jsonrpc.org/specification)
* [NEO JSON-RPC 2.0 docs](https://docs.neo.org/en-us/node/cli/apigen.html)
