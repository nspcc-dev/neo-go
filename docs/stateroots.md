# NeoGo state validation

NeoGo supports state validation using N3 stateroots and can also act as state
validator (run state validation service).

All NeoGo nodes always calculate MPT root hash for data stored by contracts.
Unlike in Neo Legacy, this behavior can't be turned off. They also process
stateroot messages broadcasted through the network and save validated
signatures from them if the state root hash specified there matches the one signed
by validators (or shouts loud in the log if it doesn't because it should be
the same).

## State validation service

The service is configured as `StateRoot` subsection of
`ApplicationConfiguration` section in your node config.

Parameters:
 * `Enabled`: boolean value, enables/disables the service, `true` for service
   to be enabled
 * `UnlockWallet`: service's wallet configuration:
     - `Path`: path to NEP-6 wallet.
     - `Password`: password for the account to be used by state validation
       node.

### Example

```
  StateRoot:
    Enabled: true
    UnlockWallet:
      Path: "/path/to/stateroot.wallet.json"
      Password: "knowyouare"
```

### Operation

To run state validation service on your network you need to:
 * set state validation node keys in `RoleManagement` contract
 * configure and run an appropriate number of state validation nodes with the keys
   specified in `RoleManagement` contract


## StateRootInHeader option

NeoGo also supports protocol extension to include state root hashes right into
header blocks. It's not compatible with regular Neo N3 state validation
service and it's not compatible with public Neo N3 networks, but you can use
it on private networks if needed.

The option is `StateRootInHeader` and it's specified in
`ProtocolConfiguration` section, set it to true and run your network with it
(whole network needs to be configured this way then).
