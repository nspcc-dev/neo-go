# Package - Connection Manager

## Responsibility

- Manages the active, failed and pending connections for the node.

## Features

- Takes an Request, dials it and logs information based on the connectivity.

- Retry failed connections.

- Removable address source. The connection manager does not manage addresses, only connections.


## Usage

The following methods are exposed from the Connection manager:

- Connect(r *Request) : This takes a Request object and connects to it. It follow the same logic as NewRequest() however instead of getting the address from the datasource given upon initialisation, you directly feed the address you want to connect to.

- Disconnect(addrport string) : Given an address:port, this will disconnect it, close the connection and remove it from the connected and pending list, if it was there.
