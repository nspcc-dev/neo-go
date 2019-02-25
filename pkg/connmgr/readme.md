# Package - Connection Manager

## Responsibility

- Manages the active, failed and pending connections for the node.

## Features

- Takes an address, dials it and packages it into a request to manage.

- Retry failed connections.

- Uses one function as a source for it's addresses. It does not manage addresses.


## Usage

The following methods are exposed from the Connection manager:

- NewRequest() : This will fetch a new address and connect to it.

- Connect(r *Request) : This takes a Request object and connects to it. It follow the same logic as NewRequest() however instead of getting the address from the datasource given upon initialisation, you directly feed the address you want to connect to.

- Disconnect(addrport string) : Given an address:port, this will disconnect it, close the connection and remove it from the connected and pending list, if it was there.

- Dial(addrport string) (net.Conn, error) : Given an address:port, this will connect to it and return a pointer to a connection plus a nil error if successful, or nil with an error.