# NeoGo CLI interface

NeoGo CLI provides all functionality from one binary, so it's used to run
node, create/compile/deploy/invoke/debug smart contracts, run vm and operate
with the wallet. The standard setup assumes that you're running a node as a
separate process and it doesn't provide any CLI of its own, instead it just
makes RPC interface available for you. To perform any actions you invoke NeoGo
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

### Starting a node

To start Neo node on private network use:

```
./bin/neo-go node
```

Or specify a different network with appropriate flag like this:

```
./bin/neo-go node --mainnet
```

By default the node will run in foreground using current standard output for
logging.

### DB import/exports

Node operates using some database as a backend to store blockchain data. NeoGo
allows to dump chain into file from the database (when node is stopped) or to
import blocks from file into the database (also when node is stopped). Use
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
things.

### Wallet management

#### Create wallet

Use `wallet init` command to create new wallet:
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

where "wallet.nep6" is a wallet file name. This wallet will be empty, to
generate a new key pair and add an account for it use `-a` option:
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
                        "isdefault": false
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

or use `wallet create` command to create new account in existing wallet:
```
./bin/neo-go wallet create -w wallet.nep6
Enter the name of the account > Joe Random
Enter passphrase > 
Confirm passphrase >
```

#### Convert Neo Legacy wallets to Neo N3

Use `wallet convert` to update addresses in NEP-6 wallets used with Neo
Legacy. New wallet is specified in `-o` option, it will have the same keys
with Neo N3 addresses.
```
./bin/neo-go wallet convert -w old.nep6 -o new.nep6
```

#### Check wallet contents
`wallet dump` can be used to see wallet contents in more user-friendly way,
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
                        "isdefault": false
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
`wallet export` allows you to export private key in NEP-2 encrypted or WIF
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
need all public keys and one private key to do that. Then you could sign
transactions for this multisignature account with imported key.

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

### NEP-17 token functions

`wallet nep17` contains a set of commands to use for NEP-17 tokens.

#### Token metadata

NEP-17 commands are designed to work with any NEP-17 tokens, but NeoGo needs
some metadata for these tokens to function properly. Native NEO or GAS are
known to NeoGo by default, but other tokens are not. NeoGo can get this
metadata from the specified RPC server, but that's an additional request to
make, so if you care about command processing delay you can import token
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

By default you'll get data for all tokens for the default wallet's
address. You can select non-default address with `-a` flag and/or select token
with `--token` flag (token hash or name can be used as parameter)

#### Transfers

`wallet nep17 transfer` creates a token transfer transaction and pushes it to
the RPC server (or saves to file if it needs to be signed by multiple
parties). For example, transferring 100 GAS looks like this:

```
./bin/neo-go wallet nep17 transfer -w wallet.nep6 -r http://localhost:20332 --to NjEQfanGEXihz85eTnacQuhqhNnA6LxpLp --from NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E --token GAS --amount 100
```

You can omit `--from` parameter (default wallet's address will be used in this
case), you can add `--gas` for extra network fee (raising priority of your
transaction). And you can save transaction to file with `--out` instead of
sending it to the network if it needs to be signed by multiple parties.

One `transfer` invocation creates one transaction, but in case you need to do
many transfers you can save on network fees by doing multiple token moves with
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
automatically distributed with every NEO transfer for NEO owners you still
won't get GAS if you don't do any actions. So the old `wallet claim` command
was updated to be an easier way to do NEO "flipping" when you send a
transaction that transfers all of your NEO to yourself thereby triggering GAS
distribution.

## Conversion utility

NeoGo provides conversion utility command to reverse data, convert script
hashes to/from address, convert data to/from hexadecimal or base64
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

Use `help` command to get more detailed information on all possibilities and
particular commands. Note that this VM is completely disconnected from the
blockchain, so you won't have all interop functionality available for smart
contracts (use test invocations via RPC for that).
