<p align="center">
<img 
    src="http://res.cloudinary.com/vidsy/image/upload/v1503160820/CoZ_Icon_DARKBLUE_200x178px_oq0gxm.png" 
    width="125px"
  >
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
> This project is currently in alpha and under active development.

### Long term project goals
Full port of the original C# [NEO project](https://github.com/neo-project). A complete toolkit for the NEO blockchain.

- Full server (consensus and RPC) nodes.
- RPC client
- build, compile and deploy smart contracts with the Go vm

### Current state
This project is still under heavy development. Still working on internal API's and project layout. This should not take longer than 2 weeks. 

The project will exist out of the following topics/packages:

1. network (started) 
2. core (started)
3. vm (open)
4. smartcontract (open)
5. api (RPC server) (open)

# Getting started 
### Server

Install the neoserver cli `go install ./cmd/neoserver`

Currently, there is a minimal subset of the NEO protocol implemented. To start experimenting make sure you a have a private net running on your machine. If you dont, take a look at [docker-privnet-with-gas](https://hub.docker.com/r/metachris/neo-privnet-with-gas/). 

Start the server:

`neoserver -seed 127.0.0.1:20333`

You can add multiple seeds if you want:

`neoserver -seed 127.0.0.1:20333,127.0.01:20334`

### RPC
To be implemented..

### vm
To be implemented..

### smart contracts
To be implemented..

# Contributing
Feel free to contribute to this project after reading the [contributing guidelines](https://github.com/anthdm/neo-go/blob/master/CONTRIBUTING.md).

Before starting to work on a certain topic, create an new issue first, describing the feauture/topic you are going to implement.

# Contact
- [@anthdm](https://github.com/anthdm) on Github
- [@anthdm](https://twitter.com/anthdm) on Twitter
- Send me an email anthony@cityofzion.io

# License
- Open-source [MIT](https://github.com/anthdm/neo-go/blob/master/LICENCE.md)
