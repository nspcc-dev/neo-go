# Roadmap for neo-go

This defines approximate plan of neo-go releases and key features planned for
them. Things can change if there is a need to push a bugfix or some critical
functionality.

## Versions 0.7X.Y (as needed)
* Neo 2.0 support (bug fixes, minor functionality additions)

## Version 0.102.1 (~October 2023)
 * bug fixes

## Version 0.103.0 (~November 2023)
 * extended data types for iterators to be used by RPC wrapper generator
 * RPC subscription extensions

## Version 1.0 (2023, TBD)
 * stable version

# Deprecated functionality

As the node and the protocol evolve some external APIs can change. Usually we
try keeping backwards compatibility for some time (like half a year) unless
it's impossible to do for some reason. But eventually old
APIs/commands/configurations will be removed and here is a list of scheduled
breaking changes. Consider changing your code/scripts/configurations if you're
using anything mentioned here.

## Node-specific configuration moved from Protocol to Application

GarbageCollectionPeriod, KeepOnlyLatestState, RemoveUntraceableBlocks,
SaveStorageBatch and VerifyBlocks settings were moved from
ProtocolConfiguration to ApplicationConfiguration in version 0.100.0. Old
configurations are still supported, except for VerifyBlocks which is replaced
by SkipBlockVerification with inverted meaning (and hence an inverted default)
for security reasons.

Removal of these options from ProtocolConfiguration is scheduled for May-June
2023 (~0.103.0 release).

## GetPeers RPC server response type changes and RPC client support

GetPeers RPC command returns a list of Peers where the port type has changed from 
string to uint16 to match C#. The RPC client currently supports unmarshalling both
formats. 

Removal of Peer unmarshalling with string based ports is scheduled for ~September 2023
(~0.105.0 release).

## `NEOBalance` from stack item
 
We check struct items count before convert LastGasPerVote to let RPC client be compatible with
old versions.

Removal of this compatiblility code is scheduled for Sep-Oct 2023.

## `serv_node_version` Prometheus gauge metric

This metric is replaced by the new `neogo_version` and `server_id` Prometheus gauge
metrics with proper version formatting. `neogo_version` contains NeoGo version
hidden under `version` label and `server_id` contains network server ID hidden
under `server_id` label.

Removal of `serv_node_version` is scheduled for Sep-Oct 2023 (~0.105.0 release).

## RPC error codes returned by old versions and C#-nodes 

NeoGo retains certain deprecated error codes: `neorpc.ErrCompatGeneric`, 
`neorpc.ErrCompatNoOpenedWallet`. They returned by nodes not compliant with the 
neo-project/proposals#156 (NeoGo pre-0.102.0 and all known C# versions).

Removal of the deprecated RPC error codes is planned once all nodes adopt the new error standard.