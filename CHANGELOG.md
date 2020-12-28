# Changelog

This document outlines major changes between releases.

## 0.92.0 "Fermentation" (28 Dec 2020)

NeoGo project is closing year 2020 with 3.0.0-preview4 compatible release that
also has much improved performance, a lot of updates to compiler and SDK and
some experimental protocol extensions. This release also is the most tested
release of NeoGo ever, we've reached 83% code coverage (Neo 2.x only has
66%).

Please note that this release is incompatible with 0.91.0 and there are no
plans for long-term support of it, Neo 3 is still changing and improving.

Protocol-wise this release is tested with preview4 testnet (including working
in consensus with C# nodes) and it is compatible with it even though there are
some known storage change mismatches between NeoGo and C# (functionally the
contents is the same, this mismatch is caused by JSON handling differences and
needs to be addressed at the protocol level).

New features:
 * "high priority" transaction attribute (#1341)
 * "oracle response" transaction attribute (#1407, #1520, #1573)
 * new GAS distribution and committee update scheme (#1364, #1529, #1518,
   #1542, #1549, #1636, #1639)
 * oracle native contract (#1409, #1432, #1474, #1476, #1521, #1528, #1548,
   #1556, #1625)
 * designation native contract (#1451, #1504, #1519)
 * support for `_deploy` method (#1452, #1466)
 * `MaxTraceableBlocks` parameter can now be configured (#1520)
 * `KeepOnlyLatestState` configuration option to drop old MPT data (#1553)
 * `StateRootInHeader` configuration option to include state root data in the
   header (it's a protocol extension, so use with care, #1500)
 * NEP-5 was replaced by NEP-17 (#1558, #1574, #1617, #1636)
 * `RemoveUntraceableBlocks` configuration option to perform old (unreachable)
   block and transaction purging (#1561)
 * stable deployed contract hashes (#1555, #1616, #1628)
 * `Safe` method flag for contract manifests (#1597, #1607)
 * native Management contract to deploy/update/destroy contracts (#1610,
   #1624, #1636)
 * Compiler/SDK:
   - base58 encoding/decoding interops for smart contracts (#1350, #1375)
   - you can use aliases now when importing interop packages (#397)
   - `+=` operator is now supported for strings (#1351)
   - `switch` without statement support (#1355)
   - `make()` support for maps and slices using basic types (#1352)
   - `copy()` support for byte slices (#1352, #1383)
   - `iota` support (#1361)
   - support for basic function literals (#1343)
   - new variable initialization is now supported in `if` statements (#1343)
   - `defer` is now supported (#1343)
   - `recover()` support for exception handling (#1343, #1383)
   - `delete()` support (#1391)
   - `util.Remove()` function is added for smart contracts to remove elements
     from slice (#1401)
   - `interop` package now provides specific type aliases like Hash160/Hash256
     for smart contracts to use (it affects generated manifests, #1375, #1603)
   - variables and function calls support in struct and slice literals (#1425)
   - atoi/itoa encoding/decoding interops for smart contracts (#1530, #1533)
   - contracts specifying NEP-17 as supported standard are now checked for
     interface compliance (#1373)
   - `contract.CallEx` function is now available to contracts that allows
     specifying flags for contract called (#1598)
 * CLI:
   - support for array parameters in invocations (#1363, #1367)
   - contract addresses can now be imported into wallet (#1362)
   - deploying/invoking from multisignature accounts (#1461)
 * RPC:
   - `getcommittee` RPC call (#1416)
   - limits and paging are now supported for `get*transfers` calls (#1419)
   - `getunclaimedgas` RPC call now also supports passing address as a
     parameter (#1418)
   - `getcontractstate` now also accepts addresses, IDs and names (for native
     contracts, #1426)
   - batched JSON-RPC requests support (#1514)
   - `invokecontractverify` RPC call to run verification scripts (#1524)
 * P2P notaries subsystem (configurable `P2PSigExtensions` protocol extension,
   use with care):
   - optional `NotValidBefore` transaction attribute (#1490)
   - optional `Conflicts` transaction attribute (#1507)
   - native Notary contract (#1557, #1593)
   - notary request P2P payload (#1582)

Behavior changes:
 * converting items to boolean now fail for strings of bytes longer than 32
   (#1346)
 * consensus messages now use uint8 field for validator indexes (#1350)
 * maximum possible try-catch nesting is now limited to 16 levels (#1347)
 * maximum manifest size increased to 64K (#1365, #1555)
 * required flags changed for many interop functions (#1362)
 * compiler no longer generates code to log `panic()` message (#1343)
 * `GetBlock`, `GetTransaction` and similar interop functions now return
   pointers to structures (#1375)
 * calling script hash now also is accounted for by CheckWitness interop
   (#1435, #1439)
 * CLI is using `--address` parameter everywhere it's needed (some commands
   used `--addr` previously (#1434)
 * VM now restricts comparable byte array size to 64K (#1433)
 * `FeeOnly` witness scope was renamed to `None` (#1450)
 * `getvalidators` RPC call was renamed to `getnextblockvalidators` (#1449)
 * `emit.Opcode` is now `emit.Opcodes`, allowing for variadic specification of
   opcodes (#1452)
 * `CalculateNetworkFee` was moved to `fee.Calculate` (#1467)
 * fault exception string is now returned for failed invocations (#1462)
 * `runtime.GetInvocationCounter` no longer errors (#1455)
 * `invoke*` RPC calls now also return `transaction` field (#1418)
 * `getversion` RPC call now also returns network magic number (#1489)
 * RPC calls now return data in base64 instead of hex (#1489, #1525, #1537)
 * blocked accounts interface and storage was changed in Policy contract (#1504)
 * voting fee is lower now (#1510)
 * blocks are now processed with two execution triggers, `OnPersist` and
   `PostPersist`, `getapplicationlog` RPC call was updated to support passing
   trigger type (#1515, #1531, #1619)
 * storage fee formula has changed (#1517)
 * `MaxValidUntilBlockIncrement` is now 5760 instead of 2102400 (#1520)
 * Policy contract no longer saves default values during initialization
   (#1535)
 * opcode pricing was changed and now it's adjustable (#1538, #1615)
 * contracts no longer have `IsPayable` (see NEP-17) and `HasStorage` (they
   all have it by default now) features (#1544)
 * notification size is restricted now (#1551)
 * unsolicited `addr` messages are now treated as errors (#1562)
 * native contracts no longer have `name()` methods, it is now a part of
   manifest (#1557)
 * transaction fees, invocation GAS counters and unclaimed GAS values are now
   returned as strings with floating point values from RPC calls (#1566,
   #1572, #1604)
 * NEF format was changed (#1555)
 * `engine.AppCall()` interop was renamed to `contract.Call()` (#1598)
 * call flags were renamed (#1608)
 * deploying contract now costs at minimum 10 GAS and MaxGasInvoke
   configuration was adjusted to account for that (the fee is configurable by
   the committee, #1624, #1635)

Improvements:
 * a lot of new tests added (#1335, #1339, #1341, #1336, #1378, #1452, #1508,
   #1564, #1567, #1583, #1584, #1587, #1590, #1591, #1595, #1602, #1633,
   #1638)
 * a number of optimizations across all node's components applied (#1342,
   #1347, #1337, #1396, #1398, #1403, #1405, #1408, #1428, #1421, #1463,
   #1471, #1492, #1493, #1526, #1561, #1618)
 * smartcontract package now provides function to create simple majority
   multisignature contract (in addition to BFT majority, #1341)
 * `AddNetworkFee()` now supports contract addresses (#1362)
 * error handling and help texts for CLI wallet commands (#1434)
 * compiler emitting short jump instructions if possible (#805, #1488)
 * compiler emitting jump instructions with embedded conditions where possible
   (#1351)
 * transaction retransmission mechanism added to mempool (#1536)
 * parallel block fetching (#1568)
 * Binary and Runtime interops refactored to separate packages (#1587)
 * message broadcasts now finish successfully if the message is sent to 2/3 of
   peers (#1637)

Bugs fixed:
 * TRY opcode implementation was not allowing for 0 offsets (#1347)
 * compiler wasn't dropping unused elements returned from calls (#683)
 * MEMCPY with non-zero destination index was not working correctly (#1352)
 * asset transfer from CLI didn't work (#1354)
 * specifying unknown DB type in configuration file induced node crash (#1356)
 * address specifications in configuration file didn't work correctly (#1356)
 * RPC server wasn't processing hashes with "0x" prefix in parameters (#1368)
 * incorrect context unloding on exception handling (#1343)
 * attempt to get committee only returned validators if there was no voting on
   chain (#1370)
 * block queue could be attacked with wrong blocks to cause OOM (#1374)
 * token sale example wasn't checking witness correctly (#1379)
 * structure methods were added to generated manifests (#1381)
 * type conversions in `switch` and `range` statements were compiled as
   function calls (#1383)
 * RPC server result format wasn't conforming to C# implementation for
   `getapplicationlog` and `invoke*` (#1371, #1387)
 * subslicing non-byte slices was miscompiled (it's forbidden now, #1401)
 * potential deadlock in consensus subsystem (#1410)
 * race in peer connection closing method (#1378)
 * race in MPT calculation functions (#1378)
 * possible panic on shutdown (#1411)
 * block-level `getapplicationlog` result had block hash in it (#1413)
 * fail CN execution if wrong password is provided in the configuration
   (#1419)
 * tx witness check didn't account for GAS properly when several witnesses are
   used (#1439)
 * map keys longer than 64 bytes were allowed (#1433)
 * unregistered candidate with zero votes wasn't removed (#1445)
 * standard contract's network fee wasn't calculated correctly (#1446)
 * witness checking wasn't taking into account transaction size fee (#1446)
 * sending invalid transaction from the CLI wasn't prevented in some cases
   (#1448, #1479, #1604)
 * `System.Storage.Put` wasn't accounting for new key length in its GAS price
   (#1458)
 * blocks can't have more than 64K transactions (#1454)
 * Policy native contract wasn't limiting some values (#1442)
 * MerkleBlock payload wasn't serialized/deserialized properly (#1454, #1591)
 * partial contract updates were not always possible (#1460)
 * potential panic on verification with incorrect signature size (#1467)
 * CLI attempted to save transaction when it wasn't requested (#1467)
 * VM allowed to create bigger integers than it should support (#1464)
 * some protocol limits were not enforced (#1469)
 * iterating over storage items could produce incorrect KV pairs when using
   LevelDB or BadgerDB (#1478)
 * stale transaction were not deleted from the mempool (#1480)
 * node panic during block processing with BoltDB (#1482)
 * node that failed to connect to seeds on startup never attempted to
   reconnect to them again (#1486, #1592)
 * compiler produced incorrect code for if statements using function calls
   (#1479)
 * invocation stack size check wasn't performed in some cases (#1492)
 * incorrect code produced by the compiler when assigning slices returned from
   functions to new variables (#1495)
 * websocket client closing connection on new events (#1494)
 * minor `getrawtransaction`/`gettransactionheight` and NEP5-related RPC
   implementation incompatibilities (#1543, #1550)
 * VM CLI breakpoint wasn't stopping before the instruction (#1584)
 * VM CLI was incorrectly processing missing parameter error (#1584)
 * wallet conversion wasn't performed correctly (#1590)
 * node didn't return all requested blocks in response to `getblocks` P2P
   requests (#1595)
 * CN didn't request transactions properly from its peers in some cases
   (#1595)
 * incorrect manifests generated for some parameter types (#1599)
 * incorrect code generated when no global variables are present, but there
   are some global constants (#1598)
 * native contract invocations now set proper calling script hash (#1600)
 * byte string and buffer VM stack items conversion to JSON differed from C#
   (#1609)
 * when mempool is full new transaction's hash could still be added into it
   even if it is to be rejected afterwards (#1621)
 * CN wasn't always performing timestamp validation correctly (#1620)
 * incorrect stack contents after execution could stop block processing
   (#1631)
 * `getapplicationlog` RPC call handler wasn't validating its parameters
   properly, potentially leading to node crash (#1636)
 * a peer could be connected twice in rare circumstances (#1637)
 * missing write timeout could lead to broadcasting stalls (#1637)

## 0.91.0 "Ululation" (18 August 2020)

We've updated NeoGo for 3.0.0-preview3 compatibility implementing all the
appropriate protocol changes as well as improving NeoGo-specific
components. This release brings with it significant changes for smart
contracts, both in terms of Neo protocol changes (no more there is a single
entry point! execution environment has also changed in lots of ways) and Go
smart contract compiler updates and fixes.

Please note that this release is incompatible with 0.90.0 and there will be no
long-term support provided for it, Neo 3 is still changing and
improving. If you have any wallets used with 0.90.0 release you'll need to
regenerate them from private keys because of changes to verification scripts
that also changed hashes and addresses.

But nonetheless it is tested to be compatible with preview3 testnet for up to
68K of blocks in terms of storage changes, with neo-debugger for debug data
produced by the compiler and with consensus process for heterogeneous setups
(like 2 neo-go CNs with 2 C# CNs).

New features:
 * secp256k1 signature checks added to interop functions
   (Neo.Crypto.VerifyWithECDsaSecp256k1 and
   Neo.Crypto.CheckMultisigWithECDsaSecp256k1 syscalls,
   crypto.ECDsaSecp256k1Verify and crypto.ECDSASecp256k1CheckMultisig interop
   functions, #918)
 * RIPEMD160 hash added to interop functions (Neo.Crypto.RIPEMD160 syscall,
   crypto.RIPEMD160 interop function, #918, #1193)
 * "NotFound" P2P message (#1135, #1333)
 * base64 encoding/decoding interop functions (binary.Base64Encode and
   binary.Base64Decode, #1187)
 * new contract.GetCallFlags interop (System.Contract.GetCallFlags syscall)
   implemented (#1187)
 * it is possible now to create iterators and enumerators over primitive VM
   types in smart contracts (#1218)
 * runtime.Platform interop (System.Runtime.Platform syscall) is now available
   to smart contracts in Go (#1218)
 * storage.PutEx interop (System.Storage.PutEx syscall) is now available to
   smart contracts in Go (#1221)
 * exceptions support in VM (#885)
 * CLI conversion utility functions for addresses/hashes/etc (#1207, #1258)
 * multitransfer transactions now can be generated with RPC client or CLI
   (#940, #1260)
 * System.Callback.* syscalls for callback creation and execution (callback.*
   interop functions, #1197)
 * MPT implementation was added (#1235)
 * Policy native contract now also contains MaxBlockSystemFee setting (#1195)
 * getting blocks by indexes via P2P is now supported (#1192)
 * limited pointer support was added to the compiler (#1247)
 * voting support in CLI (#1206, #1286)

Behavior changes:
 * crypto.ECDsaVerify interop function was renamed to
   crypto.ECDsaSecp256r1Verify now that we have support for secp256k1 curve
   (#918)
 * many RPC requests/responses changed names used for data fields (#1169)
 * runtime.Notify interop function now requires a mandatory UTF8 name
   parameter (#1052) and this name can be used to filter notifications (#1266)
 * sendrawtransaction and submitblock RPC calls now return a hash instead of
   boolean value in case of success (#1216)
 * System.Runtime.Log syscall now only accepts valid UTF8 strings no longer
   than 1024 bytes (#1218)
 * System.Storage.Put syscall now limits keys to 64 bytes and values to 1024
   bytes (#1221)
 * PUSHA instruction now works with relative code offset (#1226)
 * EQUAL instruction no longer does type conversions, so that values of
   different types are always unequal (#1225)
 * verification scripts now can't use more than 0.5 GAS (#1202)
 * contracts no longer have single entry point, rather they export a set of
   methods with specific offsets. Go smart contract compiler has been changed
   accordingly to add all exported (as in Go) methods to the manifest
   (but with the first letter being lowercased to match NEP-5 expections,
   #1228). Please also refer to examples changes to better see how it affects
   contracts, manifests and configuration files (#1296)
 * native contracts are now called via Neo.Native.Call syscall (#1191)
 * compressed P2P payloads now also contain their uncompressed size (#1212,
   #1255)
 * NEF files now use double SHA256 for checksums (#1203)
 * VM's map keys and contract methods now can only contain valid UTF-8 strings
   (#1198)
 * stack items now can be converted to/from JSON natively (without
   smartcontract.ContractParameters intermediate) which is now used for
   invoke* RPC calls and application execution logs (#1242, #1317)
 * invoking Policy native contracts now requires AllowsStates (to get
   settings) or AllowModifyStates (to change setting) flags (#1254)
 * Transaction now has Signers field unifying Sender (the first Signer) and
   Cosigners, a Signer can have FeeOnly or any other regular witness scope
   (#1184)
 * verification scripts no longer have access to blockchain's state (#1277)
 * governance scheme was changed to delegated committee-based one. The number
   of validators is now specified with ValidatorsCount configuration option,
   standby validators are no longer being registered by default (#867, #1300)
 * Go 1.13+ is now required to build neo-go (#1281)
 * public contract methods now always return some value and this is being
   checked by the VM (#1196, #1331, #1332)
 * runtime interop package now exports triggers as proper constants rather
   than functions (#1299)
 * RPC client no longer has SetWIF/WIF methods that didn't do anything useful
   anyway (#1328)

Improvements:
 * Neo.Crypto.CheckMultisigWithECDsaSecp256r1 syscall is now available via
   crypto.ECDSASecp256r1CheckMultisig interop function (#1175)
 * System.Contract.IsStandard syscall now also checks script's container (#1187)
 * syscalls no longer have allowed triggers limitations (#1205)
 * better testing coverage (#1232, #1318, #1328)
 * getrawmempool RPC call now also supports verbose parameter (#1182)
 * VMState is no longer being stored as a string for application execution
   results (#1236)
 * manifest now contains a list of supported standards (#1204)
 * notifications can't be changed now by a contract after emitting them
   (#1199)
 * it is possible to call other contracts from native contracts now (#1271)
 * getnep5transfers now supports timing parameters (#1289)
 * smartcontract package now has CreateDefaultMultiSigRedeemScript that should
   be used for BFT-compliant "m out of n" multisignature script generation
   (#1300)
 * validators are always sorted now (standby validators were not previously,
   #1300)
 * debug information now contains all file names (#1295)
 * compiler now accepts directory to compile a package, only one file could be
   passed previously (#1295)
 * some old no longer used functions and structures were removed (#1303)
 * contract inspection output was improved for new Neo 3 VM instructions (#1231)
 * ping P2P message handling was changed to trigger block requests (#1326)

Bugs fixed:
 * inability to transfer NEO/GAS from deployed contract's address (#1180)
 * System.Blockchain.GetTransactionFromBlock syscall didn't pick all of its
   arguments from the stack in some error cases (#1187)
 * System.Contract.CallEx syscall didn't properly check call flags (#1187)
 * System.Blockchain.GetContract and System.Contract.Create syscalls returned
   an interop interface instead of plain well-defined structure (#1187)
 * System.Contract.Update syscall's manifest checks were improved, return
   value was fixed (#1187)
 * getnep5balances and getnep5transfers RPC calls now support addresses in
   their parameters (#1188)
 * rare panic during node's shutdown (#1188)
 * System.Runtime.CheckWitness, System.Runtime.GetTime syscalls are only allowed to be called with
   AllowStates flag (#1218)
 * System.Runtime.GasLeft syscall result for test VM mode was wrong (#1218)
 * getrawtransaction RPC call now also returns its VM state after execution
   (#1183)
 * getnep5balances and getnep5transfers RPC calls now correctly work for
   migrated contracts (#1188, #1239)
 * compiler now generates correct code for global variables from multiple
   files (#1240)
 * compiler now correctly supports exported contracts and variables in
   packages (#1154)
 * compiler no longer confuses functions with the same name from different
   packages (#1150)
 * MaxBlockSize policy setting was not enforced (#1254)
 * missing scope check for signers (#1244)
 * compiler now properly supports init() functions (#1253, #1295)
 * getvalidators RPC call now returns zero-length array of validators when
   there are no registered candidates instead of null value (#1300)
 * events were not added to the debug data (#1311, #1315)
 * RPC client's BalanceOf method was lacking account parameter (#1323)
 * VM CLI debugging commands didn't really allow to step through the contract
   (#1328)
 * recovery message decoding created incorrect PrepareRequest payload that
   lead to consensus failures (#1334)

## 0.90.0 "Tantalization" (14 July 2020)

The first Neo 3 compatible release of neo-go! We've targeted to make it
compatible with preview2 release of Neo 3, so it only contains features
available there, but at the same time this makes the node more useful until we
have some more up to date reference version. It's a completely different
network, so almost everything has changed and it's hard to describe it with
the same level of details we usually do (but we'll provide them for subsequent
releases where the changeset is going to be lower in size). Please note that
this is a preview-level release and there won't be long-term support provided
for it, Neo 3 is evolving and the next release won't be compatible with this
one.

Main Neo 3 features in this release:
 * no UTXO
 * native contracts
 * new VM
 * scoped witnesses for transaction
 * updated interop/syscalls set
 * contract manifests
 * more efficient P2P protocol

Things that have also changed:
 * transaction format
 * block format
 * address format
 * wallets
 * RPC protocol
 * notification subsystem
 * executable format output for compiler

Compatibility level of this neo-go release:
 * identical storage changes compared to C# node for 378K blocks of preview2
   testnet
 * debugging info produced is compatible with preview2-compatible neo-debugger
 * running consensus nodes in heterogeneous setup is possible (2 neo-go CNs
   with 2 C# CNs, for example)

Changes specific to neo-go:
 * some CLI parameters like wallet path or RPC endpoint URL have been unified
   across all commands and thus have changed in some of them (refer to CLI
   help for details)
 * as an extension we support post-preview2 cosigners parameter for
   invokefunction RPC calls (see neo-project/neo-modules#260)
 * Go compiler now supports comparisons with nil properly
 * we no longer provide bootstrapping 6k block dump for private networks, you
   have 30000000 GAS right in the genesis block and it's not hard to make use
   of it (see
   [neo-go-sc-wrkshp](https://github.com/nspcc-dev/neo-go-sc-wrkshp) for an
   example of how to use it)
 * we have a conversion tool for your old Neo 2 wallets (`wallet convert`
   command), so you can reuse keys on Neo 3 networks
 * util.Equals interop function may not function the way you expect it to due
   to Neo VM changes, it still is an EQUAL opcode though. This interop may be
   removed in the future.

## Older versions

Please refer to the [master-2.x branch
CHANGELOG](https://github.com/nspcc-dev/neo-go/tree/master-2.x/CHANGELOG.md)
for versions prior to 0.90.0 (that are Neo 2 compatible, unlike 0.90.0+ that
are Neo 3 compatible).
