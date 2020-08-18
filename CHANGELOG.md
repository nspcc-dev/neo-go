# Changelog

This document outlines major changes between releases.

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
