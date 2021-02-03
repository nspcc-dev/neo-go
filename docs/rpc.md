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
| `getapplicationlog` |
| `getbestblockhash` |
| `getblock` |
| `getblockcount` |
| `getblockhash` |
| `getblockheader` |
| `getblocksysfee` |
| `getconnectioncount` |
| `getcontractstate` |
| `getnep17balances` |
| `getnep17transfers` |
| `getpeers` |
| `getrawmempool` |
| `getrawtransaction` |
| `getstorage` |
| `gettransactionheight` |
| `getunclaimedgas` |
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

It's possible to use `invokefunction` not only with contract scripthash, but also 
with contract name (for native contracts) or contract ID (for all contracts). This
feature is not supported by the C# node.

##### `getunclaimedgas`

It's possible to call this method for any address with neo-go, unlike with C#
node where it only works for addresses from opened wallet.

##### `getcontractstate`

It's possible to get non-native contract state by its ID, unlike with C# node where
it only works for native contracts.

##### `getstorage`

This method doesn't work for the Ledger contract, you can get data via regular
`getblock` and `getrawtransaction` calls.

### Unsupported methods

Methods listed down below are not going to be supported for various reasons
and we're not accepting issues related to them.

| Method  | Reason |
| ------- | ------------|
| `claimgas` | Doesn't fit neo-go wallet model, use CLI to do that |
| `dumpprivkey` | Shouldn't exist for security reasons, see `claimgas` comment also |
| `getbalance` | To be implemented |
| `getmetricblocktimestamp` | Not really useful, use other means for node monitoring |
| `getnewaddress` | See `claimgas` comment |
| `getwalletheight` | Not applicable to neo-go, see `claimgas` comment |
| `importprivkey` | Not applicable to neo-go, see `claimgas` comment |
| `listaddress` | Not applicable to neo-go, see `claimgas` comment |
| `listplugins` | neo-go doesn't have any plugins, so it makes no sense |
| `sendfrom` | Not applicable to neo-go, see `claimgas` comment |
| `sendmany` | Not applicable to neo-go, see `claimgas` comment |
| `sendtoaddress` | Not applicable to neo-go, see `claimgas` comment |

### Extensions

Some additional extensions are implemented as a part of this RPC server.

#### Limits and paging for getnep17transfers

`getnep17transfers` RPC call never returns more than 1000 results for one
request (within specified time frame). You can pass your own limit via an
additional parameter and then use paging to request the next batch of
transfers.

Example requesting 10 events for address NbTiM6h8r99kpRtb428XcsUk1TzKed2gTc
within 0-1600094189 timestamps:

```json
{ "jsonrpc": "2.0", "id": 5, "method": "getnep17transfers", "params":
["NbTiM6h8r99kpRtb428XcsUk1TzKed2gTc", 0, 1600094189, 10] }
```

Get the next 10 transfers for the same account within the same time frame:

```json
{ "jsonrpc": "2.0", "id": 5, "method": "getnep17transfers", "params":
["NbTiM6h8r99kpRtb428XcsUk1TzKed2gTc", 0, 1600094189, 10, 1] }
```

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
