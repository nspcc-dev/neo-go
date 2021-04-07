# NEO-GO consensus node

Neo-go node can act as a consensus node.
It uses pure Go dBFT implementation from [nspcc-dev/dbft](https://github.com/nspcc-dev/dbft).

## How to start your own privnet with neo-go nodes
### Using existing Dockerfile

neo-go comes with two preconfigured private network setups, the first one has
four consensus nodes and the second one uses single node. Nodes are packed
into Docker containers and four-node setup shares a volume for chain data.

Four-node setup uses ports 20333-20336 for P2P communication and ports
30333-30336 for RPC (Prometheus monitoring is also available at ports
20001-20004). Single-node is on ports 20333/30333/20001 for
P2P/RPC/Prometheus.

NeoGo default privnet configuration is made to work with four node consensus,
you have to modify it if you're to use single consensus node.

Node wallets are located in the `.docker/wallets` directory where
`wallet1_solo.json` is used for single-node setup and all the other ones for
four-node setup.

#### Prerequisites
- `docker`
- `docker-compose`
- `go` compiler

#### Instructions
You can use existing docker-compose file located in `.docker/docker-compose.yml`:
```bash
make env_image # build image
make env_up    # start containers, use "make env_single" for single CN
```
To monitor logs:
```bash
docker-compose -f .docker/docker-compose.yml logs -f
```

To stop:
```bash
make env_down
```

To remove old blockchain state:
```bash
make env_clean
``` 

### Start nodes manually
1. Create a separate config directory for every node and
place corresponding config named `protocol.privnet.yml` there.

2. Edit configuration file for every node.
Examples can be found at `config/protocol.privnet.docker.one.yml` (`two`, `three` etc.).
    1. Add `UnlockWallet` section with `Path` and `Password` strings for NEP-6
       wallet path and password for the account to be used for consensus node.
    2. Make sure that your `MinPeers` setting is equal to
       the number of nodes participating in consensus.
       This requirement is needed for nodes to correctly
       start and can be weakened in future.
    3. Set you `Address`, `Port` and `RPC.Port` to the appropriate values.
       They must differ between nodes.
    4. If you start binary from the same directory, you will probably want to change
       `DataDirectoryPath` from the `LevelDBOptions`. 

3. Start all nodes with `neo-go node --config-path <dir-from-step-2>`.
