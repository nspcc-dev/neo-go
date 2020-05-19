<p align="center">
<img src="./.github/neo_color_dark_gopher.png" width="300px" alt="logo">
</p>
<p align="center">
  <b>Go</b> Node and SDK for the <a href="https://neo.org">NEO</a> blockchain.
</p>

<hr />

[![codecov](https://codecov.io/gh/nspcc-dev/neo-go/branch/master-2.x/graph/badge.svg)](https://codecov.io/gh/nspcc-dev/neo-go/branch/master-2.x)
[![CircleCI](https://circleci.com/gh/nspcc-dev/neo-go/tree/master-2.x.svg?style=svg)](https://circleci.com/gh/nspcc-dev/neo-go/tree/master-2.x)
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

This branch (**master-2.x**) is a stable version of the project compatible
with Neo 2, it only receives bug fixes and minor updates. For the Neo 3
development version please refer to the [**master**
branch](https://github.com/nspcc-dev/neo-go/tree/master) and releases
after 0.90.0. Releases before 0.80.0 (**0.7X.Y** track) are made from this
branch and only contain Neo 2 code.

# Getting started

## Installation

Go: 1.12+

Install dependencies.

`neo-go` uses [GoModules](https://github.com/golang/go/wiki/Modules) as dependency manager:

```
make deps
```

## How to setup a node

### Docker

Each tagged build is built to docker hub and the `:latest` tag pointing at the latest tagged build.

By default the `CMD` is set to run a node on `testnet`, so to do this simply run:

```bash
 docker run -d --name neo-go -p 20332:20332 -p 20333:20333 cityofzion/neo-go
```

Which will start a node on `testnet` and expose the nodes port `20333` and `20332` for the `JSON-RPC` server.


### Building

Build the **neo-go** CLI:

```
make build
```

### Running

Quick start a NEO node on the private network. This requires the [neo-privatenet](https://hub.docker.com/r/cityofzion/neo-privatenet/) Docker image running on your machine.

```
make run
```

To run the binary directly:

```
./bin/neo-go node
```

By default the node will run on the `private network`, to change his:

```
./bin/neo-go node --mainnet
```

Available network flags:
- `--mainnet, -m`
- `--privnet, -p`
- `--testnet, -t`

#### Importing mainnet/testnet dump files

If you want to jump-start your mainnet or testnet node with [chain archives
provided by NGD](https://sync.ngd.network/) follow these instructions:
```
$ wget .../chain.acc.zip # chain dump file
$ unzip chain.acc.zip
$ ./bin/neo-go db restore -m -i chain.acc # for testnet use '-t' flag instead of '-m'
```

The process differs from the C# node in that block importing is a separate
mode, after it ends the node can be started normally.

## Smart contract development

Please refer to [neo-go smart contract development
workshop](https://github.com/nspcc-dev/neo-go-sc-wrkshp) that shows some
simple contracts that can be compiled/deployed/run using neo-go compiler, SDK
and private network. For details on how Go code is translated to Neo VM
bytecode and what you can and can not do in smart contract please refer to the
[compiler documentation](docs/compiler.md).

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
- [@fyrchik](https://github.com/fyrchik) on Github
- Reach out to us on the [NEO Discord](https://discordapp.com/invite/R8v48YA) channel

# License

- Open-source [MIT](LICENSE.md)
