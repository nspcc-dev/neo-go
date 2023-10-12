<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./.github/logo_dark.png">
    <source media="(prefers-color-scheme: light)" srcset="./.github/logo_light.png">
    <img src="./.github/logo_light.png"  width="300px" alt="NeoGo logo">
  </picture>
</p>
<p align="center">
  <b>Go</b> Node and SDK for the <a href="https://neo.org">Neo</a> blockchain.
</p>

<hr />

[![codecov](https://codecov.io/gh/nspcc-dev/neo-go/branch/master/graph/badge.svg)](https://codecov.io/gh/nspcc-dev/neo-go)
[![GithubWorkflows Tests](https://github.com/nspcc-dev/neo-go/actions/workflows/tests.yml/badge.svg)](https://github.com/nspcc-dev/neo-go/actions/workflows/tests.yml)
[![Report](https://goreportcard.com/badge/github.com/nspcc-dev/neo-go)](https://goreportcard.com/report/github.com/nspcc-dev/neo-go)
[![GoDoc](https://godoc.org/github.com/nspcc-dev/neo-go?status.svg)](https://godoc.org/github.com/nspcc-dev/neo-go)
![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/nspcc-dev/neo-go?sort=semver)
![License](https://img.shields.io/github/license/nspcc-dev/neo-go.svg?style=popout)

# Overview

NeoGo is a complete platform for distributed application development built on
top of and compatible with the [Neo project](https://github.com/neo-project).
This includes, but not limited to (see documentation for more details):

- [Consensus node](docs/consensus.md)
- [RPC node & client](docs/rpc.md)
- [CLI tool](docs/cli.md)
- [Smart contract compiler](docs/compiler.md)
- [Neo virtual machine](docs/vm.md)
- [Smart contract examples](examples/README.md)
- [Oracle service](docs/oracle.md)
- [State validation service](docs/stateroots.md)

The protocol implemented here is Neo N3-compatible, however you can also find
an implementation of the Neo Legacy protocol in the [**master-2.x**
branch](https://github.com/nspcc-dev/neo-go/tree/master-2.x) and releases
before 0.80.0 (**0.7X.Y** track).

# Getting started

## Installation

NeoGo is distributed as a single binary that includes all the functionality
provided (but smart contract compiler requires Go compiler to operate). You
can grab it from [releases
page](https://github.com/nspcc-dev/neo-go/releases), use a Docker image (see
[Docker Hub](https://hub.docker.com/r/nspccdev/neo-go) for various releases of
NeoGo, `:latest` points to the latest release) or build yourself.

### Building

Building NeoGo requires Go 1.19+ and `make`:

```
make
```

The resulting binary is `bin/neo-go`. Notice that using some random revision
from the `master` branch is not recommended (it can have any number of
incompatibilities and bugs depending on the development stage), please use
tagged released versions.

#### Building on Windows

To build NeoGo on Windows platform we recommend you to install `make` from [MinGW
package](https://osdn.net/projects/mingw/). Then, you can build NeoGo with:

```
make
```

The resulting binary is `bin/neo-go.exe`.

## Running a node

A node needs to connect to some network, either local one (usually referred to
as `privnet`) or public (like `mainnet` or `testnet`). Network configuration
is stored in a file and NeoGo allows you to store multiple files in one
directory (`./config` by default) and easily switch between them using network
flags.

To start Neo node on a private network, use:

```
./bin/neo-go node
```

Or specify a different network with an appropriate flag like this:

```
./bin/neo-go node --mainnet
```

Available network flags:
- `--mainnet, -m`
- `--privnet, -p`
- `--testnet, -t`

To run a consensus/committee node, refer to [consensus
documentation](docs/consensus.md).

If you're running a node on Windows, please turn off or configure Windows
Firewall appropriately (allowing inbound connections to the P2P port).

### Docker

By default, the `CMD` is set to run a node on `privnet`. So, to do this, simply run:

```bash
docker run -d --name neo-go -p 20332:20332 -p 20331:20331 nspccdev/neo-go
```

Which will start a node on `privnet` and expose node's ports `20332` (P2P
protocol) and `20331` (JSON-RPC server).

### Importing mainnet/testnet dump files

If you want to jump-start your mainnet or testnet node with [chain archives
provided by NGD](https://sync.ngd.network/), follow these instructions:
```
$ wget .../chain.acc.zip # chain dump file
$ unzip chain.acc.zip
$ ./bin/neo-go db restore -m -i chain.acc # for testnet use '-t' flag instead of '-m'
```

The process differs from the C# node in that block importing is a separate
mode. After it ends, the node can be started normally.

## Running a private network

Refer to [consensus node documentation](docs/consensus.md).

## Smart contract development

Please refer to [NeoGo smart contract development
workshop](https://github.com/nspcc-dev/neo-go-sc-wrkshp) that shows some
simple contracts that can be compiled/deployed/run using NeoGo compiler, SDK
and a private network. For details on how Go code is translated to Neo VM
bytecode and what you can and can not do in a smart contract, please refer to the
[compiler documentation](docs/compiler.md).

Refer to [examples](examples/README.md) for more Neo smart contract examples
written in Go.

## Wallets

NeoGo wallet is just a
[NEP-6](https://github.com/neo-project/proposals/blob/68398d28b6932b8dd2b377d5d51bca7b0442f532/nep-6.mediawiki)
file that is used by CLI commands to sign various things. CLI commands are not
a direct part of the node, but rather a part of the NeoGo binary, their
implementations use RPC to query data from the blockchain and perform any
required actions. It's not required to open a wallet on an RPC node (unless
your node provides some service for the network like consensus or oracle nodes
do).

## Monitoring
NeoGo provides [Prometheus](https://prometheus.io/docs/guides/go-application) and
[Pprof](https://golang.org/pkg/net/http/pprof/) services that can be enabled
in the node in order to provide additional monitoring and debugging data.

Configuring any of the two services is easy, add the following section (`Pprof`
instead of `Prometheus` if you need that) to the respective `config/protocol.*.yml`:
```
  Prometheus:
    Enabled: true
    Addresses:
      - ":2112"
```
where you can switch on/off and define port. Prometheus is enabled and Pprof is disabled by default.

## Contributing

Feel free to contribute to this project after reading the
[contributing guidelines](CONTRIBUTING.md).

Before starting to work on a certain topic, create a new issue first
describing the feature/topic you are going to implement.

# Contact

- [@roman-khimov](https://github.com/roman-khimov) on GitHub
- [@AnnaShaleva](https://github.com/AnnaShaleva) on GitHub
- [@fyrchik](https://github.com/fyrchik) on GitHub
- Reach out to us on the [Neo Discord](https://discordapp.com/invite/R8v48YA) channel

# License

- Open-source [MIT](LICENSE.md)
