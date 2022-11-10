# Roadmap for neo-go

This defines approximate plan of neo-go releases and key features planned for
them. Things can change if there is a need to push a bugfix or some critical
functionality.

## Versions 0.7X.Y (as needed)
* Neo 2.0 support (bug fixes, minor functionality additions)

## Version 0.100.0 (aligned with C# node 3.5.0 release, ~December 2022)
 * 3.5.0 protocol changes
 * drop some deprecated code (see below)
 * RPC wrappers generator with extended type information

## Version 1.0 (2023, TBD)
 * stable version

# Deprecated functionality

As the node and the protocol evolve some external APIs can change. Usually we
try keeping backwards compatibility for some time (like half a year) unless
it's impossible to do for some reason. But eventually old
APIs/commands/configurations will be removed and here is a list of scheduled
breaking changes. Consider changing your code/scripts/configurations if you're
using anything mentioned here.

## getversion RPC reply Magic and StateRootInHeader fields

"getversion" RPC reply format was extended to contain "protocol" section in
version 0.97.3. Since then we have deprecated Magic and StateRootInHeader
fields in the Version structure that allows to decode replies from pre-0.97.3
servers while RPC server implementation populates both old and new fields for
compatibility with pre-0.97.3 clients.

Version 0.97.3 was released in October 2021 and can't really be used today on
public networks, we expect at least 0.99.0+ to be used in production (0.99.2+
for C# 3.4.0 compatibility). Therefore these old fields are scheduled to be
removed in version 0.100.0 both client-side and server-side. If any of your
code uses them, just use the Protocol section with the same data.

## InitialGasDistribution field of getversion RPC reply

An incompatibility with C# node's InitialGasDistribution representation was
detected and fixed in version 0.99.0 of NeoGo (June 2022). Some compatibility
code was added to handle pre-0.99.0 servers on the client side, however it was
not possible to change pre-0.99.0 clients to work with newer servers.

We expect all servers to be migrated to 0.99.0+ by now, therefore
compatibility code will be removed in version 0.100.0.

## Old RPC client APIs

A huge set of RPC client APIs was deprecated in versions 0.99.2 and 0.99.3
(August-September 2022), including very frequently used ones like
SignAndPushInvocationTx, AddNetworkFee, TransferNEP17. A new set of
invoker/actor/unwrap/nep17/etc packages was introduced decoupling these
functions from RPC client and simplifying typical backend code. Please refer
to rpcclient package documentation for specific replacements for each of these
APIs and convert your code to using them.

While a lot of the code is already converted to new APIs, old ones still can
be used in some code not known to us. Therefore we will remove old APIs not
earlier than February-March 2023, with 0.101.0 or 0.102.0 release.

## util.FromAddress smart contract helper

`util` smart contract library has a FromAddress function that is one of the
oldest lines in the entire NeoGo code base, dating back to 2018. Version
0.99.4 of NeoGo (October 2022) has introduced a new `address` package with
`ToHash160` function, it covers a bit more use cases but can be used as a
direct replacement of the old function, so please update your code.

util.FromAddress is expected to be removed around March 2023 (~0.102.0
release).

## WSClient Notifications channel and SubscribeFor* APIs

Version 0.99.5 of NeoGo introduces a new set of subscription APIs that gives
more control to the WSClient user that can pass specific channels to be used
for specific subscriptions now. Old APIs and generic Notifications channel are
still available, but will be removed, so please convert your code to using new
Receive* APIs.

Removal of these APIs is scheduled for March-April 2023 (~0.102.0-0.103.0
releases).

## Prometheus RPC counters

A number of neogo_${method}_called Prometheus counters are marked as
deprecated since version 0.99.5, neogo_rpc_${method}_time histograms can be
used instead (that also have a counter).

It's not a frequently used thing and it's easy to replace it, so removal of
old counters is scheduled for January-February 2023 (~0.100.X release).
