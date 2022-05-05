# NeoGo CLI interface

NeoGo CLI provides all functionality from one binary. It's used to run
a node, create/compile/deploy/invoke/debug smart contracts, run vm and operate
with a wallet. Standard setup assumes that you run a node as a
separate process, and it doesn't provide any CLI of its own. Instead, it just
makes RPC interface available for you. To perform any actions, you invoke NeoGo
as a client that connects to this RPC node and does things you want it to do
(like transferring some NEP-17 asset).

All CLI commands have corresponding help messages, use `--help` option to get
them, for example:
```
./bin/neo-go db --help
```

## Running node

Use `node` command to run a NeoGo node, it will be configured using a YAML
file that contains network parameters as well as node settings.

### Configuration

All config files are located in `./config` and they are differentiated according to the network type:
- `protocol.mainnet.yml` belongs to `--mainnet` network mode (`-m` short option)
- `protocol.privnet.yml` belongs to `--privnet` network mode (`-p` short
  option) and is used by default
- `protocol.testnet.yml` belongs to `--testnet` network mode (`-t` short option)
- `protocol.unit_testnet.yml` is used by unit tests

If you want to use some non-default configuration directory path, specify
`--config-path` flag:

`./bin/neo-go node --config-path /user/yourConfigPath`

The file loaded is chosen automatically depending on network mode flag.

Refer to the [node configuration documentation](./node-configuration.md) for
detailed configuration file description.

### Starting a node

To start Neo node on private network, use:

```
./bin/neo-go node
```

Or specify a different network with an appropriate flag like this:

```
./bin/neo-go node --mainnet
```

By default, the node will run in the foreground using current standard output for
logging.


### Node synchronization

Most of the services (state validation, oracle, consensus and RPC if
configured with `StartWhenSynchronized` option) are only started after the
node is completely synchronizaed because running them before that is either
pointless or even dangerous. The node considers itself to be fully
synchronized with the network if it has more than `MinPeers` neighbours and if
at least 2/3 of them are known to have a height less than or equal to the
current height of the node.

### Restarting node services

To restart some node services without full node restart, send the SIGHUP 
signal. List of the services to be restarted on SIGHUP receiving:

| Service | Action |
| --- | --- |
| RPC server | Restarting with the old configuration and updated TLS certificates |

### DB import/exports

Node operates using some database as a backend to store blockchain data. NeoGo
allows to dump chain into a file from the database (when node is stopped) or to
import blocks from a file into the database (also when node is stopped). Use
`db` command for that.

## Smart contracts

Use `contract` command to create/compile/deploy/invoke/debug smart contracts,
see [compiler documentation](compiler.md).

## Wallet operations

`wallet` command provides interface for all operations requiring a wallet
(except contract deployment and invocations that are done via `contract
deploy` and `contract invokefunction`). Wallet management (creating wallet,
adding addresses/keys to it) is available there as well as wallet-related
functions like NEP-17 transfers, NEO votes, multi-signature signing and other
things. For all commands requiring read-only wallet (like `dump-keys`) a
special `-` path can be used to read the wallet from the standard input.

### Wallet management

#### Create wallet

Use `wallet init` command to create a new wallet:
```
./bin/neo-go wallet init -w wallet.nep6

{
        "version": "3.0",
        "accounts": [],
        "scrypt": {
                "n": 16384,
                "r": 8,
                "p": 8
        },
        "extra": {
                "Tokens": null
        }
 }

wallet successfully created, file location is wallet.nep6
```

where "wallet.nep6" is a wallet file name. This wallet will be empty. To
generate a new key pair and add an account for it, use `-a` option:
```
./bin/neo-go wallet init -w wallet.nep6 -a
Enter the name of the account > Name
Enter passphrase > 
Confirm passphrase > 

{
        "version": "3.0",
        "accounts": [
                {
                        "address": "NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E",
                        "key": "6PYL2UrC11nWFJWSLiqsPKCNm9u4zr4ttX1ZbV9f2fLDqXsePioVxEsYdg",
                        "label": "Name",
                        "contract": {
                                "script": "DCEDzs1j19gSDDsZTDsogN1Kr+FHXFfkDIUoctcwVhUlgUBBdHR2qg==",
                                "parameters": [
                                        {
                                                "name": "parameter0",
                                                "type": "Signature"
                                        }
                                ],
                                "deployed": false
                        },
                        "lock": false,
                        "isDefault": false
                }
        ],
        "scrypt": {
                "n": 16384,
                "r": 8,
                "p": 8
        },
        "extra": {
                "Tokens": null
        }
 }

wallet successfully created, file location is wallet.nep6
```

or use `wallet create` command to create a new account in an existing wallet:
```
./bin/neo-go wallet create -w wallet.nep6
Enter the name of the account > Joe Random
Enter passphrase > 
Confirm passphrase >
```

#### Convert Neo Legacy wallets to Neo N3

Use `wallet convert` to update addresses in NEP-6 wallets used with Neo
Legacy. New wallet is specified in `-o` option, it will have the same keys
with Neo N3 addresses (notice that it doesn't do anything to your assets, it
just allows to reuse the old key on N3 network).
```
./bin/neo-go wallet convert -w old.nep6 -o new.nep6
```

#### Check wallet contents
`wallet dump` can be used to see wallet contents in a more user-friendly way,
its output is the same NEP-6 JSON, but better formatted. You can also decrypt
keys at the same time with `-d` option (you'll be prompted for password):
```
./bin/neo-go wallet dump -w wallet.nep6 -d
Enter wallet password > 

{
        "version": "3.0",
        "accounts": [
                {
                        "address": "NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E",
                        "key": "6PYL2UrC11nWFJWSLiqsPKCNm9u4zr4ttX1ZbV9f2fLDqXsePioVxEsYdg",
                        "label": "Name",
                        "contract": {
                                "script": "DCEDzs1j19gSDDsZTDsogN1Kr+FHXFfkDIUoctcwVhUlgUBBdHR2qg==",
                                "parameters": [
                                        {
                                                "name": "parameter0",
                                                "type": "Signature"
                                        }
                                ],
                                "deployed": false
                        },
                        "lock": false,
                        "isDefault": false
                }
        ],
        "scrypt": {
                "n": 16384,
                "r": 8,
                "p": 8
        },
        "extra": {
                "Tokens": null
        }
 }
```

You can also get public keys for addresses stored in your wallet with `wallet
dump-keys` command:
```
./bin/neo-go wallet dump-keys -w wallet.nep6
NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E (simple signature contract):
03cecd63d7d8120c3b194c3b2880dd4aafe1475c57e40c852872d7305615258140
```

#### Private key export
`wallet export` allows you to export a private key in NEP-2 encrypted or WIF
(unencrypted) form (`-d` flag).
```
$ ./bin/neo-go wallet export -w wallet.nep6 -d NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E
Enter password > 
KyswN8r48dhsvyQJVy97RWnZmKgYLrXv9mCL81Kb4vAagZiCsePv
```

#### Private key import
You can import NEP-2 or WIF private key along with verification contract (if
it's non-standard):
```
./bin/neo-go wallet import --wif KwYgW8gcxj1JWJXhPSu4Fqwzfhp5Yfi42mdYmMa4XqK7NJxXUSK7 -w wallet.nep6
Provided WIF was unencrypted. Wallet can contain only encrypted keys.
Enter the name of the account > New Account
Enter passphrase > 
Confirm passphrase >
```

#### Special accounts
Multisignature accounts can be imported with `wallet import-multisig`, you'll
need all public keys and one private key to do that. Then, you could sign
transactions for this multisignature account with the imported key.

`wallet import-deployed` can be used to create wallet accounts for deployed
contracts. They also can have WIF keys associated with them (in case your
contract's `verify` method needs some signature).

### Neo voting
`wallet candidate` provides commands to register or unregister a committee
(and therefore validator) candidate key:
```
./bin/neo-go wallet candidate register -a NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E -w wallet.json -r http://localhost:20332
```

You can also vote for candidates if you own NEO:
```
./bin/neo-go wallet candidate vote -a NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E -w wallet.json -r http://localhost:20332 -c 03cecd63d7d8120c3b194c3b2880dd4aafe1475c57e40c852872d7305615258140
```

Do not provide candidate argument to perform unvoting:
```
./bin/neo-go wallet candidate vote -a NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E -w wallet.json -r http://localhost:20332
```

### Getting data from chain

#### Node height/validated height
`query height` returns the latest block and validated state height:
```
$ ./bin/neo-go query height -r http://localhost:20332
Latest block: 11926
Validated state: 11926
```

#### Transaction status
`query tx` provides convenient wrapper over RPC calls to query transaction status.
```
./bin/neo-go query tx --rpc-endpoint http://localhost:20332 aaf87628851e0c03ee086ff88596bc24de87082e9e5c73d75bb1c740d1d68088
Hash:			aaf87628851e0c03ee086ff88596bc24de87082e9e5c73d75bb1c740d1d68088
OnChain:		true
BlockHash:		fabcd46e93b8f4e1bc5689e3e0cc59704320494f7a0265b91ae78b4d747ee93b
Success:		true
```
`OnChain` is true if the transaction has been included in the block; and `Success` is true
if it has been executed successfully.

#### Committee members
`query commitee` returns a list of current committee members:
```
$ ./bin/neo-go query committee -r http://localhost:20332
03009b7540e10f2562e5fd8fac9eaec25166a58b26e412348ff5a86927bfac22a2
030205e9cefaea5a1dfc580af20c8d5aa2468bb0148f1a5e4605fc622c80e604ba
0207da870cedb777fceff948641021714ec815110ca111ccc7a54c168e065bda70
02147c1b1d5728e1954958daff2f88ee2fa50a06890a8a9db3fa9e972b66ae559f
0214baf0ceea3a66f17e7e1e839ea25fd8bed6cd82e6bb6e68250189065f44ff01
03184b018d6b2bc093e535519732b3fd3f7551c8cffaf4621dd5a0b89482ca66c9
0231edee3978d46c335e851c76059166eb8878516f459e085c0dd092f0f1d51c21
023e9b32ea89b94d066e649b124fd50e396ee91369e8e2a6ae1b11c170d022256d
03408dcd416396f64783ac587ea1e1593c57d9fea880c8a6a1920e92a259477806
035056669864feea401d8c31e447fb82dd29f342a9476cfd449584ce2a6165e4d7
025831cee3708e87d78211bec0d1bfee9f4c85ae784762f042e7f31c0d40c329b8
026328aae34f149853430f526ecaa9cf9c8d78a4ea82d08bdf63dd03c4d0693be6
0370c75c54445565df62cfe2e76fbec4ba00d1298867972213530cae6d418da636
03840415b0a0fcf066bcc3dc92d8349ebd33a6ab1402ef649bae00e5d9f5840828
03957af9e77282ae3263544b7b2458903624adc3f5dee303957cb6570524a5f254
02a7834be9b32e2981d157cb5bbd3acb42cfd11ea5c3b10224d7a44e98c5910f1b
02ba2c70f5996f357a43198705859fae2cfea13e1172962800772b3d588a9d4abd
03c609bea5a4825908027e4ab217e7efc06e311f19ecad9d417089f14927a173d5
02c69a8d084ee7319cfecf5161ff257aa2d1f53e79bf6c6f164cff5d94675c38b3
02cf9dc6e85d581480d91e88e8cbeaa0c153a046e89ded08b4cefd851e1d7325b5
03d84d22b8753cf225d263a3a782a4e16ca72ef323cfde04977c74f14873ab1e4c
```

#### Candidate/voting data
`query candidates` returns all current candidates, number of votes for them
and their committee/consensus status:
```
$ ./bin/neo-go query candidates -r http://localhost:20332
Key                                                                 Votes    Committee  Consensus
03009b7540e10f2562e5fd8fac9eaec25166a58b26e412348ff5a86927bfac22a2  2000000  true       true
030205e9cefaea5a1dfc580af20c8d5aa2468bb0148f1a5e4605fc622c80e604ba  2000000  true       true
0214baf0ceea3a66f17e7e1e839ea25fd8bed6cd82e6bb6e68250189065f44ff01  2000000  true       true
023e9b32ea89b94d066e649b124fd50e396ee91369e8e2a6ae1b11c170d022256d  2000000  true       true
03408dcd416396f64783ac587ea1e1593c57d9fea880c8a6a1920e92a259477806  2000000  true       true
02a7834be9b32e2981d157cb5bbd3acb42cfd11ea5c3b10224d7a44e98c5910f1b  2000000  true       true
02ba2c70f5996f357a43198705859fae2cfea13e1172962800772b3d588a9d4abd  2000000  true       true
025664cef0abcba7787ad5fb12f3af31c5cdc7a479068aa2ad8ee78804768bffe9  1000000  false      false
03650a684461a64bf46bee561d9981a4c57adc6ccbd3a9512b83701480b30218ab  1000000  false      false
026a10aa2b4d7639c5deafa4ff081467db10b5d00432749a2a5ee1d2bfed23e1c0  1000000  false      false
02d5786a9214a8a3f1757d7596fd10f5241205e2c0d68362f4766579bac6189249  1000000  false      false
033d8e35f8cd9a33852280b6d93093c7292ed5ce90d90f149fa2da50ba6168dfce  100000   false      false
0349c7ef0b4aaf181f0a3e1350c527b136cc5b42498cb83ab8880c05ed95167e1c  100000   false      false
035b4f9be2b853e06eb5a09c167e038b96b4804235961510423252f2ee3dbba583  100000   false      false
027e459b264b6f7e325ab4b0bb0fa641081fb68517fd613ebd7a94cb79d3081e4f  100000   false      false
0288cad442a877960c76b4f688f4be30f768256d9a3da2492b0180b91243918b4f  100000   false      false
02a40c552798f79636095817ec88924fc6cb7094e5a3cb059a9b3bc91ea3bf0d3d  100000   false      false
02db79e69c518ae9254e314b6f5f4b63e914cdd4b2574dc2f9236c01c1fc1d8973  100000   false      false
02ec143f00b88524caf36a0121c2de09eef0519ddbe1c710a00f0e2663201ee4c0  100000   false      false
03d8d58d2257ca6cb14522b76513d4783f7d481801695893794c2186515c6de76f  0        false      false
```

#### Voter data
`query voter` returns additional data about NEO holder: the amount of NEO he has,
the candidate it voted for (if any) and the block number of the last transactions
involving NEO on this account:
```
$ ./bin/neo-go query voter -r http://localhost:20332 Nj91C8TxQSxW1jCE1ytFre6mg5qxTypg1Y
        Voted: 0214baf0ceea3a66f17e7e1e839ea25fd8bed6cd82e6bb6e68250189065f44ff01 (Nj91C8TxQSxW1jCE1ytFre6mg5qxTypg1Y)
        Amount : 2000000
        Block: 3970
```

### NEP-17 token functions

`wallet nep17` contains a set of commands to use for NEP-17 tokens.

#### Token metadata

NEP-17 commands are designed to work with any NEP-17 tokens, but NeoGo needs
some metadata for these tokens to function properly. Native NEO or GAS are
known to NeoGo by default, but other tokens are not. NeoGo can get this
metadata from the specified RPC server, but that's an additional request to
make. So, if you care about command processing delay, you can import token
metadata into the wallet with `wallet nep17 import` command. It'll be stored
in the `extra` section of the wallet.
```
./bin/neo-go wallet nep17 import -w wallet.nep6 -r http://localhost:20332 -t abcdefc189f30098b0ba6a2eb90b3a925800ffff
```

You can later see what token data you have in your wallet with `wallet nep17
info` command and remove tokens you don't need with `wallet nep17 remove`.

#### Balance
Getting balance is easy:
```
./bin/neo-go wallet nep17 balance -w /etc/neo-go/wallet.json -r http://localhost:20332
```

By default, you'll get data for all tokens for the default wallet's
address. You can select non-default address with `-a` flag and/or select token
with `--token` flag (token hash or name can be used as parameter).

#### Transfers

`wallet nep17 transfer` creates a token transfer transaction and pushes it to
the RPC server (or saves to file if it needs to be signed by multiple
parties). For example, transferring 100 GAS looks like this:

```
./bin/neo-go wallet nep17 transfer -w wallet.nep6 -r http://localhost:20332 --to NjEQfanGEXihz85eTnacQuhqhNnA6LxpLp --from NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E --token GAS --amount 100
```

You can omit `--from` parameter (default wallet's address will be used in this
case), you can add `--gas` for extra network fee (raising priority of your
transaction). And you can save the transaction to a file with `--out` instead of
sending it to the network if it needs to be signed by multiple parties.

To add optional `data` transfer parameter, specify `data` positional argument
after all required flags. Refer to `wallet nep17 transfer --help` command
description for details.

One `transfer` invocation creates one transaction. In case you need to do
many transfers, you can save on network fees by doing multiple token moves with
one transaction by using `wallet nep17 multitransfer` command. It can transfer
things from one account to many, its syntax differs from `transfer` in that
you don't have `--token`, `--to` and `--amount` options, but instead you can
specify multiple "token:addr:amount" sets after all other options. The same
transfer as above can be done with `multitransfer` by doing this:
```
./bin/neo-go wallet nep17 multitransfer -w wallet.nep6 -r http://localhost:20332 --from NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E GAS:NjEQfanGEXihz85eTnacQuhqhNnA6LxpLp:100
```

#### GAS claims

While Neo N3 doesn't have any notion of "claim transaction" and has GAS
automatically distributed with every NEO transfer for NEO owners, you still
won't get GAS if you don't do any actions. So the old `wallet claim` command
was updated to be an easier way to do NEO "flipping" when you send a
transaction that transfers all of your NEO to yourself thereby triggering GAS
distribution.

### NEP-11 token functions

`wallet nep11` contains a set of commands to use for NEP-11 tokens. Token
metadata related commands (`info`, `import` and `remove`) works the same way as
for NEP-17 tokens. The syntax of other commands is very similar to NEP-17
commands with the following adjustments.

#### Balance

Specify token ID via `--id` flag to call divisible NEP-11 `balanceOf` method:

```
./bin/neo-go wallet nep11 balance -w /etc/neo-go/wallet.json --token 67ecb7766dba4acf7c877392207984d1b4d15731 --id R5OREI5BU+Uyd23/MuV/xzI3F+Q= -r http://localhost:20332
```

By default, no token ID specified, i.e. common `balanceOf` method is called.

#### Transfers

Specify token ID via `--id` flag to transfer NEP-11 token. Specify the amount to
transfer divisible NEP-11 token:

```
./bin/neo-go wallet nep11 transfer -w wallet.nep6 -r http://localhost:20332 --to NjEQfanGEXihz85eTnacQuhqhNnA6LxpLp --from NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E --token 67ecb7766dba4acf7c877392207984d1b4d15731 --id R5OREI5BU+Uyd23/MuV/xzI3F+Q= --amount 5
```

By default, no amount is specified, i.e. the whole token is transferred for
non-divisible tokens and 100% of the token is transferred if there is only one
owner of this token for divisible tokens.

Unlike NEP-17 tokens functionality, `multitransfer` command is currently not
supported on NEP-11 tokens.

#### Tokens Of

To print token IDs owned by the specified owner, use `tokensOf` command with
`--token` and `--address` flags:

```
./bin/neo-go wallet nep11 tokensOf -r http://localhost:20332 --token 67ecb7766dba4acf7c877392207984d1b4d15731 --address NbrUYaZgyhSkNoRo9ugRyEMdUZxrhkNaWB
```

#### Owner Of

For non-divisible NEP-11 tokens only. To print owner of non-divisible NEP-11 token
use `ownerOf` method, specify token hash via `--token` flag and token ID via
`--id` flag:

```
./bin/neo-go wallet nep11 ownerOf -r http://localhost:20332 --token 67ecb7766dba4acf7c877392207984d1b4d15731 --id R5OREI5BU+Uyd23/MuV/xzI3F+Q=
```

#### Optional methods

##### 1. Properties

If NEP-11 token supports optional `properties` method, specify token hash via
`--token` flag and token ID via `--id` flag to print properties:

```
./bin/neo-go wallet nep11 properties -r http://localhost:20332 --token 67ecb7766dba4acf7c877392207984d1b4d15731 --id 7V5gjT2WwjP3pBCQMKGMfyZsp/w=
```

##### 2. Tokens

If NEP-11 token supports optional `tokens` method, specify token hash via
`--token` flag to print the list of token IDs minted by the specified NFT:

```
./bin/neo-go wallet nep11 tokens -r http://localhost:20332 --token 67ecb7766dba4acf7c877392207984d1b4d15731
```

## Conversion utility

NeoGo provides conversion utility command to reverse data, convert script
hashes to/from address, convert public keys to hashes/addresses, convert data to/from hexadecimal or base64
representation. All of this is done by a single `util convert` command like
this:
```
$ ./bin/neo-go util convert deee79c189f30098b0ba6a2eb90b3a9258a6c7ff
BE ScriptHash to Address        NgEisvCqr2h8wpRxQb7bVPWUZdbVCY8Uo6
LE ScriptHash to Address        NjEQfanGEXihz85eTnacQuhqhNnA6LxpLp
Hex to String                           "\xde\xeey\xc1\x89\xf3\x00\x98\xb0\xbaj.\xb9\v:\x92X\xa6\xc7\xff"
Hex to Integer                          -1256651697634605895065630637163547727407485218
Swap Endianness                         ffc7a658923a0bb92e6abab09800f389c179eede
Base64 to String                        "u\xe7\x9e\xef\xd75\xf3\xd7\xf7\xd3O|oF\xda魞o\xdd\x1bݯv\xe7ƺs\xb7\xdf"
Base64 to BigInteger            -222811771454869584930239486728381018152491835874567723544539443409000587
String to Hex                           64656565373963313839663330303938623062613661326562393062336139323538613663376666
String to Base64                        ZGVlZTc5YzE4OWYzMDA5OGIwYmE2YTJlYjkwYjNhOTI1OGE2YzdmZg==
```

## VM CLI
There is a VM CLI that you can use to load/analyze/run/step through some code:

```
./bin/neo-go vm
```

Some basic commands available there:

- `loadgo` -- loads smart contract `NEO-GO-VM > loadgo TestContract/main.go`
- `ops` -- show the opcodes of currently loaded contract
- `run` -- executes currently loaded contract

Use `help` command to get more detailed information on all options and
particular commands. Note that this VM is completely disconnected from the
blockchain, so you won't have all interop functionality available for smart
contracts (use test invocations via RPC for that).
