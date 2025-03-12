/*
Package rpcclient implements NEO-specific JSON-RPC 2.0 client.

This package itself is designed to be a thin layer on top of the regular JSON-RPC
interface provided by Neo implementations. Therefore the set of methods provided
by clients is exactly the same as available from servers and they use data types
that directly correspond to JSON data exchanged. While this is the most powerful
and direct interface, it at the same time is not very convenient for many
purposes, like the most popular one --- performing test invocations and
creating/sending transactions that will do something to the chain. Please check
subpackages for more convenient APIs.

# Subpackages

The overall structure can be seen as a number of layers built on top of rpcclient
and on top of each other with each package and each layer solving different
problems.

These layers are:

  - Basic RPC API, rpcclient package itself.

  - Generic invocation/transaction API represented by invoker, unwrap (auxiliary,
    but very convenient) and actor packages. These allow to perform test
    invocations with plain Go types, use historic states for these invocations,
    get the execution results from reader functions and create/send transactions
    that change something on-chain.

  - Standard-specific wrappers that are implemented in nep11 and nep17 packages
    (with common methods in neptoken). They implement the respective NEP-11 and
    NEP-17 APIs both for safe (read-only) and state-changing methods. Safe methods
    require an Invoker to be called, while Actor is used to create/send
    transactions.

  - Contract-specific wrappers for native contracts that include management, gas,
    neo, oracle, policy and rolemgmt packages for the respective native contracts.
    Complete contract functionality is exposed (reusing nep17 package for gas and
    neo).

  - Notary actor and contract, a bit special since it's a NeoGo protocol
    extension, but notary package provides both the notary native contract wrapper
    and a notary-specific actor implementation that allows to easily wrap any
    transaction into a notary request.

  - Non-native contract-specific wrappers, currently partially provided only for
    NNS contract (it's still in development), at the moment that's mostly an
    example of how contract-specific wrappers can be built for other dApps
    (reusing invoker/actor layers it's pretty easy).

# Client

After creating a client instance with or without a ClientConfig
you can interact with the NEO blockchain by its exposed methods.

Supported methods

	calculatenetworkfee
	findstates
	getapplicationlog
	getbestblockhash
	getblock
	getblockcount
	getblockhash
	getblockheader
	getblockheadercount
	getcommittee
	getconnectioncount
	getcontractstate
	getnativecontracts
	getnep11balances
	getnep11properties
	getnep11transfers
	getnep17balances
	getnep17transfers
	getpeers
	getrawmempool
	getrawtransaction
	getstate
	getstateheight
	getstateroot
	getstorage
	gettransactionheight
	getunclaimedgas
	getnextblockvalidators
	getversion
	invokefunction
	invokescript
	invokecontractverify
	sendrawtransaction
	submitblock
	submitoracleresponse
	terminatesession
	traverseiterator
	validateaddress

Extensions:

	getblocksysfee
	getrawnotarypool
	getrawnotarytransaction
	invokecontainedscript
	submitnotaryrequest

Unsupported methods

	canceltransaction
	claimgas
	dumpprivkey
	getbalance
	getmetricblocktimestamp
	getnewaddress
	getwalletheight
	importprivkey
	listaddress
	listplugins
	sendfrom
	sendmany
	sendtoaddress
*/
package rpcclient
