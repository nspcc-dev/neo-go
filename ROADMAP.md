# Roadmap for neo-go

This defines approximate plan of neo-go releases and key features planned for
them. Things can change if there is a need to push a bugfix or some critical
functionality.

## Versions 0.7X.Y (as needed)
* Neo 2.0 support (bug fixes, minor functionality additions)

## Version 0.107.0 (~Jun-Jul 2024)
 * protocol updates
 * bug fixes
 * node resynchronisation from local DB
 * CLI library upgrade

## Version 1.0 (2024, TBD)
 * stable version

# Deprecated functionality

As the node and the protocol evolve some external APIs can change. Usually we
try keeping backwards compatibility for some time (like half a year) unless
it's impossible to do for some reason. But eventually old
APIs/commands/configurations will be removed and here is a list of scheduled
breaking changes. Consider changing your code/scripts/configurations if you're
using anything mentioned here.

## GetPeers RPC server response type changes and RPC client support

GetPeers RPC command returns a list of Peers where the port type has changed from 
string to uint16 to match C#. The RPC client currently supports unmarshalling both
formats. 

Removal of Peer unmarshalling with string based ports is scheduled for Jun-Jul 2024
(~0.107.0 release).

## `NEOBalance` from stack item
 
We check struct items count before convert LastGasPerVote to let RPC client be compatible with
old versions.

Removal of this compatiblility code is scheduled for Jun-Jul 2024.

## `serv_node_version` Prometheus gauge metric

This metric is replaced by the new `neogo_version` and `server_id` Prometheus gauge
metrics with proper version formatting. `neogo_version` contains NeoGo version
hidden under `version` label and `server_id` contains network server ID hidden
under `server_id` label.

Removal of `serv_node_version` is scheduled for Jun-Jul 2024 (~0.107.0 release).

## RPC error codes returned by old versions and C#-nodes 

NeoGo retains certain deprecated error codes: `neorpc.ErrCompatGeneric`, 
`neorpc.ErrCompatNoOpenedWallet`. They returned by nodes not compliant with the 
neo-project/proposals#156 (NeoGo pre-0.102.0 and all known C# versions).

Removal of the deprecated RPC error codes is planned for Jun-Jul 2024 (~0.107.0
release).

## Block based web-socket waiter transaction awaiting

Web-socket RPC based `waiter.EventWaiter` uses `header_of_added_block` notifications
subscription to manage transaction awaiting. To support old NeoGo RPC servers
(older than 0.105.0) that do not have block headers subscription ability,
event-based waiter fallbacks to the old way of block monitoring with
`block_added` notifications subscription.

Removal of stale RPC server compatibility code from `waiter.EventWaiter` is
scheduled for Jun-Jul 2024 (~0.107.0 release).

## Dump*Slot() methods of `vm.Context`

The following new methods have been exposed to give access to VM context slot contents
with greater flexibility:
- `ArgumentsSlot`
- `LocalsSlot`
- `StaticsSlot`.

Removal of the `Dump*Slot()` methods are scheduled for the 0.108.0 release.