# Package - Address Manager

This package can be used as a standalone to manage addresses on the NEO network. Although you can use it for other chains, the config parameters have been modified for the state of NEO as of now.

## Responsibility

To manage the data that the node knows about addresses;  data on good addresses, retry rates, failures, and lastSuccessful connection will be managed by the address manager. Also, If a service wants to fetch good address then it will be asked for from the address manager.


## Features

- On GetAddr it will give a list of good addresses to connect to

- On Addr it will receive addresses and remove any duplicates.

- General Management of Addresses

- Periodically saves the peers and metadata about peer into a .json file for retrieval (Not implemented yet)


## Note 

The Address manager will not deal with making connections to nodes. Please check the tests for the use cases for this package.
