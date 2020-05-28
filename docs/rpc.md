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

| Method  |
| ------- |
| `getaccountstate` |
| `getapplicationlog` |
| `getassetstate` |
| `getbestblockhash` |
| `getblock` |
| `getblockcount` |
| `getblockhash` |
| `getblockheader` |
| `getblocksysfee` |
| `getconnectioncount` |
| `getcontractstate` |
| `getnep5balances` |
| `getnep5transfers` |
| `getpeers` |
| `getrawmempool` |
| `getrawtransaction` |
| `getstorage` |
| `gettransactionheight` |
| `gettxout` |
| `getunclaimed` |
| `getunspents` |
| `getvalidators` |
| `getversion` |
| `invoke` |
| `invokefunction` |
| `invokescript` |
| `sendrawtransaction` |
| `submitblock` |
| `validateaddress` |

#### Implementation notices

##### `invokefunction` and `invoke`

neo-go's implementation of `invokefunction` and `invoke` does not return `tx`
field in the answer because that requires signing the transaction with some
key in the server which doesn't fit the model of our node-client interactions.
Lacking this signature the transaction is almost useless, so there is no point
in returning it.

Both methods also don't currently support arrays in function parameters.

### Unsupported methods

Methods listed down below are not going to be supported for various reasons
and we're not accepting issues related to them.

| Method  | Reason |
| ------- | ------------|
| `claimgas` | Doesn't fit neo-go wallet model, use CLI to do that |
| `dumpprivkey` | Shouldn't exist for security reasons, see `claimgas` comment also |
| `getbalance` | Use `getaccountstate` instead, see `claimgas` comment also |
| `getmetricblocktimestamp` | Not really useful, use other means for node monitoring |
| `getnewaddress` | See `claimgas` comment |
| `getunclaimedgas` | Use `getunclaimed` instead, see `claimgas` comment also |
| `getwalletheight` | Not applicable to neo-go, see `claimgas` comment |
| `importprivkey` | Not applicable to neo-go, see `claimgas` comment |
| `listaddress` | Not applicable to neo-go, see `claimgas` comment |
| `listplugins` | neo-go doesn't have any plugins, so it makes no sense |
| `sendfrom` | Not applicable to neo-go, see `claimgas` comment |
| `sendmany` | Not applicable to neo-go, see `claimgas` comment |
| `sendtoaddress` | Not applicable to neo-go, see `claimgas` comment |

### Extensions

Some additional extensions are implemented as a part of this RPC server.

#### Websocket server

This server accepts websocket connections on `ws://$BASE_URL/ws` address. You
can use it to perform regular RPC calls over websockets (it's supposed to be a
little faster than going regular HTTP route) and you can also use it for
additional functionality provided only via websockets (like notifications).

#### Notification subsystem

Notification subsystem consists of two additional RPC methods (`subscribe` and
`unsubscribe` working only over websocket connection) that allow to subscribe
to various blockchain events (with simple event filtering) and receive them on
the client as JSON-RPC notifications. More details on that are written in the
[notifications specification](notifications.md).

## Reference

* [JSON-RPC 2.0 Specification](http://www.jsonrpc.org/specification)
* [NEO JSON-RPC 2.0 docs](https://docs.neo.org/docs/en-us/reference/rpc/latest-version/api.html)
