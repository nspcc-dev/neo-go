# NeoGo Oracle service

NeoGo node can act as oracle service node for https and neofs protocols. It
has to have a wallet with key belonging to one of network's designated oracle
nodes (stored in `RoleManagement` native contract).

It needs [RPC service](rpc.md) to be enabled and configured properly because
RPC is used by oracle nodes to exchange signatures of the resulting
transaction.

## Configuration

To enable oracle service add `Oracle` subsection to `ApplicationConfiguration`
section of your node config.

Parameters:
 * `Enabled`: boolean value, enables/disables the service, `true` for service
   to be enabled
 * `AllowPrivateHost`: boolean value, enables/disables private IPs (like
   127.0.0.1 or 192.168.0.1) for https requests, it defaults to false and it's
   false on public networks, but you can enable it for private ones.
 * `Nodes`: list of oracle node RPC endpoints, it's used for oracle node
   communication. All oracle nodes should be specified there.
 * `NeoFS`: a subsection of its own for NeoFS configuration with two
   parameters:
     - `Timeout`: request timeout, like "5s"
     - `Nodes`: list of NeoFS nodes (their gRPC interfaces) to get data from,
       one node is enough to operate, but they're used in round-robin fashion,
       so you can spread the load by specifying multiple nodes
 * `MaxTaskTimeout`: maximum time a request can be active (retried to
   process), defaults to 1 hour if not specified.
 * `RefreshInterval`: retry period for requests that aren't yet processed,
   defaults to 3 minutes.
 * `MaxConcurrentRequests`: maximum number of requests processed in parallel,
   defaults to 10.
 * `RequestTimeout`: https request timeout, default is 5 seconds.
 * `ResponseTimeout`: RPC communication timeout for inter-oracle exchange,
   default is 4 seconds.
 * `UnlockWallet`: oracle wallet configuration:
     - `Path`: path to NEP-6 wallet.
     - `Password`: password for the account to be used by oracle node.

### Example

```
  Oracle:
    Enabled: true
    AllowPrivateHost: false
    MaxTaskTimeout: 432000000
    Nodes:
      - http://oracle1.example.com:20332
      - http://oracle2.example.com:20332
      - http://oracle3.example.com:20332
      - http://oracle4.example.com:20332
    NeoFS:
      Nodes:
        - st1.storage.fs.neo.org:8080
        - st2.storage.fs.neo.org:8080
        - st3.storage.fs.neo.org:8080
        - st4.storage.fs.neo.org:8080
    UnlockWallet:
      Path: "/path/to/oracle-wallet.json"
      Password: "dontworryaboutthevase"
```

## Operation

To run oracle service on your network you need to:
 * set oracle node keys in `RoleManagement` contract
 * configure and run appropriate number of oracle nodes with keys specified in
   `RoleManagement` contract
