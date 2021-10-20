# NeoGo P2P signature collection (notary) service

P2P signature (notary) service is a NeoGo node extension that allows several
parties to sign one transaction independently of chain and without going beyond the
chain environment. The on-chain P2P service is aimed to automate, accelerate and
secure the process of signature collection. The service was initially designed as
a solution for
[multisignature transaction forming](https://github.com/neo-project/neo/issues/1573#issue-600384746)
and described in the [proposal](https://github.com/neo-project/neo/issues/1573#issuecomment-704874472).

The original problem definition:
> Several parties want to sign one transaction, it can either be a set of signatures
> for multisignature signer or multiple signers in one transaction. It's assumed
> that all parties can generate the same transaction (with the same hash) without
> any interaction, which is the case for oracle nodes or NeoFS inner ring nodes.
> 
> As some of the services using this mechanism can be quite sensitive to the
> latency of their requests processing it should be possible to construct complete
> transaction within the time frame between two consecutive blocks.


## Components and functionality
The service consists of a native contract and a node module. Native contract is
mostly concerned with verification, fees and payment guarantees, while module is
doing the actual work. It uses generic `Conflicts` and `NotValidBefore`
transaction attributes for its purposes as well as an additional special one
(`Notary assisted`).

A new designated role is added, `P2PNotary`. It can have arbitrary number of
keys associated with it.

Using the service costs some GAS, so below we operate with `FEE` as a unit of cost
for this service. `FEE` is set to be 0.1 GAS.

We'll also use `NKeys` definition as the number of keys that participate in the
process of signature collection. This is the number of keys that could potentially
sign the transaction, for transactions lacking appropriate witnesses that would be
the number of witnesses, for "M out of N" multisignature scripts that's N, for
combination of K standard signature witnesses and L multisignature "M out of N"
witnesses that's K+N*L.

### Transaction attributes

#### Conflicts

This attribute makes the chain only accept one transaction of the two conflicting
and adds an ability to give a priority to any of the two if needed. This
attribute was originally proposed in
[neo-project/neo#1991](https://github.com/neo-project/neo/issues/1991).

The attribute has Uint256 data inside of it containing the hash of conflicting
transaction. It is allowed to have multiple attributes of this type.

#### NotValidBefore

This attribute makes transaction invalid before certain height. This attribute
was originally proposed in
[neo-project/neo#1992](https://github.com/neo-project/neo/issues/1992).

The attribute has uint32 data inside which is the block height starting from
which the transaction is considered to be valid. It can be seen as the opposite
of `ValidUntilBlock`, using both allows to have a window of valid block numbers
that this transaction could be accepted into. Transactions with this attribute
are not accepted into mempool before specified block is persisted.

It can be used to create some transactions in advance with a guarantee that they
won't be accepted until specified block.

#### NotaryAssisted

This attribute contains one byte containing the number of transactions collected
by the service. It could be 0 for fallback transaction or `NKeys` for normal
transaction that completed its P2P signature collection. Transactions using this
attribute need to pay an additional network fee of (`NKeys`+1)×`FEE`. This attribute
could be only be used by transactions signed by the notary native contract.

### Native Notary contract

It exposes several methods to the outside world:

| Method | Parameters | Return value | Description |
| --- | --- | --- | --- |
| `onNEP17Payment` | `from` (uint160) - GAS sender account.<br>`amount` (int) - amount of GAS to deposit.<br>`data` represents array of two parameters: <br>1. `to` (uint160) - account of the deposit owner.<br>2. `till` (int) - deposit lock height. | `bool` | Automatically called after GAS transfer to Notary native contract address and records deposited amount as belonging to `to` address with a lock till `till` chain's height. Can only be invoked from native GAS contract. Must be witnessed by `from`. `to` can be left unspecified (null), with a meaning that `to` is the same address as `from`. `amount` can't be less than 2×`FEE` for the first deposit call for the `to` address. Each successive deposit call must have `till` value equal to or more than the previous successful call (allowing for renewal), if it has additional amount of GAS it adds up to the already deposited value.|
| `lockDepositUntil` | `address` (uint160) - account of the deposit owner.<br>`till` (int) - new height deposit is valid until (can't be less than previous value). | `void` | Updates deposit expiration value. Must be witnessed by `address`. |
| `withdraw` | `from` (uint160) - account of the deposit owner.<br>`to` (uint160) - account to transfer GAS to. | `bool` | Sends all deposited GAS for `from` address to `to` address. Must be witnessed by `from`. `to` can be left unspecified (null), with a meaning that `to` is the same address as `from`. It can only be successful if the lock has already expired, attempting to withdraw the deposit before that height fails. Partial withdrawal is not supported. Returns boolean result, `true` for successful calls and `false` for failed ones. |
| `balanceOf` | `addr` (uint160) - account of the deposit owner. | `int` | Returns deposited GAS amount for specified address (integer). |
| `expirationOf` | `addr` (uint160) - account of the deposit owner. | `int` | Returns deposit lock height for specified address (integer). |
| `verify` | `signature` (signature) - notary node signature bytes for verification. | `bool` | This is used to verify transactions with notary contract specified as a signer, it needs one signature in the invocation script and it checks for this signature to be made by one of designated keys, effectively implementing "1 out of N" multisignature contract. |
| `getMaxNotValidBeforeDelta` | | `int` | Returns `MaxNotValidBeforeDelta` constraint. Default value is 140. |
| `setMaxNotValidBeforeDelta` | `value` (int) | `void` | Set `MaxNotValidBeforeDelta` constraint. Must be witnessed by committee. |

See the [Notary deposit guide](#1.-Notary-deposit) section on how to deposit
funds to Notary native contract and manage the deposit.

### P2PNotaryRequest payload

A new broadcasted payload type is introduced for notary requests. It's
distributed via regular inv-getdata mechanism like transactions, blocks or
consensus payloads. An ordinary P2P node verifies it, saves in a structure
similar to mempool and relays. This payload has witness (standard
single-signature contract) attached signing all of the payload.

This payload has two incomplete transactions inside:

- *Fallback tx*. This transaction has P2P Notary contract as a sender and service
  request sender as an additional signer. It can't have a witness for Notary
  contract, but it must have proper witness for request sender. It must have
  `NotValidBefore` attribute that is no more than `MaxNotValidBeforeDelta` higher
  than the current chain height and it must have `Conflicts` attribute with the
  hash of the main transaction. It at the same time must have `Notary assisted`
  attribute with a count of zero.
- *Main tx*. This is the one that actually needs to be completed, it:
  1. *either* doesn't have all witnesses attached
  2. *or* it only has a partial multisignature
  3. *or* have not all witnesses attached and some of the rest are partial multisignature
  
  This transaction must have `Notary assisted` attribute with a count of `NKeys`
  (and Notary contract as one of the signers).

See the [Notary request submission guide](#2-request-submission) to learn how to
construct and send the payload.

### Notary node module

Node module with the designated key monitors the network for `P2PNotaryRequest`
payloads. It maintains a list of current requests grouped by main transaction
hash, when it receives enough requests to correctly construct all transaction
witnesses it does so, adds a witness of its own (for Notary contract witness) and
sends the resulting transaction to the network.

If the main transaction with all witnesses attached still can't be validated
because of fee (or other) issues, the node waits for `NotValidBefore` block of
the fallback transaction to be persisted.

If `NotValidBefore` block is persisted and there are still some signatures
missing (or the resulting transaction is invalid), the module sends all the
associated fallback transactions for the main transaction.

After processing service request is deleted from the module.

See the [NeoGo P2P signature extensions](#NeoGo P2P signature extensions) on how
to enable notary-related extensions on chain and
[NeoGo Notary service node module](#NeoGo Notary service node module) on how to
set up Notary service node.

## Environment setup

To run P2P signature collection service on your network you need to do:
* Set up [`P2PSigExtensions`](#NeoGo P2P signature extensions) for all nodes in
  the network.
* Set notary node keys in `RoleManagement` native contract.
* [Configure](#NeoGo Notary service node module) and run appropriate number of
  notary nodes with keys specified in `RoleManagement` native contract (at least
  one node is necessary to complete signature collection).

After service is running, you can [create and send](#Notary request lifecycle guide)
notary requests to the network.

### NeoGo P2P signature extensions

As far as Notary service is an extension of the standard NeoGo node, it should be
enabled and properly configured before the usage.

#### Configuration

To enable P2P signature extensions add `P2PSigExtensions` subsection set to
`true` to `ProtocolConfiguration` section of your node config. This enables all
notary-related logic in the network, i.e. allows your node to accept and validate
`NotValidBefore`, `Conflicts` and `NotaryAssisted` transaction attribute, handle,
verify and broadcast `P2PNotaryRequest` P2P payloads, properly initialize native
Notary contract and designate `P2PNotary` node role in RoleManagement native
contract.

If you use custom `NativeActivations` subsection of the `ProtocolConfiguration`
section in your node config, then specify the height of the Notary contract
activation, e.g. `0`.

Note, that even if `P2PSigExtensions` config subsection enables notary-related
logic in the network, it still does not turn your node into notary service node.
To enable notary service node functionality refer to the
[NeoGo Notary service](#NeoGo-Notary-service-node-module) documentation.

##### Example

```
  P2PSigExtensions: true
  NativeActivations:
    Notary: [0]
    ContractManagement: [0]
    StdLib: [0]
    CryptoLib: [0]
    LedgerContract: [0]
    NeoToken: [0]
    GasToken: [0]
    PolicyContract: [0]
    RoleManagement: [0]
    OracleContract: [0]
```


### NeoGo Notary service node module

NeoGo node can act as notary service node (the node that accumulates notary
requests, collects signatures and releases fully-signed transactions). It has to
have a wallet with key belonging to one of network's designated notary nodes
(stored in `RoleManagement` native contract). Also, the node must be connected to
the network with enabled P2P signature extensions, otherwise problems with states
and peer disconnections will occur.

Notary service node doesn't need [RPC service](rpc.md) to be enabled, because it
receives notary requests and broadcasts completed transactions via P2P protocol.
However, enabling [RPC service](rpc.md) allows to send notary requests directly
to the notary service node and avoid P2P communication delays.

#### Configuration

To enable notary service node check firstly that
[P2PSignatureExtensions](#NeoGo P2P signature extensions) are properly set up.
Then add `P2PNotary` subsection to `ApplicationConfiguration` section of your
node config.

Parameters:
* `Enabled`: boolean value, enables/disables the service node, `true` for service
  node to be enabled
* `UnlockWallet`: notary node wallet configuration:
    - `Path`: path to NEP-6 wallet.
    - `Password`: password for the account to be used by notary node.

##### Example

```
P2PNotary:
  Enabled: true
  UnlockWallet:
    Path: "/notary_node_wallet.json"
    Password: "pass"
```


## Notary request lifecycle guide

Below are presented all stages each P2P signature collection request goes through. Use
stages 1 and 2 to create, sign and submit P2P notary request. Stage 3 is
performed by the notary service, does not require user's intervention and is given
for informational purposes. Stage 4 contains advice to check for notary request
results.

### 1. Notary deposit

To guarantee that payment to the notary node will still be done if things go wrong,
sender's deposit to the Notary native contract is used. Before the notary request will be
submitted, you need to deposit enough GAS to the contract, otherwise, request
won't pass verification.

Notary native contract supports `onNEP17Payment` method, thus to deposit funds to
the Notary native contract, transfer desired amount of GAS to the contract
address. Use
[func (*Client) TransferNEP17](https://pkg.go.dev/github.com/nspcc-dev/neo-go@v0.97.2/pkg/rpc/client#Client.TransferNEP17)
with the `data` parameter matching the following requirements:
- `data` should be an array of two elements: `to` and `till`.
- `to` denotes the receiver of the deposit. It can be nil in case if `to` equals
  to the GAS sender.
- `till` denotes chain's height before which deposit is locked and can't be
  withdrawn. `till` can't be set if you're not the deposit owner. Default `till`
  value is current chain height + 5760. `till` can't be less than current chain
  height. `till` can't be less than currently set `till` value for that deposit if
  the deposit already exists.

Note, that the first deposit call for the `to` address can't transfer less than 2×`FEE` GAS.
Deposit is allowed for renewal, i.e. consequent `deposit` calls for the same `to`
address add up specified amount to the already deposited value.

After GAS transfer successfully submitted to the chain, use [Notary native
contract API](#Native Notary contract) to manage your deposit.

Note, that regular operation flow requires deposited amount of GAS to be
sufficient to pay for *all* fallback transactions that are currently submitted (all
in-flight notary requests). The default deposit sum for one fallback transaction
should be enough to pay the fallback transaction fees which are system fee and
network fee. Fallback network fee includes (`NKeys`+1)×`FEE` = (0+1)×`FEE` = `FEE`
GAS for `NotaryAssisted` attribute usage and regular fee for the fallback size.
If you need to submit several notary requests, ensure that deposited amount is
enough to pay for all fallbacks. If the deposited amount is not enough to pay the
fallback fees, then `Insufficiend funds` error will be returned from the RPC node
after notary request submission.

### 2. Request submission

Once several parties want to sign one transaction, each of them should generate
the transaction, wrap it into `P2PNotaryRequest` payload and send to the known RPC
server via [`submitnotaryrequest` RPC call](./rpc.md#submitnotaryrequest-call).
Note, that all parties must generate the same main transaction, while fallbacks
can differ.

To create notary request, you can use [NeoGo RPC client](./rpc.md#Client). Follow
the steps to create a signature request:

1. Prepare list of signers with scopes for the main transaction (i.e. the
   transaction that signatures are being collected for, that will be `Signers`
   transaction field). Use the following rules to construct the list:
   * First signer is the one who pays transaction fees.
   * Each signer is either multisignature or standard signature or a contract
     signer.
   * Multisignature and signature signers can be combined.
   * Contract signer can be combined with any other signer.

   Include Notary native contract in the list of signers with the following
   constraints:
   * Notary signer hash is the hash of native Notary contract that can be fetched
     from
     [func (*Client) GetNativeContractHash](https://pkg.go.dev/github.com/nspcc-dev/neo-go@v0.97.2/pkg/rpc/client#Client.GetNativeContractHash).
   * Notary signer must have `None` scope.
   * Notary signer shouldn't be placed at the beginning of the signer list,
     because Notary contract does not pay main transaction fees. Other positions
     in the signer list are available for Notary signer.
2. Construct script for the main transaction (that will be `Script` transaction
   field) and calculate system fee using regular rules (that will be `SystemFee`
   transaction field). Probably, you'll perform one of these actions:
   1. If the script is a contract method call, use `invokefunction` RPC API
      [func (*Client) InvokeFunction](https://pkg.go.dev/github.com/nspcc-dev/neo-go@v0.97.2/pkg/rpc/client#Client.InvokeFunction)
      and fetch script and gas consumed from the result.
   2. If the script is more complicated than just a contract method call,
      construct the script manually and use `invokescript` RPC API
      [func (*Client) InvokeScript](https://pkg.go.dev/github.com/nspcc-dev/neo-go@v0.97.2/pkg/rpc/client#Client.InvokeScript)
      to fetch gas consumed from the result.
   3. Or just construct the script and set system fee manually.
3. Calculate the height main transaction is valid until (that will be
   `ValidUntilBlock` transaction field). Consider the following rules for `VUB`
   value estimation:
      * `VUB` value must not be lower than current chain height.
      * The whole notary request (including fallback transaction) is valid until
        the same `VUB` height.
      * `VUB` value must be lower than notary deposit expiration height. This
        condition guarantees that deposit won't be withdrawn before notary
        service payment.
      * All parties must provide the same `VUB` for the main transaction. 
4. Construct the list of main transaction attributes (that will be `Attributes`
   transaction field). The list must include `NotaryAssisted` attribute with
   `NKeys` equals to the sum number of keys to be collected excluding notary and
   other contract-based witnesses. For m out of n multisignature request
   `NKeys = n`. For multiple standard signature request signers `NKeys` equals to
   the standard signature signers count.
5. Construct the list of accounts (`wallet.Account` structure from the `wallet`
   package) to calculate network fee for the transaction
   using following rules. This list will be used in the next step.
   - Number and order of the accounts should match transaction signers
     constructed at step 1.
   - Account for contract signer should have `Contract` field with `Deployed` set
     to `true` if the corresponding contract is deployed on chain.
   - Account for signature or multisignature signer should have `Contract` field
     with `Deployed` set to `false` and `Script` set to the signer's verification
     script.
   - Account for notary signer is **just a placeholder** and should have
     `Contract` field with `Deployed` set to `false`, i.e. the default value for
     `Contract` field. That's needed to skip notary verification during regular
     network fee calculation at the next step.
     
7. Calculate network fee for the transaction (that will be `NetworkFee`
   transaction field). Network fee consists of several parts:
   - *Notary network fee.* That's amount of GAS need to be paid for
     `NotaryAssisted` attribute usage and for notary contract witness
     verification (that is to be added by the notary node in the end of
     signature collection process). Use
     [func (*Client) CalculateNotaryFee](https://pkg.go.dev/github.com/nspcc-dev/neo-go@v0.97.2/pkg/rpc/client#Client.CalculateNotaryFee)
     to calculate notary network fee. Use `NKeys` estimated on the step 4 as an
     argument.
   - *Regular network fee.* That's amount of GAS to be paid for other witnesses
     verification. Use
     [func (*Client) AddNetworkFee](https://pkg.go.dev/github.com/nspcc-dev/neo-go@v0.97.2/pkg/rpc/client#Client.AddNetworkFee)
     to calculate regular network fee and add it to the transaction. Use
     partially-filled main transaction from the previous steps as `tx` argument.
     Use notary network fee calculated at the previous substep as `extraFee`
     argument. Use the list of accounts constructed at the step 5 as `accs`
     argument.
8. Fill in main transaction `Nonce` field.
9. Construct the list of main transactions witnesses (that will be `Scripts`
   transaction field). Use the following rules:
   - Contract-based witness should have `Invocation` script that pushes arguments
     on stack (it may be empty) and empty `Verification` script. Currently, **only
     empty** `Invocation` scripts are supported for contract-based witnesses.
   - **Notary contract witness** (which is also a contract-based witness) should
     have empty `Verification` script. `Invocation` script should be of the form
     [opcode.PUSHDATA1, 64, make([]byte, 64)...], i.e. to be a placeholder for
     notary contract signature.
   - Standard signature witness must have regular `Verification` script filled
     even if the `Invocation` script is to be collected from other notary
     requests.
     `Invocation` script either should push signature bytes on stack **or** (in
     case if the signature is to be collected) **should be empty**.
   - Multisignature witness must have regular `Verification` script filled even
     if `Invocation` script is to be collected from other notary requests.
     `Invocation` script either should push on stack signature bytes (one
     signature at max per one resuest) **or** (in case if there's no ability to
     provide proper signature) **should be empty**.
10. Define lifetime for the fallback transaction. Let the `fallbackValidFor` be
    the lifetime. Let `N` be the current chain's height and `VUB` be
    `ValidUntilBlock` value estimated at the step 3. Then notary node is trying to
    collect signatures for the main transaction from `N` up to
    `VUB-fallbackValidFor`. In case of failure after `VUB-fallbackValidFor`-th
    block is accepted, notary node stops attempts to complete main transaction and
    tries to push all associated fallbacks. Use the following rules to define
    `fallbackValidFor`:
       - `fallbackValidFor` shouldn't be more than `MaxNotValidBeforeDelta` value.
       - Use [func (*Client) GetMaxNotValidBeforeDelta](https://pkg.go.dev/github.com/nspcc-dev/neo-go@v0.97.2/pkg/rpc/client#Client.GetMaxNotValidBeforeDelta)
         to check `MaxNotValidBefore` value.
11. Construct script for the fallback transaction. Script may do something useful,
    i.g. invoke method of a contract, but if you don't need to perform something
    special on fallback invocation, you can use simple `opcode.RET` script.
12. Sign and submit P2P notary request. Use
    [func (*Client) SignAndPushP2PNotaryRequest](https://pkg.go.dev/github.com/nspcc-dev/neo-go@v0.97.2/pkg/rpc/client#Client.SignAndPushP2PNotaryRequest) for it.
    - Use signed main transaction from step 8 as `mainTx` argument.
    - Use fallback script from step 10 as `fallbackScript` argument.
    - Use `-1` as `fallbackSysFee` argument to define system fee by test
      invocation or provide custom value.
    - Use `0` as `fallbackNetFee` argument not to add extra network fee to the
      fallback.
    - Use `fallbackValidFor` estimated at step 9 as `fallbackValidFor` argument.
    - Use your account you'd like to send request (and fallback transaction) from
      to sign the request (and fallback transaction).
    
    `SignAndPushP2PNotaryRequest` will construct and sign fallback transaction,
    construct and sign P2PNotaryRequest and submit it to the RPC node. The
    resulting notary request and an error are returned.

After P2PNotaryRequests are sent, participants should then wait for one of their
transactions (main or fallback) to get accepted into one of subsequent blocks.

### 3. Signatures collection and transaction release

Valid P2PNotaryRequest payload is distributed via P2P network using standard
broadcasting mechanisms until it reaches designated notary nodes that have the
respective node module active. They collect all payloads for the same main
transaction until enough signatures are collected to create proper witnesses for
it. They then attach all witnesses required and send this transaction as usual
and monitor subsequent blocks for its inclusion.

All the operations leading to successful transaction creation are independent
of the chain and could easily be done within one block interval, so if the
first service request is sent at current height `H` it's highly likely that the
main transaction will be a part of `H+1` block.
 
### 4. Results monitoring

Once P2PNotaryRequest reached RPC node, it is added to the notary request pool.
Completed or outdated requests are being removed from the pool. Use
[NeoGo notification subsystem](./notifications.md) to track request addition and
removal:

- Use RPC `subscribe` method with `notary_request_event` stream name parameter to
  subscribe to `P2PNotaryRequest` payloads that are added or removed from the
  notary request pool.
- Use `sender` or `signer` filters to filter out notary request with desired
  request senders or main tx signers.

Use the notification subsystem to track that main or fallback transaction
accepted to the chain:

- Use RPC `subscribe` method with `transaction_added` stream name parameter to
  subscribe to transactions that are accepted to the chain.
- Use `sender` filter with Notary native contract hash to filter out fallback
  transactions sent by Notary node. Use `signer` filter with notary request
  sender address  to filter out fallback transactions sent by the specified
  sender.
- Use `sender` or `signer` filters to filter out main transaction with desired
  sender or signers. You can also filter out main transaction using Notary
  contract `signer` filter.
- Don't rely on `sender` and `signer` filters only, check also that received
  transaction has `NotaryAssisted` attribute with expected `NKeys` value.

Use the notification subsystem to track main or fallback transaction execution
results.

Moreover, you can use all regular RPC calls to track main or fallback transaction
invocation: `getrawtransaction`, `getapplicationlog` etc.

## Notary service use-cases

Several use-cases where Notary subsystem can be applied are described below.

### Committee-signed transactions

The signature collection problem occures every time committee participants need
to submit transaction with `m out of n` multisignature, i.g.:
- transfer initial supply of NEO and GAS from committee multisignature account to
  other addresses on new chain start
- tune valuable chain parameters like gas per block, candidate register price,
  minimum contract deployment fee, Oracle request price, native Policy values etc
- invoke non-native contract methods that require committee multisignature witness

Current solution supposes off-chain non-P2P signature collection (either manual
or using some additional network connectivity). It has an obvious downside of
reliance on something external to the network. If it's manual, it's slow and
error-prone, if it's automated, it requires additional protocol for all the
parties involved. For the protocol used by oracle nodes that also means
explicitly exposing nodes to each other.

With Notary service all signature collection logic is unified and is on chain already,
the only thing that committee participants should perform is to create and submit
P2P notary request (can be done independently). Once sufficient number of signatures
is collected by the service, desired transaction will be applied and pass committee
witness verification.

### NeoFS Inner Ring nodes

Alphabet nodes of the Inner Ring signature collection is a particular case of committee-signed
transactions. Alphabet nodes multisignature is used for the various cases, such as:
- main chain and side chain funds synchronization and withdrawal
- bootstrapping new storage nodes to the network
- network map management and epoch update
- containers and extended ACL management
- side chain governance update

Non-notary on-chain solution for Alphabet nodes multisignature forming is
imitated via contracts collecting invocations of their methods signed by standard
signature of each Alphabet node. Once sufficient number of invocations is
collected, the invocation is performed.

The described solution has several drawbacks:

- it can only be app-specific (meaning that for every use case this logic would
  be duplicated) because we can't create transactions from transactions (thus
  using proper multisignature account is not possible)
- for `m out of n` multisignature we need at least `m` transactions instead of
  one we really wanted to have, but in reality we'll create and process `n` of
  them, so this adds substantial overhead to the chain
- some GAS is inevitably wasted because any invocation could either go the easy
  path (just adding a signature to the list) or really invoke the function we
  wanted to (when all signatures are in place), so test invocations don't really
  help and the user needs to add some GAS to all of these transactions

Notary on-chain Alphabet multisignature collection solution
[uses Notary subsystem](https://github.com/nspcc-dev/neofs-node/pull/404) to
successfully solve these problems, e.g. to calculate precisely amount of GAS to
pay for contract invocation witnessed by Alphabet nodes (see
[nspcc-dev/neofs-node#47](https://github.com/nspcc-dev/neofs-node/issues/47)),
to reduce container creation delay
(see [nspcc-dev/neofs-node#519](https://github.com/nspcc-dev/neofs-node/issues/519))
etc.

### Contract-sponsored (free) transactions

The original problem and solution are described in the
[neo-project/neo#2577](https://github.com/neo-project/neo/issues/2577) discussion.