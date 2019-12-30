# Changelog

This document outlines major changes between releases.

## 0.71.0 "Celebration" (30 Dec 2019)

Version 0.71.0 ends the year 2019 of neo-go development with a solid release
that adds support for pprof, implements all missing interoperability
functions, improves performance and as usual fixes some bugs. This is also the
first release with a test coverage of more than 60% which is a significant
milestone in our trend to improve code quality and testing. Back in August
2019 when NSPCC had started neo-go developement the project only had a
coverage of 45% and even though a lot of code has been added since we're
improving this metrics with every release.

New features:
 * pprof support (disabled by default, #536)
 * compiler now supports assignments to an element of slice using variable
   index (#564)
 * hashed 4-byte IDs support for SYSCALL instruction  (#565)
 * remainder ('%') operator support in compiler (#563)
 * Neo.Witness.GetVerificationScript and Neo.InvocationTransaction.GetScript
   interop functions (#421)
 * Neo.Iterator.*, Neo.Enumerator.* and Neo.Storage.Find interops (#422)
 * tuple return support in compiler (#562)
 * `getblocks` P2P command implementation (#577)

Behaviour changes:
 * `Monitoring` config section changed its name to `Prometheus` because of
   pprof addition (#536)
 * `db dump` command now also writes the block number zero by default to
   comply with ImportBlocks format (#582)
 * `skip` parameter was renamed to `start` in the `db dump` command (#582)
 * `db restore` now skips the genesis block in the dump if starting to import
   into the new DB and if this block matches the one generated on blockchain
   init (#582)

Improvements:
 * VM optimizations (#558, #559, #565, #572)
 * top block caching in core to improve dApps performance (#572)
 * transaction's TXType can now be JSON-unmarshalled (#575)
 * `crypto` package no longer exports AES functions (#579)
 * merkle tree functions and structures were moved into the `hash` package
   (#579)
 * base58 functionality was moved into its own package from `crypto` (#579)
 * `Uint160DecodeAddress` and `AddressFromUint160` functions were moved out of
   `crypto` into their own package (#579)
 * `address` package now can be configured to use different prefixes (#579)

Bugs fixed:
 * wrong INVERT instruction behaviour for converted values (#561)
 * wrong DUP behaviour when the next instruction doesn't convert the value
   (#561)
 * OVER and PICK implementations didn't properly duplicate stack items (#561)
 * PICKITEM instruction failed to duplicate picked items (#561)
 * `getblock` RPC call was not answering with full hex-encoded block (#575)
 * base58.CheckDecode was decoding values with leading zeroes in a wrong way
   (#579)
 * decoding wrong public key could succeed (#579)
 * consensus payload verification was inconsistent with other verification
   code (#555)
 * DB dump was using wrong format in release 0.70.1 (#582)
 * DB restorer was not properly counting skipped blocks which could lead to
   bogus error being reported (#582)
 * the DB was not closed properly on error during DB restore (#582)
 * NPE in consensus (#583)

## 0.70.1 "Centrifugation" (16 Dec 2019)

Release 0.70.1 brings no new functionality focusing on internal node
improvements instead. Some bugs were fixed, refactoring was done and
performance was improved substantially.

Behaviour changes:
 * as with the previous release, this one changes the database format, so you
   need to resynchronize your nodes
 * code testing will fail if you're not to checkout submodules (we have one
   with neo-vm tests, #543)

Improvements:
 * "inv" command handling was changed to not ask peers of the items we already
   have (#533)
 * numerous optimizations across the whole system (#535, #546, #547, #545,
   #549, #550, #553, #554, #556, #557)
 * compiler no longer outputs errors by itself, passing them programmatically
   instead via its API (#457)
 * VM opcodes were moved into their own package allowing wider reuse (#538)
 * `compiler` package was moved one level up from `pkg/vm/compiler` to
   `pkg/compiler` (#538)
 * proper script trigger constants were added (#509)
 * consensus payloads are now properly signed and this signature is being
   checked (#544)
 * Uint160/Uint256 types were made more consistent and having an explicit
   LE/BE suffix for conversion functions (#314)
 * a separate layer based on storage.Store with state item-specific functions
   was created (#517, #334)
 * ReadBytes/WriteBytes in io package were renamed into
   ReadVarBytes/WriteVarBytes to more accurately describe them and be more
   consistent with C# implementation (#545)
 * io package was refactored to add type-specific read/write methods to its
   structures and remove generic (slow) ReadLE/WriteLE (#553, #554)
 * a simple tx performance benchmark was added (#551)

Bugs fixed:
 * consensus could start and stall with not-yet-connected peers (#532)
 * nil pointer dereference in PrepareResponse and PrepareRequest messages
   handling (#532)
 * fatal error because of locking problems in mempool (#535)
 * missing error handling in compiler emitter functions (#297)
 * wallet opening command now hides the password printed into it (#539)
 * storage was updated improperly when several transactions in one block were
   touching the same keys, leading to subsequent transactions failures (#542)
 * application execution results were not saved in full (#517)
 * mempool transaction verification was wrong (#557)

## 0.70.0 "Constellation" (29 Nov 2019)

This is a long-awaited and exciting release implementing a full Neo
consensus node that you can run your own neo-go private net with! It also
brings with it serious improvements in contract handling, you can now not
only compile, but also deploy and invoke contracts with neo-go.

New features:
 * systemd unit file for deployment (#326)
 * claim transactions processing was added (#489)
 * Consensus payloads decoding/encoding (#431)
 * `getunspents` method is RPC server (#473)
 * updated docker-compose environment for privnet setup (#497, #529)
 * client-side `getunspents` RPC method support (#511)
 * contract deployment from the CLI (#474)
 * enrollment and state transactions processing (#508)
 * `Neo.Blockchain.GetValidators` interop support (#420)
 * `invokefunction` RPC method support in the server (#347)
 * `testinvokefunction` command in the CLI to do test invocations via
   `invokefunction` RPC method (#521)
 * consensus node support (#507, #525)
 * server-side `invoke` RPC method support (#346)
 * `testinvoke` CLI command to invoke contracts via `invoke` RPC method (#527)
 * `getheaders` P2P message processing (#529)
 * relaying was added for transactions coming from P2P network (#529)
 * `invoke` and `invokefunction` commands to invoke deployed script and send an
   invocation transaction to the network (#531)

Behavior changes:
 * db dump/restore format is now compatible with NGD chain dumps (#466)
 * smart contracts now have a new configuration format that is used to deploy
   them (#511)
 * `testinvoke` CLI command was renamed to `testinvokescript` (#521)

Improvements:
 * `core.Blockchainer` interface now has a `Close` method (#485)
 * `util.Uint256Size` is now public (#490)
 * `io` package now has generic functions for array
   serialization/deserialization (#490)
 * `util.Uint256` now supports `io.Serializable` interface (#495)
 * `smartcontract.ParamType` type now supports `io.Serializable` interface
   (#495)
 * `vm.ByteArrayItem` now uses hex representation when being marshalled into
   JSON (#499)
 * `io` serialization/deserialization for arrays is now restricted in elements
   count (#503, #505)
 * `core.AccountState` now stores all UTXOs for the account (#504)
 * interop functions got some testing coverage (#492)
 * rpc client now implements `CalculateInputs` method via `getunspents` call
   for transaction building (#511)
 * `transaction.NewInvocationTX` now accepts a gas parameter for the
   corresponding transaction field (#511)
 * `rpc.StackParamType` now supports YAML marshaling/unmarshaling
 * `rpc` package now has more fine-grained methods for transaction building
   (#511, #531)
 * `Blockchain` now stores and updates validators list (#508)
 * blockchain state management refactored (#508)
 * `rpc` invocation parameter management reworked (#513)
 * `util` test coverage improved (#515)
 * `invokescript` tests were added to the `rpc` package (#521)
 * `crypto/keys` and `crypto/hash` packages test coverage improved (#516)

Bugs fixed:
 * blockchain not persisting the latest changes on exit (#485)
 * db dump/restore commands incorrectly handled `skip` parameter (#486)
 * vm failed to serialize duplicating non-reference elements (#496)
 * improper smartcontract notifications handling (#453)
 * nondeterministic `GetReferences` interop behaviour leading to contract
   failures (#454)
 * writing message to the peer could be interleaved with other messages
   leading to garbage being sent (#503, #506)
 * inability to process block with previously relayed transaction (#511)
 * decoding transaction with invalid type didn't return an error (#522)
 * attempts to reconnect to the node with the same ID (#507)
 * peer disconnects during handshake because of code race (#529)
 * useless header requests from peers with low height (#529)
 * wrong header hashes initialization from the DB in case there are 2000*N + 1
   blocks in the chain (#529)

## 0.62.0 "Commotion" (07 Nov 2019)

Release 0.62.0 finishes one very important work some pieces of which were
gradually rolled out in previous releases --- it integrates all neo-vm project
JSON-based tests for NEO 2.0 C# VM and runs them successfully against neo-go
VM. There are also important bug fixes based on mainnet nodes deployment
experience and additional configuration options.

New Features:
 * implemented `Runtime.Serialize` and `Runtime.Deserialize` syscalls (#419)
 * new configuration option -- `AttemptConnPeers` to set the number of
   connections that the node will try to establish when it goes below the
   MinPeers setting (#478)
 * `LogPath` configuration parameter to write logs into some file and not to
   stdout (#460), not enabled by default
 * `Address` configuration parameter to specify the address to bind to (#460),
   not enabled by default

Behavior changes:
 * mainnet configuration now has correct ports specified (#478)
 * multiple connections to the same peer are disallowed now (as they are in C#
   node (#478))
 * the default MaxPeers setting was increased to 100 for mainnet and testnet
   configurations and limited to 10 for privnet (#478)

Improvements:
 * implemented missing VM constraints: stack item number limitation (#462) and
   integer size checks (#484, #373)
 * added a framework to run JSON-based neo-vm tests for C# VM and fixed all
   remaining incompabitibilities (#196)
 * added wallet unit tests (#475)
 * network.Peer's NetAddr method was split into RemoteAddr and PeerAddr (#478)
 * `MakeDirForFile` function was added to the `io` package (#470)

Bugs fixed:
 * RPC service responded with block height to `getblockcount` request which
   differs from C# interpretation of `getblockcount` (#471)
 * `getbestblockhash` RPC method response was not adding leading `0x` prefix
   to the hash, while C# node does it
 * inability to correctly handshake clients on the server side (#458, #480)
 * data race in `Server` structure fields access (#478)
 * MaxPeers configuration setting was not working properly (#478)
 * useless DB reads (that failed in some cases) on persist attempt that didn't
   persist anything (#481)
 * current header height was not stored in the DB when starting a new
   blockchain which lead to node failures on restart (#481)
 * crash on node restart if no header hashes were written into the DB (#481)

## 0.61.0 "Cuspidation" (01 Nov 2019)

New features:
 * Prometheus support for monitoring (#441)
 * `neo-go contract invoke` now accepts endpoint parameter (`--endpoint` or
   `-e`) to specify RPC node to be used for invocation (#363)
 * RPC server now supports `invokescript` method (#348)
 * minimum peers number can now be configured (#468)
 * configured CORS workaround implemented in the RPC package (#469)

Behavior changes:
 * `neo-go contract inspect` now expects avm files in input, but can also
   compile Go code with `-c` parameter (previously is was done by default),
   `inspect` subcommand was removed from `neo-go vm` (it dumped avm files in
   previous release) (#463)
 * the default minimum peers was reduced to 3 for privnet setups to avoid
   useless reconnections to only 4 available nodes
 * RPC service now has its own section in configuration, update your
   configurations (#469)

Improvements:
 * VM.Load() now clears the state properly, making VM reusable after the Run()
   (#463)
 * Compile() in compiler package no longer accepts Options, they were not used
   previously anyway (#463)
 * invocation stack depth is now limited in the VM (#461)
 * VM got new State() method to get textual state description (#463)
 * vm's Stack structure can now be marshalled into JSON (#463)

Bugs fixed:
 * race in discoverer part of the server (#445)
 * RPC server giving improper (not JSON) respons to unimplemented API requests
   (#463)

## 0.60.0 "Cribration" (25 Oct 2019)

Release 0.60.0 brings with it an implementation of all NEO 2.0 VM opcodes,
full support for transaction relaying, improved logging, a bunch of fixes and
an updated project logo.

New features:
 * blocks dumping from DB to file and restoring from file to DB (#436)
 * new logo (#444)
 * implemented `getdata` message handling (#448)
 * issue tx processing (#450)
 * CALL_I, CALL_E, CALL_ET, CALL_ED, CALL_EDT implementation in the VM (#192)

Internal improvements:
 * codestyle fixes (#439, #443)
 * removed spurious prints from all the code, now everything is passed/logged
   correctly (#247)

Bugs fixed:
 * missing max size limitation in CAT and PUSHDATA4 opcodes implementation
   (#435)
 * wrong interpretation of missing unspent coin state when checking for double
   spend (#439)
 * panic on successive node starts when no headers were saved in the DB (#440)
 * NEWARRAY/NEWSTRUCT opcodes didn't copy operands for array<->struct
   conversions
 * deadlock in MemPool on addition (#448)
 * transactions were not removed from the MemPool when processing new signed
   block (#446)
 * wrong contract property constants leading to storage usage failures (#450)

## 0.51.0 "Confirmation" (17 Oct 2019)

With over a 100 commits made since 0.50.0 release 0.51.0 brings with it full
block verification, improved and fixed transaction verifications,
implementation of most of interop functions and VM improvements. Block
verification is an important milestone on the road to full consensus node
support and it required implementing a lot of other associated functionality.

New features:
 * CHECKSIG, VERIFY and CHECKMULTISIG instructions in VM (#269)
 * witness verification logic for transactions (#368)
 * NEWMAP, HASKEY, KEYS and VALUES instructions, support for Map type in
   PICKITEM, SETITEM, REMOVE, EQUAL, ARRAYSIZE (#359)
 * configurable transaction verification on block addition (#415, #418)
 * contract storage and support for VM to call contracts via APPCALL/TAILCALL
   (#417)
 * support for Interop type in VM (#417)
 * VM now has stepInto/stepOver/stepOut method implementations for debugging
   matching neo-vm behavior (#187)
 * storage support for contracts (#418)
 * added around 90% of interop functions (#418)
 * Invocation TX processing now really does invoke contracts using internal VM
   (#418)
 * blocks are now completely verified when added to the chain (if not
   configured otherwise; #12, #418)

Behavior changes:
 * full block verification is now enabled for all network types
 * block's transaction verification enabled for privnet setups, mainnet and
   testnet don't have it enabled

Technical improvements:
 * GetVarIntSize and GetVarStringSize were removed from the io package (use
   GetVarSize instead; #408)
 * OVER implementation was optimized to not pop the top element from the stack
   (#406, part of #196 work)
 * vm.VM was extended with HasFailed() method to check its state (previously
   external VM users couldn't do it; #411)
 * redesigned input block queue mechanism, now it's completely moved out of
   the Blockchain, which only accepts the next block via AddBlock() (#414)
 * unpersisted blocks are now fully available with the Blockchain (thus we
   have symmetry now in AddBlock/GetBlock APIs; #414, #366)
 * removed duplicating batch structures from BoltDB and Redis code, now all of
   them use the same batch as MemoryStore does (#414)
 * MemoryStore was exporting its mutex for no good reason, now it's hidden
   (#414)
 * storage layer now returns ErrKeyNotFound for all DBs in appropriate
   situations (#414)
 * VM's PopResult() now doesn't panic if there is no result (#417)
 * VM's Element now has a Value() method to quickly get the item value (#417)
 * VM's stack PushVal() method now accepts uint8/16/32/64 (#417, #418)
 * VM's Element now has TryBool() method similar to Bool(), but without a
   panic (for external VM users; #417)
 * VM has now completely separated instruction read and execution phases
   (#417)
 * Store interface now has Delete method (#418)
 * Store tests were reimplemented to use one test set for all Store
   implementations, including LevelDB that was not tested at all previously
   (#418)
 * Batch interface doesn't have Len method now as it's not used at all (#418)
 * New*FromRawBytes functions were renamed to New*FromASN1 in the keys
   package, previous naming made it easy to confuse them with functions
   operating with NEO serialization format (#418)
 * PublicKey's IsInfinity method is exported now (#418)
 * smartcontract package now has CreateSignatureRedeemScript() matching C#
   code (#418)
 * vm package now has helper functions
   IsSignatureContract/IsMultiSigContract/IsStandardContract matching C# code
   (#418)
 * Blockchain's GetBlock() now returns full block with transactions (#418)
 * Block's Verify() was changed to return specific error (#418)
 * Blockchain's GetTransationResults was renamed into GetTransactionResults
   (#418)
 * Blockchainer interface was extended with GetUnspentCoinState,
   GetContractState and GetScriptHashesForVerifying methods (#418)
 * introduced generic MemCacheStore that is used now for write caching
   (including temporary stores for transaction processing) and batched
   persistence (#425)

Bugs fixed:
 * useless persistence failure message printed with no error (#409)
 * persistence error message being printed twice (#409)
 * segmentation fault upon receival of message that is not currently handled
   properly (like "consensus" message; #409)
 * BoltDB's Put for a batch wasn't copying data which could lead to data
   corruption (#409)
 * APPEND instruction applied to struct element was not copying it like neo-vm
   does (#405, part of #196 work)
 * EQUAL instruction was comparing array contents, while it should've compared
   references (#405, part of #196 work)
 * SUBSTR instruction was failing for out of bounds length parameters while it
   should've truncated them to string length (#406, part of #196 work)
 * SHL and SHR implementations had no limits, neo-vm restricts them to
   -256/+256 (#406, part of #196 work)
 * minor VM state mismatches with neo-vm on failures (#405, #406)
 * deadlock on Blockchain init when headers pointer is not in sync with the
   hashes list (#414)
 * node failed to request blocks when headers list was exactly one position
   ahead of block count (#414)
 * TestRPC/getassetstate_positive failed occasionally (#410)
 * panic on block verification with no transactions inside (#415)
 * DutyFlag check in GetScriptHashesForVerifying was not done correctly (#415)
 * default asset expiration for assets created with Register TX was wrong, now
   it matches C# code (#415)
 * Claim transactions needed specific GetScriptHashesForVerifying logic to
   be verified correctly (#415)
 * VerifyWitnesses wasn't properly sorting hashes and witnesses (#415)
 * transactions referring to two outputs of some other transaction were
   failing to verify (#415)
 * wrong program dumps (#295)
 * potential data race in logging code (#418)
 * bogus port check during handshake (#432)
 * missing max size checks in NEWARRAY, NEWSTRUCT, APPEND, PACK, SETITEM
   (#427, part of #373)

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
