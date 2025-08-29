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

## SessionExpirationTime of RPC server configuration

RPC server configuration setting SessionExpirationTime of `int` type has been
replaced by SessionLifetime of `Duration` type.

Removal of SessionExpirationTime is scheduled for 0.113.0 release.

## Candidate registration via `registerCandidate` method of native NeoToken contract

The original way of Neo candidate registration via `wallet candidate register` CLI
command using `registerCandidate` method of native NeoToken contract is deprecated
and has been superseded by the GAS transfer approach. Deprecated candidate
registration way via `registerCandidate` method call is supported via
`--useRegisterCall` flag.

Removal of `registerCandidate`–based support of candidate registration will be
done once `registerCandidate` method is officially deprecated and removed from
the NeoToken manifest with the subsequent hardfork.

## math.Max and math.Min interop functions

Interop functions `math.Min` and `math.Max` have been replaced by built-in `min`
and `max` functions.

The old functions are deprecated and their removal is scheduled for 0.113.0 release.
