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

## Dump*Slot() methods of `vm.Context`

The following new methods have been exposed to give access to VM context slot contents
with greater flexibility:
- `ArgumentsSlot`
- `LocalsSlot`
- `StaticsSlot`.

Removal of the `Dump*Slot()` methods are scheduled for the 0.108.0 release.
