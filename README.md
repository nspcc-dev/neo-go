<p align="center">
<img 
    src="http://res.cloudinary.com/vidsy/image/upload/v1503160820/CoZ_Icon_DARKBLUE_200x178px_oq0gxm.png" 
    width="125px"
  >
</p>

<h1 align="center">neo-go</h1>

<p align="center">
  <b>Go</b> Node and SDK for the <a href="https://neo.org">NEO</a> blockchain.
</p>

<p align="center">
  <a href="https://travis-ci.org/anthdm/neo-go">
    <img src="https://travis-ci.org/anthdm/neo-go.svg?branch=master">
  </a>
</p>

# Overview

> This project is currently in **alpha** and under active development.

## Project Goals

Full port of the original C# [NEO project](https://github.com/neo-project). 
A complete toolkit for the NEO blockchain, including:

- Full consensus node
- Full RPC node
- RPC client
- CLI tool
- Smart contract compiler

## Current State

This project is still under heavy development. Still working on internal API's and project layout. T
his should not take longer than 2 weeks. 

The project will exist out of the following packages:

| Package       | State   | Developer                            |
|---------------|---------|--------------------------------------|
| api           | started | [@anthdm](https://github.com/anthdm) |
| core          | started | [@anthdm](https://github.com/anthdm) |
| network       | started | [@anthdm](https://github.com/anthdm) |
| smartcontract | started | [@revett](https://github.com/revett) |
| vm            | started | [@revett](https://github.com/revett) |

# Getting Started 

## Server

Install dependencies, this requires [Glide](https://github.com/Masterminds/glide#install):

```
make deps
```

Build the **neo-go** CLI:

```
make build
```

Currently, there is a minimal subset of the NEO protocol implemented. 
To start experimenting make sure you a have a private net running on your machine. 
If you dont, take a look at [docker-privnet-with-gas](https://hub.docker.com/r/metachris/neo-privnet-with-gas/). 

Start the server:

```
./bin/neo-go -seed 127.0.0.1:20333
```

You can add multiple seeds if you want:

```
./bin/neo-go -seed 127.0.0.1:20333,127.0.01:20334
```

By default the server will currently run on port 3000, for testing purposes. 
You can change that by setting the tcp flag:

```
./bin/neo-go -seed 127.0.0.1:20333 -tcp 1337
```

## RPC

If you want your node to also serve JSON-RPC, you can do that by setting the following flag:

```
./bin/neo-go -rpc 4000
```

In this case server will accept and respond JSON-RPC on port 4000. 
Keep in mind that currently there is only a small subset of the JSON-RPC implemented. 
Feel free to make a PR with more functionality.

## VM

```
TODO
```

## Smart Contracts

```
TODO
```

# Contributing

Feel free to contribute to this project after reading the 
[contributing guidelines](https://github.com/anthdm/neo-go/blob/master/CONTRIBUTING.md).

Before starting to work on a certain topic, create an new issue first, 
describing the feauture/topic you are going to implement.

# Contact

- [@anthdm](https://github.com/anthdm) on Github
- [@anthdm](https://twitter.com/anthdm) on Twitter
- Send me an email anthony@cityofzion.io

# License

- Open-source [MIT](https://github.com/anthdm/neo-go/blob/master/LICENCE.md)
