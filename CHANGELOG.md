# Changelog

This document outlines major changes between releases.

## 0.50.0 "Consolidation" (19 Sep 2019)

The first release from the new team focuses on bringing all related
development effort into one codebase, refactoring things, fixing some
long-standing bugs and adding new functionality. This release merges two
radically different development branches --- `dev` and `master` that were
present in the project (along with all associated pull requests) and also
brings in changes made to the compiler in the neo-storm project.

New features:
 * configurable storage backends supporting LevelDB, in-memory DB (for
   testing) and Redis
 * BoltDB support for storage backend
 * updated and extended interop APIs (thanks to neo-storm)

Notable behavior changes:
 * the default configuration for privnet was changed to use ports 20331 and
   20332 so that it doesn't clash with the default dockerized neo-privnet
   setups
 * the default configuration path was changed from `../config` to `./config`,
   at this stage it makes life a bit easier for development, later this will
   be changed to some sane default for production version
 * VM CLI now supports type inference for `run` parameters (you don't need to
   specify types explicitly in most of the cases) and treats `operation`
   parameter as mandatory if anything is passed to `run`
 
VM improvements:
 * added implementation for `EQUAL`, `NZ`, `PICK`, `TUCK`, `XDROP`, `INVERT`,
   `CAT`, `SUBSTR`, `LEFT`, `RIGHT`, `UNPACK`, `REVERSE`, `REMOVE`
 * expanded tests
 * better error messages for different erroneous code
 * implemented item conversions following neo-vm behavior: array to/from
   struct, bigint to/from boolean, anything to bytearray and anything to
   boolean
 * improved compatibility with neo-vm (#394)

Technical improvements:
 * switched to Go 1.12+
 * gofmt, golint (#213)
 * fixed and improved CircleCI builds
 * removed internal rfc6969 package (#285)
 * refactored util/crypto/io packages, removed a lot of duplicating code
 * updated READMEs and user-level documents
 * update Makefile with useful targets
 * dropped internal base58 implementation (#355)
 * updated default seed lists for mainnet and testnet from neo-cli

Bugs fixed:
 * a lot of compiler fixes from neo-storm
 * data access race in memory-backed storage backend (#313)
 * wrong comparison opcode emitted by compiler (#294)
 * decoding error in `publish` transactions (#179)
 * decoding error in `invocation` transactions (#173)
 * panic in `state` transaction decoding
 * double VM run from CLI (#96)
 * non-constant time crypto (#245)
 * APPEND pushed value on the stack and worked for bytearrays (#391)
 * reading overlapping hash blocks from the DB leading to blockchain state
   neo-go couldn't recover from (#393)
 * codegen for `append()` wasn't type-aware and emitted wrong code (#395)
 * node wasn't trying to reconnect to other node if connection failed (#390)
 * stricly follow handshare procedure (#396)
 * leaked connections if disconnect happened before handshake completed (#396)

### Inherited unreleased changes

Some changes were also done before transition to the new team, highlights are:
 * improved RPC testing
 * implemented `getaccountstate`, `validateaddress`, `getrawtransaction` and
   `sendrawtransaction` RPC methods in server
 * fixed `getaccountstate` RPC implementation
 * implemented graceful blockchain shutdown with proper DB closing

## 0.45.14 (not really released, 05 Dec 2018)

This one can technically be found in the git history and attributed to commit
fa1da2cb917cf4dfccbe49d44c5741eec0e0bb65, but it has no tag in the repository
and so can't count as a properly released thing. Still it can be marked as a
point in history with the following changes relative to 0.44.10:
 * switched to Go modules for dependency management
 * various optimizations for basic structures like Uin160/Uint256/Fixed8
 * improved testing
 * added support for `invoke` method in RPC client
 * implemented `getassetstate` in RPC server
 * fixed NEP2Encrypt and added tests
 * added `NewPrivateKeyFromRawBytes` function to the `wallet` package

## 0.44.10 (27 Sep 2018)

This is the last one tagged in the repository, so it's considered as the last
one properly released before 0.50+. Releases up to 0.44.10 seem to be made in
automated fashion and you can find their [changes on
GitHub](https://github.com/nspcc-dev/neo-go/releases).
