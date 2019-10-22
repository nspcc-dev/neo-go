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

This project aims to be a full port of the original C# [NEO project](https://github.com/neo-project).
A complete toolkit for the NEO blockchain, including:

- Consensus node (WIP)
- [RPC node & client](https://github.com/nspcc-dev/neo-go/tree/master/docs/rpc.md)
- [CLI tool](https://github.com/nspcc-dev/neo-go/blob/master/docs/cli.md)
- [Smart contract compiler](https://github.com/nspcc-dev/neo-go/blob/master/docs/compiler.md)
- [NEO virtual machine](https://github.com/nspcc-dev/neo-go/blob/master/docs/vm.md)

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

# Contributing

Feel free to contribute to this project after reading the
[contributing guidelines](https://github.com/nspcc-dev/neo-go/blob/master/CONTRIBUTING.md).

Before starting to work on a certain topic, create an new issue first,
describing the feature/topic you are going to implement.

# Contact

- [@roman-khimov](https://github.com/roman-khimov) on GitHub
- [@volekerb](https://github.com/volekerb) on Github
- Reach out to us on the [NEO Discord](https://discordapp.com/invite/R8v48YA) channel

# License

- Open-source [MIT](https://github.com/nspcc-dev/neo-go/blob/master/LICENSE.md)
