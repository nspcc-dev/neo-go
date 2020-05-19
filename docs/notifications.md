# Notification subsystem

Original motivation, requirements and general solution strategy are described
in the issue #895.

This extension allows a websocket client to subscribe to various events and
receive them as JSON-RPC notifications from the server.

## Events
Currently supported events:
 * new block added
   Contents: block.
   Filters: none.
 * new transaction in the block
   Contents: transaction.
   Filters: type.
 * notification generated during execution
   Contents: container hash, contract script hash, stack item.
   Filters: contract script hash.
 * transaction executed
   Contents: application execution result.
   Filters: VM state.

## Ordering and persistence guarantees
 * new block is only announced after its processing is complete and the chain
   is updated to the new height
 * no disk-level persistence guarantees are given
 * new in-block transaction is announced after block processing, but before
   announcing the block itself
 * transaction notifications are only announced for successful transactions
 * all announcements are being done in the same order they happen on the chain
   At first transaction execution is announced, then followed by notifications
   generated during this execution, then followed by transaction announcement.
   Transaction announcements are ordered the same way they're in the block.
 * unsubscription may not cancel pending, but not yet sent events

## Subscription management

Errors are not described down below, but they can be returned as standard
JSON-RPC errors (most often caused by invalid parameters).

### `subscribe` method

Parameters: event stream name, stream-specific filter rules hash (can be
omitted if empty).

Recognized stream names:
 * `block_added`
   No filter parameters defined.
 * `transaction_added`
   Filter: `type` as a string containing standard transaction types
   (MinerTransaction, InvocationTransaction, etc)
 * `notification_from_execution`
   Filter: `contract` field containing string with hex-encoded Uint160 (LE
   representation).
 * `transaction_executed`
   Filter: `state` field containing `HALT` or `FAULT` string for successful
   and failed executions respectively.

Response: returns subscription ID (string) as a result. This ID can be used to
cancel this subscription and has no meaning other than that.

Example request (subscribe to notifications from contract
0x6293a440ed80a427038e175a507d3def1e04fb67 generated when executing
transactions):

```
{
  "jsonrpc": "2.0",
  "method": "subscribe",
  "params": ["notification_from_execution", {"contract": "6293a440ed80a427038e175a507d3def1e04fb67"}],
  "id": 1
}

```

Example response:

```
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": "55aaff00"
}
```

### `unsubscribe` method

Parameters: subscription ID as a string.

Response: boolean true.

Example request (unsubscribe from "55aaff00"):

```
{
  "jsonrpc": "2.0",
  "method": "unsubscribe",
  "params": ["55aaff00"],
  "id": 1
}
```

Example response:

```
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": true
}
```

## Events

Events are sent as JSON-RPC notifications from the server with `method` field
being used for notification names. Notification names are identical to stream
names described for `subscribe` method with one important addition for
`event_missed` which can be sent for any subscription to signify that some
events were not delivered (usually when client isn't able to keep up with
event flow).

Verbose responses for various structures like blocks and transactions are used
to simplify working with notifications on client side. Returned structures
mostly follow the one used by standard Neo RPC calls, but may have some minor
differences.

### `block_added` notification
As a first parameter (`params` section) contains block converted to JSON
structure which is similar to verbose `getblock` response but with the
following differences:
 * it doesn't have `size` field (you can calculate it client-side)
 * it doesn't have `nextblockhash` field (it's supposed to be the latest one
    anyway)
 * it doesn't have `confirmations` field (see previous)
 * transactions contained don't have `net_fee` and `sys_fee` fields

No other parameters are sent.

Example:
```
{
   "jsonrpc" : "2.0",
   "method" : "block_added",
   "params" : [
      {
         "previousblockhash" : "0x33f3e0e24542b2ec3b6420e6881c31f6460a39a4e733d88f7557cbcc3b5ed560",
         "nextconsensus" : "AZ81H31DMWzbSnFDLFkzh9vHwaDLayV7fU",
         "index" : 205,
         "nonce" : "0000000000000457",
         "version" : 0,
         "tx" : [
            {
               "version" : 0,
               "attributes" : [],
               "txid" : "0xf9adfde059810f37b3d0686d67f6b29034e0c669537df7e59b40c14a0508b9ed",
               "size" : 10,
               "vin" : [],
               "type" : "MinerTransaction",
               "scripts" : [],
               "vout" : []
            },
            {
               "version" : 1,
               "txid" : "0x93670859cc8a42f6ea994869c944879678d33d7501d388f5a446a8c7de147df7",
               "attributes" : [],
               "size" : 60,
               "script" : "097465737476616c756507746573746b657952c103507574676f20ccfbd5f01d5b9633387428b8bab95a9e78c2",
               "vin" : [],
               "scripts" : [],
               "type" : "InvocationTransaction",
               "vout" : []
            }
         ],
         "time" : 1586154525,
         "hash" : "0x48fba8aebf88278818a3dc0caecb230873d1d4ce1ea8bf473634317f94a609e5",
         "script" : {
            "invocation" : "4047a444a51218ac856f1cbc629f251c7c88187910534d6ba87847c86a9a73ed4951d203fd0a87f3e65657a7259269473896841f65c0a0c8efc79d270d917f4ff640435ee2f073c94a02f0276dfe4465037475e44e1c34c0decb87ec9c2f43edf688059fc4366a41c673d72ba772b4782c39e79f01cb981247353216d52d2df1651140527eb0dfd80a800fdd7ac8fbe68fc9366db2d71655d8ba235525a97a69a7181b1e069b82091be711c25e504a17c3c55eee6e76e6af13cb488fbe35d5c5d025c34041f39a02ebe9bb08be0e4aaa890f447dc9453209bbfb4705d8f2d869c2b55ee2d41dbec2ee476a059d77fb7c26400284328d05aece5f3168b48f1db1c6f7be0b",
            "verification" : "532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae"
         },
         "merkleroot" : "0x9d922c5cfd4c8cd1da7a6b2265061998dc438bd0dea7145192e2858155e6c57a"
      }
   ]
}
```

### `transaction_added` notification

In the first parameter (`params` section) contains transaction converted to
JSON which is similar to verbose `getrawtransaction` response, but with the
following differences:
 * `net_fee` and `sys_fee` fields are always missing
 * block's metadata is missing (`blockhash`, `confirmations`, `blocktime`)

No other parameters are sent.

Example:
```
{
   "params" : [
      {
         "vin" : [],
         "scripts" : [],
         "attributes" : [],
         "txid" : "0x93670859cc8a42f6ea994869c944879678d33d7501d388f5a446a8c7de147df7",
         "size" : 60,
         "vout" : [],
         "type" : "InvocationTransaction",
         "version" : 1,
         "script" : "097465737476616c756507746573746b657952c103507574676f20ccfbd5f01d5b9633387428b8bab95a9e78c2"
      }
   ],
   "method" : "transaction_added",
   "jsonrpc" : "2.0"
}
```

### `notification_from_execution` notification

Contains three parameters: container hash (hex-encoded LE Uint256 in a
string), contract script hash (hex-encoded LE Uint160 in a string) and stack
item (encoded the same way as `state` field contents for notifications from
`getapplicationlog` response).

Example:

```
{
   "method" : "notification_from_execution",
   "jsonrpc" : "2.0",
   "params" : [
      {
         "state" : {
            "value" : [
               {
                  "value" : "636f6e74726163742063616c6c",
                  "type" : "ByteArray"
               },
               {
                  "value" : "507574",
                  "type" : "ByteArray"
               },
               {
                  "value" : [
                     {
                        "type" : "ByteArray",
                        "value" : "746573746b6579"
                     },
                     {
                        "value" : "7465737476616c7565",
                        "type" : "ByteArray"
                     }
                  ],
                  "type" : "Array"
               }
            ],
            "type" : "Array"
         },
         "contract" : "0xc2789e5ab9bab828743833965b1df0d5fbcc206f"
      }
   ]
}
```

### `transaction_executed` notification

Contains the same result as from `getapplicationlog` method in the first
parameter and no other parameters. One difference from `getapplicationlog` is
that it always contains zero in the `contract` field.

Example:
```
{
   "method" : "transaction_executed",
   "params" : [
      {
         "executions" : [
            {
               "vmstate" : "HALT",
               "contract" : "0x0000000000000000000000000000000000000000",
               "notifications" : [
                  {
                     "state" : {
                        "type" : "Array",
                        "value" : [
                           {
                              "type" : "ByteArray",
                              "value" : "636f6e74726163742063616c6c"
                           },
                           {
                              "type" : "ByteArray",
                              "value" : "507574"
                           },
                           {
                              "value" : [
                                 {
                                    "value" : "746573746b6579",
                                    "type" : "ByteArray"
                                 },
                                 {
                                    "type" : "ByteArray",
                                    "value" : "7465737476616c7565"
                                 }
                              ],
                              "type" : "Array"
                           }
                        ]
                     },
                     "contract" : "0xc2789e5ab9bab828743833965b1df0d5fbcc206f"
                  }
               ],
               "gas_consumed" : "1.048",
               "stack" : [
                  {
                     "type" : "Integer",
                     "value" : "1"
                  }
               ],
               "trigger" : "Application"
            }
         ],
         "txid" : "0x93670859cc8a42f6ea994869c944879678d33d7501d388f5a446a8c7de147df7"
      }
   ],
   "jsonrpc" : "2.0"
}
```

### `event_missed` notification

Never has any parameters. Example:

```
{
  "jsonrpc": "2.0",
  "method": "event_missed",
  "params": []
}
```
