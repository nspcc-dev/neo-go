# Roadmap for neo-go

This defines approximate plan of neo-go releases and key features planned for
them. Things can change if there is a need to push a bugfix or some critical
functionality.

## Versions 0.7X.Y (as needed)
* Neo 2.0 support (bug fixes, minor functionality additions)

## Version 0.112.0 (~Aug 2025)
 * protocol updates
 * bug fixes

## Version 1.0 (2025, TBD)
 * stable version

# Deprecated functionality

As the node and the protocol evolve some external APIs can change. Usually we
try keeping backwards compatibility for some time (like half a year) unless
it's impossible to do for some reason. But eventually old
APIs/commands/configurations will be removed and here is a list of scheduled
breaking changes. Consider changing your code/scripts/configurations if you're
using anything mentioned here.

## GetBlockHeader and GetBlockHeaderVerbose methods of RPCClient

GetBlockHeader and GetBlockHeaderVerbose were replaced by GetBlockHeaderByHash
and GetBlockHeaderByHashVerbose methods respectively to follow RPCClient
naming convention. No functional changes implied.

Removal of GetBlockHeader and GetBlockHeaderVerbose methods is scheduled for
0.112.0 release.

## SessionExpirationTime of RPC server configuration

RPC server configuration setting SessionExpirationTime of `int` type has been
replaced by SessionLifetime of `Duration` type.

Removal of SessionExpirationTime is scheduled for 0.113.0 release.
