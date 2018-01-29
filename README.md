<p align="center">
  <img
  src="http://files.coinmarketcap.com.s3-website-us-east-1.amazonaws.com/static/img/coins/200x200/neo.png"
    width="125px;">
</p>

<h1 align="center">NEO-GO</h1>

<p align="center">
    Node and SDK for the <b>NEO</b> blockchain written in the <b>Go</b> language.
</p>

<p align="center">
  <a href="https://travis-ci.org/anthdm/neo-go">
    <img src="https://travis-ci.org/anthdm/neo-go.svg?branch=master">
  </a>
</p>

# Overview
### Long term project goals
Full port of the original C# [NEO project](https://github.com/neo-project). A complete toolkit for the NEO blockchain.

- Full server (consensus, RPC and bookkeeping) nodes.
- RPC client
- build, compile and deploy smart contracts with the Go vm

### Current state
This project is still under heavy development. Still working on internal API's and project layout. This should not take longer than 2 weeks. 

# Getting started 
If you can't wait to experiment with the current state of the project. clone the project, cd into it and run:

`go install ./cmd/neoserver`

Make sure you have a private net running. If you dont, take a look at [docker-privnet-with-gas](https://hub.docker.com/r/metachris/neo-privnet-with-gas/).

`neoserver -seed 127.0.0.1:20333`

The only thing the server currently will do is asking for peers and connect to the responded peers. All other messages are in development.

# Contributing
todo.

