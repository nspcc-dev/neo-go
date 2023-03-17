# Roadmap for neo-go

This defines approximate plan of neo-go releases and key features planned for
them. Things can change if there is a need to push a bugfix or some critical
functionality.

## Versions 0.7X.Y (as needed)
* Neo 2.0 support (bug fixes, minor functionality additions)

## Version 0.102.0 (~March 2022)
 * 3.6.0 compatibility

## Version 0.102.1 (~April 2022)
 * improved RPC error codes
 * extended data types for iterators to be used by RPC wrapper generator

## Version 1.0 (2023, TBD)
 * stable version

# Deprecated functionality

As the node and the protocol evolve some external APIs can change. Usually we
try keeping backwards compatibility for some time (like half a year) unless
it's impossible to do for some reason. But eventually old
APIs/commands/configurations will be removed and here is a list of scheduled
breaking changes. Consider changing your code/scripts/configurations if you're
using anything mentioned here.

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
earlier than May 2023, with 0.103.0 release.

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

Removal of these APIs is scheduled for May 2023 (~0.103.0 release).

## Prometheus RPC counters

A number of neogo_${method}_called Prometheus counters are marked as
deprecated since version 0.99.5, neogo_rpc_${method}_time histograms can be
used instead (that also have a counter).

It's not a frequently used thing and it's easy to replace it, so removal of
old counters is scheduled for January-February 2023 (~0.100.X release).

## SecondsPerBlock protocol configuration

With 0.100.0 version SecondsPerBlock protocol configuration setting was
deprecated and replaced by a bit more generic and precise TimePerBlock
(allowing for subsecond time). An informational message is printed on node
startup to inform about this, it's very easy to deal with this configuration
change, just replace one line.

Removal of SecondsPerBlock is scheduled for May-June 2023 (~0.103.0 release).

## Services/node address and port configuration

Version 0.100.0 of NeoGo introduces a multiple binding addresses capability to
the node's services (RPC server, TLS RPC configuration, Prometheus, Pprof) and
the node itself. It allows to specify several listen addresses/ports using an
array of "address:port" pairs in the service's `Addresses` config section and
array of "address:port:announcedPort" tuples in the `ApplicationConfiguration`'s
`Addresses` node config section. Deprecated `Address` and `Port` sections of
`RPC`, `Prometheus`, `Pprof` subsections of the `ApplicationConfiguration`
as far as the one of RPC server's `TLSConfig` are still available, but will be
removed, so please convert your node configuration file to use new `P2P`-level
`Addresses` section for the node services. Deprecated `Address`, `NodePort` and
`AnnouncedPort` sections of `ApplicationConfiguration` will also be removed
eventually, so please update your node configuration file to use `Addresses`
section for the P2P addresses configuration.

Removal of these config sections is scheduled for May-June 2023 (~0.103.0 release).

## P2P application settings configuration

Version 0.100.0 of NeoGo marks the following P2P application settings as
deprecated: `AttemptConnPeers`, `BroadcastFactor`, `DialTimeout`,
`ExtensiblePoolSize`, `MaxPeers`, `MinPeers`, `PingInterval`, `PingTimeout`,
`ProtoTickInterval`. These settings are moved to a separate `P2P` section of
`ApplicationConfiguration`. The `DialTimeout`, `PingInterval`, `PingTimeout`,
`ProtoTickInterval` settings are converted to more precise `Duration` format
(allowing for subsecond time). Please, update your node configuration (all you
need is to move specified settings under the `P2P` section and convert
time-related settings to `Duration` format).

Removal of deprecated P2P related application settings is scheduled for May-June
2023 (~0.103.0 release).

## Direct UnlockWallet consensus configuration

Top-level UnlockWallet section in ApplicationConfiguration was used as an
implicit consensus service configuration, now this setting (with Enabled flag)
is moved into a section of its own (Consensus). Old configurations are still
supported, but this support will eventually be removed.

Removal of this compatibility code is scheduled for May-June 2023 (~0.103.0
release).

## Node-specific configuration moved from Protocol to Application

GarbageCollectionPeriod, KeepOnlyLatestState, RemoveUntraceableBlocks,
SaveStorageBatch and VerifyBlocks settings were moved from
ProtocolConfiguration to ApplicationConfiguration in version 0.100.0. Old
configurations are still supported, except for VerifyBlocks which is replaced
by SkipBlockVerification with inverted meaning (and hence an inverted default)
for security reasons.

Removal of these options from ProtocolConfiguration is scheduled for May-June
2023 (~0.103.0 release).
