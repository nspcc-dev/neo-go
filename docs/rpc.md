# RPC

## Client

Client is provided as a Go package, so please refer to the
[relevant godocs page](https://godoc.org/github.com/nspcc-dev/neo-go/pkg/rpcclient).

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
  "result" : {
    "useragent" : "/NEO-GO:0.97.2/",
    "tcpport" : 10333,
    "network" : 860833102,
    "nonce" : 105745208
  },
  "jsonrpc" : "2.0",
  "id" : 1
}
```
### Supported methods

| Method  |
| ------- |
| `calculatenetworkfee` |
| `findstates` |
| `findstorage` |
| `getapplicationlog` |
| `getbestblockhash` |
| `getblock` |
| `getblockcount` |
| `getblockhash` |
| `getblockheader` |
| `getblockheadercount` |
| `getcandidates` |
| `getcommittee` |
| `getconnectioncount` |
| `getcontractstate` |
| `getnativecontracts` |
| `getnep11balances` |
| `getnep11properties` |
| `getnep11transfers` |
| `getnep17balances` |
| `getnep17transfers` |
| `getnextblockvalidators` |
| `getpeers` |
| `getproof` |
| `getrawmempool` |
| `getrawtransaction` |
| `getstate` |
| `getstateheight` |
| `getstateroot` |
| `getstorage` |
| `gettransactionheight` |
| `getunclaimedgas` |
| `getversion` |
| `invokecontractverify` |
| `invokefunction` |
| `invokescript` |
| `sendrawtransaction` |
| `submitblock` |
| `submitoracleresponse` |
| `terminatesession` |
| `traverseiterator` |
| `validateaddress` |
| `verifyproof` |

#### Implementation notices

##### JSON representation of enumerations

C# implementation contains a number of enumerations and while it outputs them
into JSON as comma-separated strings (or just strings if only one value is
allowed for this type) it accepts pure numbers for input (see #2563 for
example). NeoGo currently doesn't support this behavior. This affects the
following data types:
 * transaction attribute type
 * oracle response code
 * transaction witness scope
 * rule witness action
 * condition rule witness type
 * function call flag
 * function call parameter type
 * execution trigger type
 * stack item type

Any call that takes any of these types for input in JSON format is affected.

##### Response error codes

NeoGo currently uses a different set of error codes in comparison to C# implementation, see 
[proposal](https://github.com/neo-project/proposals/pull/156).
NeoGo retains certain deprecated error codes, which will be removed once 
all nodes adopt the new error standard.

##### `calculatenetworkfee`

NeoGo tries to cover more cases with its calculatenetworkfee implementation,
whereas C# node support only standard signature contracts and deployed
contracts that can execute `verify` successfully on incomplete (not yet signed
properly) transaction, NeoGo also works with deployed contracts that fail at
this stage and executes non-standard contracts (that can fail
too). It's ignoring the result of any verification script (since the method
calculates fee and doesn't care about transaction validity). Invocation script
is used as is when provided, but absent it the system will try to infer one
based on the `verify` method signature (pushing dummy signatures or
hashes). If signature has some types which contents can't be adequately
guessed (arrays, maps, interop, void) they're ignored. See
neo-project/neo#2805 as well.

##### `invokefunction`, `invokescript`

neo-go implementation of `invokefunction` does not return `tx`
field in the answer because that requires signing the transaction with some
key in the server, which doesn't fit the model of our node-client interactions.
If this signature is lacking, the transaction is almost useless, so there is no point
in returning it.

It's possible to use `invokefunction` not only with a contract scripthash, but also 
with a contract name (for native contracts) or a contract ID (for all contracts). This
feature is not supported by the C# node.

If iterator is present on stack after function or script invocation then, depending
on `SessionEnable` RPC-server setting, iterator either will be marshalled as iterator
ID (corresponds to `SessionEnabled: true`) or as a set of traversed iterator values
up to `DefaultMaxIteratorResultItems` packed into array (corresponds to
`SessionEnabled: false`).

##### `getcontractstate`

It's possible to get non-native contract state by its ID, unlike with C# node where
it only works for native contracts.

##### `getrawtransaction`

VM state is included into verbose response along with other transaction fields if
the transaction is already on chain.

##### `getstateroot`

This method is able to accept state root hash instead of index, unlike the C# node
where only index is accepted.

##### `getstorage`

This method doesn't work for the Ledger contract, you can get data via regular
`getblock` and `getrawtransaction` calls. This method is able to get storage of
a native contract by its name (case-insensitive), unlike the C# node where
it only possible for index or hash.

##### `getnep11balances` and `getnep17balances`
neo-go implementation of `getnep11balances` and `getnep17balances` does not
perform tracking of NEP-11 and NEP-17 balances for each account as it is done
in the C# node. Instead, a neo-go node maintains a list of standard-compliant
contracts, i.e. those contracts that have `NEP-11` or `NEP-17` declared in the
supported standards section of the manifest. Each time balances are queried,
the neo-go node asks every NEP-11/NEP-17 contract for the account balance by
invoking `balanceOf` method with the corresponding args. Invocation GAS limit
is set to be 3 GAS. All non-zero balances are included in the RPC call result.

Thus, if a token contract doesn't have proper standard declared in the list of
supported standards but emits compliant NEP-11/NEP-17 `Transfer`
notifications, the token balance won't be shown in the list of balances
returned by the neo-go node (unlike the C# node behavior). However, transfer
logs of such tokens are still available via respective `getnepXXtransfers` RPC
calls.

The behavior of the `LastUpdatedBlock` tracking for archival nodes as far as for
governing token balances matches the C# node's one. For non-archival nodes and
other NEP-11/NEP-17 tokens, if transfer's `LastUpdatedBlock` is lower than the
latest state synchronization point P the node working against,
`LastUpdatedBlock` equals P. For NEP-11 NFTs `LastUpdatedBlock` is equal for
all tokens of the same asset.

##### `getversion`

NeoGo can return additional fields in the `protocol` object depending on the
extensions enabled. Specifically that's `p2psigextensions` and
`staterootinheader` booleans and `committeehistory` and `validatorshistory`
objects (that are effectively maps from stringified integers to other
integers). These fields are only returned when corresponding settings are
enabled in the server's protocol configuration. NeoGo can return an additional
`application` object depending on the extensions enabled which contains three
fields: `saveinvocations`, `keeponlylateststate` and `removeuntraceableblocks`
booleans. These fields are only returned when corresponding settings are enabled
in the node's application configuration.

##### `getnep11transfers` and `getnep17transfers`
`transfernotifyindex` is not tracked by NeoGo, thus this field is always zero.

##### `traverseiterator` and `terminatesession`

NeoGo returns an error when it is unable to find a session or iterator, unlike 
the error-free C# response that provides a default result.

##### `verifyProof`

NeoGo can generate an error in response to an invalid proof, unlike
the error-free C# implementation.

##### `getPeers`

NeoGo extends the `getpeers` RPC call to return the user agent
(`useragent` JSON field) and last known block height
(`lastknownheight` JSON field) for each connected peer where available.
The last known block height field may be stale depending on the
PingInterval node config and the time since the last ping.
Ping behavior may also differ between node implementations.

### Unsupported methods

Methods listed below are not going to be supported for various reasons
and we're not accepting issues related to them.

| Method  | Reason |
| ------- | ------------|
| `canceltransaction` | Doesn't fit neo-go wallet model, use CLI to do that (`neo-go util canceltx`) |
| `closewallet` | Doesn't fit neo-go wallet model |
| `dumpprivkey` | Shouldn't exist for security reasons, see `closewallet` comment also |
| `getnewaddress` | See `closewallet` comment, use CLI to do that |
| `getwalletbalance` | See `closewallet` comment, use `getnep17balances` for that |
| `getwalletunclaimedgas` | See `closewallet` comment, use `getunclaimedgas` for that |
| `importprivkey` | Not applicable to neo-go, see `closewallet` comment |
| `listaddress` | Not applicable to neo-go, see `closewallet` comment |
| `listplugins` | neo-go doesn't have any plugins, so it makes no sense |
| `openwallet` | Doesn't fit neo-go wallet model |
| `sendfrom` | Not applicable to neo-go, see `openwallet` comment |
| `sendmany` | Not applicable to neo-go, see `openwallet` comment |
| `sendtoaddress` | Not applicable to neo-go, see `claimgas` comment |

### Extensions

Some additional extensions are implemented as a part of this RPC server.

#### `getblocksysfee` call

This method returns cumulative system fee for all transactions included in a
block. It can be removed in future versions, but at the moment you can use it
to see how much GAS is burned with a particular block (because system fees are
burned).

#### `getblocknotifications` call

This method returns notifications from a block organized by trigger type.
Supports filtering by contract and event name (the same filter as provided
for subscriptions to execution results, see [notifications specification](notifications.md).
The resulting JSON is an object with three (if matched) field: "onpersist",
"application" and "postpersist" containing arrays of notifications (same JSON
as used in notification service) for the respective triggers.

#### Historic calls

A set of `*historic` extension methods provide the ability of interacting with
*historical* chain state including invoking contract methods, running scripts and
retrieving contract storage items. It means that the contracts' storage state has
all its values got from MPT with the specified stateroot from past (or, which is
the same, with the stateroot of the block of the specified height). All
operations related to the contract storage will be performed using this past
contracts' storage state and using interop context (if required by the RPC
handler) with a block which is next to the block with the specified height.

Any historical RPC call needs the historical chain state to be presented in the
node storage, thus if the node keeps only latest MPT state the historical call
can not be handled properly and
[neorpc.ErrUnsupportedState](https://github.com/nspcc-dev/neo-go/blob/87e4b6beaafa3c180184cbbe88ba143378c5024c/pkg/neorpc/errors.go#L134)
is returned in this case. The historical calls only guaranteed to correctly work
on archival node that stores all MPT data. If a node keeps the number of latest
states and has the GC on (this setting corresponds to the
`RemoveUntraceableBlocks` set to `true`), then the behaviour of historical RPC
call is undefined. GC can always kick some data out of the storage while the
historical call is executing, thus keep in mind that the call can be processed
with `RemoveUntraceableBlocks` only with limitations on available data.

##### `invokecontractverifyhistoric`, `invokefunctionhistoric` and `invokescripthistoric` calls

These methods provide the ability of *historical* calls and accept block hash or
block index or stateroot hash as the first parameter and the list of parameters
that is the same as of `invokecontractverify`, `invokefunction` and
`invokescript` correspondingly. The historical call assumes that the contracts'
storage state has all its values got from MPT with the specified stateroot (or,
which is the same, with the stateroot of the block of the specified height) and
the transaction will be invoked using interop context with block which is next to
the block with the specified height. This allows to perform test invocation using
the specified past chain state. These methods may be useful for debugging
purposes.

##### `getstoragehistoric` and `findstoragehistoric` calls

These methods provide the ability of retrieving *historical* contract storage
items and accept stateroot hash as the first parameter and the list of parameters
that is the same as of `getstorage` and `findstorage` correspondingly. The
historical storage items retrieval process assume that the contracts' storage
state has all its values got from MPT with the specified stateroot. This allows
to track the contract storage scheme using the specified past chain state. These
methods may be useful for debugging purposes.

#### `invokecontainedscript` call

This method accepts transaction (serialized JSON representation), block header
(serialized JSON representation), execution trigger (string representation) and
verbose parameter (boolean value). This method invokes the script of the given
transaction in a testing VM using provided transaction as an execution container.
Header is used to mock the execution environment (in particular, to mock the
persisting block) so that interops like `System.Runtime.GetTime` and native
Ledger contract's APIs like `currentHash` or `currentIndex` work properly. Note
that native Ledger contract's APIs can retrieve data related to already persisted
blocks only. It's up to the caller to decide which fields of transaction and header
should be filled in, but at least transaction's script is required for successful
request processing. If header is not specified, then "fake" header will be
generated on the server side based on the current chain's height. If needed,
transaction's hash and size may be overridden via `transaction.NewFakeTX`
constructor to mock transaction's hash/size respectively, no hash/size check is
performed on the server side during deserialization. If trigger parameter is
specified then it will be used to run the transaction's script (`Application` is
used by default). If `Verification` trigger is used then execution GAS limit will
be restricted by the current `MaxVerificationGAS` Policy value. `OnPersist` and
`PostPersist` triggers are not supported. If verbose parameter is specified then
invocation tree data will be attached to the result (similar to `invokefunction`
behaviour, `false` is used as the default value). All parameters are positional.
Note that similar to `invokefunction`, this method performs a test invocation and
will not affect the blockchain state.

This method is designated for more precise execution environment setup during
testing invocations, so that transaction-sensitive, block-sensitive and
trigger-sensitive interops and native contract APIs work as expected during test
execution.

#### P2PNotary extensions

The following P2PNotary extensions can be used on P2P Notary enabled networks
only.

##### `getrawnotarypool` call

`getrawnotarypool` method provides the ability to retrieve the content of the 
RPC node's notary pool (a map from main transaction hashes to the corresponding
fallback transaction hashes for currently processing P2PNotaryRequest payloads).
You can use the `getrawnotarytransaction` method to iterate through
the results of `getrawnotarypool`, retrieve main/fallback transactions,
check their contents and act accordingly.

##### `getrawnotarytransaction` call

The `getrawnotarytransaction` method takes a transaction hash and aims to locate
the corresponding transaction in the P2PNotaryRequest pool. It performs
this search across all the verified main and fallback transactions.

##### `submitnotaryrequest` call

This method can be used on P2P Notary enabled networks to submit new notary
payloads to be relayed from RPC to P2P.

#### Limits and paging for getnep11transfers and getnep17transfers

`getnep11transfers` and `getnep17transfers` RPC calls never return more than
1000 results for one request (within the specified time frame). You can pass your
own limit via an additional parameter and then use paging to request the next
batch of transfers.

An example of requesting 10 events for address NbTiM6h8r99kpRtb428XcsUk1TzKed2gTc
within 0-1600094189000 timestamps:

```json
{ "jsonrpc": "2.0", "id": 5, "method": "getnep17transfers", "params":
["NbTiM6h8r99kpRtb428XcsUk1TzKed2gTc", 0, 1600094189000, 10] }
```

Get the next 10 transfers for the same account within the same time frame:

```json
{ "jsonrpc": "2.0", "id": 5, "method": "getnep17transfers", "params":
["NbTiM6h8r99kpRtb428XcsUk1TzKed2gTc", 0, 1600094189000, 10, 1] }
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

#### `applicationlog` call invocations

The `SaveInvocations` configuration setting causes the RPC server to store smart contract
invocation details as part of the application logs. This feature is specifically useful to
capture information in the absence of `System.Runtime.Notify` calls for the given smart 
contract method. Other use-cases are described in [this issue](https://github.com/neo-project/neo/issues/3386).

Example transaction on Testnet which interacts with the native PolicyContract:
```json
{
  "id": 1,
  "jsonrpc": "2.0",
  "result": {
    "txid": "0xd6fe5f61d9cb34d6324db1be42c056d02ba1f1f6cd0bd3f3c6bb24faaaeef2a9",
    "executions": [
      {
        "trigger": "Application",
        "vmstate": "HALT",
        "gasconsumed": "2028120",
        "stack": [
          {
            "type": "Any"
          }
        ],
        "notifications": [],
        "exception": null,
        "invocations": [
          {
            "hash": "0xcc5e4edd9f5f8dba8bb65734541df7a1c081c67b",
            "method": "setFeePerByte",
            "arguments": {
              "type": "Array",
              "value": [
                {
                  "type": "Integer",
                  "value": "100"
                }
              ]
            },
            "argumentscount": 1,
            "truncated": false
          }
        ]
      }
    ]
  }
}
```

For security reasons the `arguments` field data may result in `null` if the count exceeds 2048. 
In such case the `Truncated` field will be set to `true`.

The invocation records are presented in a flat structure in the order as how they were executed.
Note that invocation records for faulted transactions are kept and are present in the 
applicationlog. This behaviour differs from notifications which are omitted for faulted transactions.

#### Iterator sessions expansion

The `SessionExpansionEnabled` configuration setting enforces RPC server to expand any iterator
returned by `invoke*` calls. Specifically, up to `MaxIteratorResultItems` elements are 
returned in the initial response, and the returned iterator is marked as `Truncated` if the 
iterator contains more elements beyond the returned batch. In this case, if `SessionEnabled`
is also `true`, an iterator `ID` and session `ID` is provided allowing clients to retrieve additional elements via
`traverseiterator`. If SessionExpansionEnabled is false, the iterator behavior depends on 
`SessionEnabled`: if `SessionEnabled` is `true`, a full iterator object with an ID is returned.
If `SessionEnabled` is `false`, the iterator is expanded up to `MaxIteratorResultItems`.

## Reference

* [JSON-RPC 2.0 Specification](http://www.jsonrpc.org/specification)
* [Neo JSON-RPC 2.0 docs](https://docs.neo.org/docs/en-us/reference/rpc/latest-version/api.html)
