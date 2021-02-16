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
| `getclaimable` |
| `getconnectioncount` |
| `getcontractstate` |
| `getminimumnetworkfee` |
| `getnep5balances` |
| `getnep5transfers` |
| `getpeers` |
| `getproof` |
| `getrawmempool` |
| `getrawtransaction` |
| `getstateheight` |
| `getstateroot` |
| `getstorage` |
| `gettransactionheight` |
| `gettxout` |
| `getunclaimed` |
| `getunspents` |
| `getutxotransfers` |
| `getvalidators` |
| `getversion` |
| `invoke` |
| `invokefunction` |
| `invokescript` |
| `sendrawtransaction` |
| `submitblock` |
| `validateaddress` |
| `verifyproof` |

#### Implementation notices

##### `getaccountstate`

The order of assets in `balances` section may differ from the one returned by
C# implementation. Assets can still be identified by their hashes so it
shouldn't be an issue.

##### `getapplicationlog`

Error handling for incorrect stack items differs with C# implementation. C#
implementation substitutes `stack` and `state` arrays with "error: recursive
reference" string if there are any invalid items. NeoGo never does this, for
bad `state` items it uses byte array susbstitute with message "bad
notification: ..." (may vary depending on the problem), for incorrect `stack`
items it just omits them (still returning valid ones).

##### `getassetstate`

It returns "NEO" for NEO and "NEOGas" for GAS in the `name` field instead of
language-aware JSON structures.

##### `getblock` and `getrawtransaction`

In their verbose outputs neo-go can omit some fields with default values for
transactions, this includes:
 * zero "nonce" for Miner transactions (usually nonce is not zero)
 * zero "gas" for Invocation transactions (most of the time it is zero).

##### `getclaimable`

`claimable` array ordering differs, neo-go orders entries there by the
`end_height` field, while C# implementation orders by `txid`.

##### `getcontractstate`

C# implementation doesn't return `Payable` flag in its output, neo-go has
`is_payable` field in `properties` for that.

##### `getnep5transfers`

`received` and `sent` entries are sorted differently, C# node uses
chronological order and neo-go uses reverse chronological order (which is
important for paging support, see Extensions section down below).

##### `getrawmempool`

neo-go doesn't support boolean parameter to `getrawmempool` for unverified
transactions request because neo-go actually never stores unverified
transactions in the mempool.

##### `getunclaimed`

Numeric results are wrapped into strings in neo-go (the same way fees are
encoded) to prevent floating point rounding errors.

##### `getunspents`

neo-go uses standard "0xhash" syntax for `txid` and `asset_hash` fields
whereas C# module doesn't add "0x" prefix. The order of `balance` or `unspent`
entries can differ. neo-go returns all UTXO assets while C# module only tracks
and returns NEO and GAS.

##### `getutxotransfers`

`transactions` are sorted differently, C# node uses chronological order and
neo-go uses reverse chronological order (which is important for paging
support, see Extensions section down below).


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

#### Limits and paging for getnep5transfers and getutxotransfers

Both `getnep5transfers` and `getutxotransfers` RPC calls never return more than
1000 results for one request (within specified time frame). You can pass your
own limit via an additional parameter and then use paging to request the next
batch of transfers.

Example requesting 10 events for address AYC7wn4xb8SEeYpgPXHHjLr3gBuWbgAC3Q
within 0-1600094189 timestamps:

```json
{ "jsonrpc": "2.0", "id": 5, "method": "getnep5transfers", "params":
["AYC7wn4xb8SEeYpgPXHHjLr3gBuWbgAC3Q", 0, 1600094189, 10] }
```

Get the next 10 transfers for the same account within the same time frame:

```json
{ "jsonrpc": "2.0", "id": 5, "method": "getnep5transfers", "params":
["AYC7wn4xb8SEeYpgPXHHjLr3gBuWbgAC3Q", 0, 1600094189, 10, 1] }
```

#### getalltransfertx call

In addition to regular `getnep5transfers` and `getutxotransfers` RPC calls
`getalltransfertx` is provided to return both NEP5 and UTXO events for account
in a single stream of events. These events are grouped by transaction and an
additional metadata like fees is provided. It has the same parameters as
`getnep5transfers`, but limits and paging is applied to transactions instead
of transfer events. UTXO inputs and outputs are provided by `elements` array,
while NEP5 transfer events are contained in `events` array.

Example request:

```json
{ "jsonrpc": "2.0", "id": 5, "method": "getalltransfertx", "params":
["AYC7wn4xb8SEeYpgPXHHjLr3gBuWbgAC3Q", 0, 1600094189, 2] }

```

Reply:

```json
{
   "jsonrpc" : "2.0",
   "result" : [
      {
         "txid" : "0x1cb7e089bb52cabb35c480de9d99c41c6fea7f5a276b41d71ab3fc7c470dcb74",
         "net_fee" : "0",
         "events" : [
            {
               "asset" : "3a4acd3647086e7c44398aac0349802e6a171129",
               "type" : "send",
               "address" : "ALuZLuuDssJqG2E4foANKwbLamYHuffFjg",
               "value" : "20000000000"
            }
         ],
         "sys_fee" : "0",
         "timestamp" : 1600094117,
         "block_index" : 6163114
      },
      {
         "block_index" : 6162995,
         "timestamp" : 1600092165,
         "sys_fee" : "0",
         "events" : [
            {
               "asset" : "3a4acd3647086e7c44398aac0349802e6a171129",
               "address" : "ALuZLuuDssJqG2E4foANKwbLamYHuffFjg",
               "type" : "receive",
               "value" : "20000000000"
            }
         ],
         "net_fee" : "0",
         "txid" : "0xc8b45480ade5395a4a239bb44eea6d86113f32090c4854b0c4aeee1b9485edab"
      }
   ],
   "id" : 5
}
```

Another request:

```json
{ "jsonrpc": "2.0", "id": 5, "method": "getalltransfertx", "params":
["AKJL9HwrFGdic9GTTXrdaHuNYa5oxqioRY", 0, 1600079056, 2, 13] }
```

Reply:

```json
{
   "jsonrpc" : "2.0",
   "id" : 5,
   "result" : [
      {
         "elements" : [
            {
               "asset" : "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7",
               "address" : "AZCcft1uYtmZXxzHPr5tY7L6M85zG7Dsrv",
               "value" : "0.00000831",
               "type" : "input"
            },
            {
               "value" : "0.0000083",
               "type" : "output",
               "asset" : "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7",
               "address" : "AZCcft1uYtmZXxzHPr5tY7L6M85zG7Dsrv"
            }
         ],
         "events" : [
            {
               "asset" : "1578103c13e39df15d0d29826d957e85d770d8c9",
               "address" : "AZCcft1uYtmZXxzHPr5tY7L6M85zG7Dsrv",
               "type" : "receive",
               "value" : "2380844141430"
            }
         ],
         "timestamp" : 1561566911,
         "net_fee" : "0.00000001",
         "block_index" : 3929554,
         "sys_fee" : "0",
         "txid" : "0xb4f1bdb466d8bd3524502008a0bc1f9342356b4eea67be19d384845c670442a6"
      },
      {
         "txid" : "0xc045c0612b34218b7e5eaee973114af3eff925f859adf23cf953930f667cdc93",
         "sys_fee" : "0",
         "block_index" : 3929523,
         "net_fee" : "0.00000001",
         "timestamp" : 1561566300,
         "events" : [
            {
               "asset" : "1578103c13e39df15d0d29826d957e85d770d8c9",
               "address" : "AZCcft1uYtmZXxzHPr5tY7L6M85zG7Dsrv",
               "type" : "receive",
               "value" : "2100000000"
            }
         ],
         "elements" : [
            {
               "asset" : "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7",
               "address" : "AZCcft1uYtmZXxzHPr5tY7L6M85zG7Dsrv",
               "type" : "input",
               "value" : "0.00000838"
            },
            {
               "value" : "0.00000837",
               "type" : "output",
               "address" : "AZCcft1uYtmZXxzHPr5tY7L6M85zG7Dsrv",
               "asset" : "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"
            }
         ]
      }
   ]
}
```

#### getblocktransfertx call

`getblocktransfertx` provides a list of transactions that did some asset
transfers in a block (either UTXO or NEP5). It gets a block number or hash as
a single parameter and its output format is similar to `getalltransfertx`
except for `events` where it doesn't use `address` and `type` fields, but
rather provides `from` and `to` (meaning that the asset was moved from `from`
to `to` address).

Example request:

```json
{ "jsonrpc": "2.0", "id": 5, "method": "getblocktransfertx", "params": [6000003]}

```

Reply:
```json
{
   "id" : 5,
   "result" : [
      {
         "txid" : "0xaec0994211e5d7fd459a4445b113db0102ac79cb90a08b3211b9a9190a6feaa3",
         "elements" : [
            {
               "asset" : "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7",
               "type" : "output",
               "value" : "0.19479178",
               "address" : "AHwyehUHV8ujVJBN6Tz3jBDuPAHQ1wKU5R"
            }
         ],
         "block_index" : 6000003,
         "timestamp" : 1597295221,
         "sys_fee" : "0",
         "net_fee" : "0"
      },
      {
         "sys_fee" : "0",
         "net_fee" : "0",
         "elements" : [
            {
               "value" : "971",
               "address" : "AHFvPbmMbxnD6EQQWcope8VWKEMDtG1qTQ",
               "asset" : "c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b",
               "type" : "input"
            },
            {
               "address" : "AP18zgg58bK6vZ7MX51XfD63eEEuqKCgJt",
               "value" : "971",
               "asset" : "c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b",
               "type" : "output"
            }
         ],
         "block_index" : 6000003,
         "txid" : "0x6b0888b10b1150d301f749d56b7365b307d814cfd843bd064e68313bb30c9351",
         "timestamp" : 1597295221
      },
      {
         "sys_fee" : "0",
         "net_fee" : "0",
         "block_index" : 6000003,
         "txid" : "0x6b2220834059710aecfe4b2cbdb56311bbb27ac5d94795c041b5a2e6fb76f96e",
         "timestamp" : 1597295221,
         "events" : [
            {
               "from" : "AeNAPrVp7ZWtYLaAWvZ3gkKQsJBZUJJz3r",
               "asset" : "b951ecbbc5fe37a9c280a76cb0ce0014827294cf",
               "to" : "AVkhaHaxLaboUVFD1Rke5abTJuKAqziCkY",
               "value" : "69061428"
            }
         ]
      }
   ],
   "jsonrpc" : "2.0"
}
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
