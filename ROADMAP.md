# Roadmap for neo-go

This defines approximate plan of neo-go releases and key features planned for
them. Things can change if there is a need to push a bugfix or some critical
functionality.

## Versions 0.7X.Y (as needed)
* Neo 2.0 support (bug fixes, minor functionality additions)

## Version 0.119.0 (~Apr 2026)
 * protocol updates
 * bug fixes

## Version 1.0 (2026, TBD)
 * stable version

# Deprecated functionality

As the node and the protocol evolve some external APIs can change. Usually we
try keeping backwards compatibility for some time (like half a year) unless
it's impossible to do for some reason. But eventually old
APIs/commands/configurations will be removed and here is a list of scheduled
breaking changes. Consider changing your code/scripts/configurations if you're
using anything mentioned here.

## Candidate registration via `registerCandidate` method of native NeoToken contract

The original way of Neo candidate registration via `wallet candidate register` CLI
command using `registerCandidate` method of native NeoToken contract is deprecated
and has been superseded by the GAS transfer approach. Deprecated candidate
registration way via `registerCandidate` method call is supported via
`--useRegisterCall` flag.

Removal of `registerCandidate`–based support of candidate registration will be
done once `registerCandidate` method is officially deprecated and removed from
the NeoToken manifest with the subsequent hardfork.

## `--decrypt` flag of `wallet export` command

The `--decrypt` (`-d`) flag of the `wallet export` CLI command is deprecated
and has been superseded by the `--format wif` option. Use `--format wif`
instead of `--decrypt`.

Removal is planned for v0.120.0 release (~June 2026).

## actor.ErrExecFailed RPC client error

`actor.ErrExecFailed` RPC client error is replaced by `neorpc.FaultException`
error. Use a set of predefined `neorpc.FaultException` errors with errors.Is to
detect specific FAULT exceptions or create a custom `neorpc.FaultException`
error if no suitable standard one is available.

`actor.ErrExecFailed` will be removed in ~v0.121.0 release (~July 2026).