<p align="center">
<img src="./.github/neo_color_dark_gopher.png" width="300px" alt="logo">
</p>
<p align="center">
  <b>Go</b> Node and SDK for the <a href="https://neo.org">NEO</a> blockchain.
</p>

<hr />

[![codecov](https://codecov.io/gh/nspcc-dev/neo-go/branch/master/graph/badge.svg)](https://codecov.io/gh/nspcc-dev/neo-go)
[![CircleCI](https://circleci.com/gh/nspcc-dev/neo-go/tree/master.svg?style=svg)](https://circleci.com/gh/nspcc-dev/neo-go/tree/master)
[![Report](https://goreportcard.com/badge/github.com/nspcc-dev/neo-go)](https://goreportcard.com/report/github.com/nspcc-dev/neo-go)
[![GoDoc](https://godoc.org/github.com/nspcc-dev/neo-go?status.svg)](https://godoc.org/github.com/nspcc-dev/neo-go)
![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/nspcc-dev/neo-go?sort=semver)
![License](https://img.shields.io/github/license/nspcc-dev/neo-go.svg?style=popout)

# Overview

This project aims to be a full port of the original C# [Neo project](https://github.com/neo-project).
A complete toolkit for the NEO blockchain, including:

- [Consensus node](docs/consensus.md)
- [RPC node & client](docs/rpc.md)
- [CLI tool](docs/cli.md)
- [Smart contract compiler](docs/compiler.md)
- [NEO virtual machine](docs/vm.md)
- [Smart contract examples](examples/README.md)
- [Oracle service](docs/oracle.md)
- [State validation service](docs/stateroots.md)

This branch (**master**) is under active development now (read: won't work
out of the box) and aims to be compatible with Neo 3. For the current stable
version compatible with Neo 2 please refer to the [**master-2.x**
branch](https://github.com/nspcc-dev/neo-go/tree/master-2.x) and releases
before 0.80.0 (**0.7X.Y** track). Releases starting from 0.90.0 contain
Neo 3 code (0.90.0 being compatible with Neo 3 preview2).

# Getting started

## Installation

NeoGo is distributed as a single binary that includes all the functionality
provided (but smart contract compiler requires Go compiler to operate). You
can grab it from [releases
page](https://github.com/nspcc-dev/neo-go/releases), use a Docker image (see
[Docker Hub](https://hub.docker.com/r/nspccdev/neo-go) for various releases of
NeoGo, `:latest` points to the latest release) or build yourself.

### Building

To build NeoGo you need Go 1.14+ and `make`:

```
make build
```

The resulting binary is `bin/neo-go`.

## Running a node

A node needs to connect to some network, either local one (usually referred to
as `privnet`) or public (like `mainnet` or `testnet`). Network configuration
is stored in a file and NeoGo allows you to store multiple files in one
directory (`./config` by default) and easily switch between them using network
flags.

To start Neo node on private network use:

```
./bin/neo-go node
```

Or specify a different network with appropriate flag like this:

```
./bin/neo-go node --mainnet
```

Available network flags:
- `--mainnet, -m`
- `--privnet, -p`
- `--testnet, -t`

### Docker

By default the `CMD` is set to run a node on `privnet`, so to do this simply run:

```bash
docker run -d --name neo-go -p 20332:20332 -p 20331:20331 nspccdev/neo-go
```

Which will start a node on `privnet` and expose node's ports `20332` (P2P
protocol) and `20331` (JSON-RPC server).

### Importing mainnet/testnet dump files

If you want to jump-start your mainnet or testnet node with [chain archives
provided by NGD](https://sync.ngd.network/) follow these instructions (when
they'd be available for 3.0 networks):
```
$ wget .../chain.acc.zip # chain dump file
$ unzip chain.acc.zip
$ ./bin/neo-go db restore -m -i chain.acc # for testnet use '-t' flag instead of '-m'
```

The process differs from the C# node in that block importing is a separate
mode, after it ends the node can be started normally.

## Running a private network

Refer to [consensus node documentation](docs/consensus.md).

## Smart contract development

Please refer to [neo-go smart contract development
workshop](https://github.com/nspcc-dev/neo-go-sc-wrkshp) that shows some
simple contracts that can be compiled/deployed/run using neo-go compiler, SDK
and private network. For details on how Go code is translated to Neo VM
bytecode and what you can and can not do in smart contract please refer to the
[compiler documentation](docs/compiler.md).

Refer to [examples](examples/README.md) for more NEO smart contract examples 
written in Go.

## Wallets

NeoGo differs substantially from C# implementation in its approach to
wallets. NeoGo wallet is just a
[NEP-6](https://github.com/neo-project/proposals/blob/68398d28b6932b8dd2b377d5d51bca7b0442f532/nep-6.mediawiki)
file that is used by CLI commands to sign various things. There is no database
behind it, the blockchain is the database and CLI commands use RPC to query
data from it. At the same time it's not required to open the wallet on RPC
node to perform various actions (unless your node is providing some service
for the network like consensus or oracle nodes).

# Developer notes
Nodes have such features as [Prometheus](https://prometheus.io/docs/guides/go-application) and 
[Pprof](https://golang.org/pkg/net/http/pprof/) in order to have additional information about them for debugging.

How to configure Prometheus or Pprof:
In `config/protocol.*.yml` there is 
```
  Prometheus:
    Enabled: true
    Port: 2112
```
where you can switch on/off and define port. Prometheus is enabled and Pprof is disabled by default.

## Contributing

Feel free to contribute to this project after reading the
[contributing guidelines](CONTRIBUTING.md).

Before starting to work on a certain topic, create an new issue first,
describing the feature/topic you are going to implement.

# Contact

- [@roman-khimov](https://github.com/roman-khimov) on GitHub
- [@AnnaShaleva](https://github.com/AnnaShaleva) on GitHub
- [@fyrchik](https://github.com/fyrchik) on Github
- Reach out to us on the [NEO Discord](https://discordapp.com/invite/R8v48YA) channel

# License

- Open-source [MIT](LICENSE.md)
