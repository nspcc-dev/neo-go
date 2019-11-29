# NEO-GO consensus node

Neo-go node can act as a consensus node.
It uses pure Go dBFT implementation from [nspcc-dev/dbft](https://github.com/nspcc-dev/dbft).

## How to start your own privnet with neo-go nodes
### Using existing Dockerfile

neo-go comes with a preconfigured private network setup that consists of four
consensus nodes and 6000 blocks to make it more usable out of the box. Nodes
are packed into Docker containers with one shared volume for chain data (they
don't share actual DB, each node has its own DB in this volume). They use ports
20333-20336 for P2P communication and ports 30333-30336 for RPC (Prometheus
monitoring is also available at ports 20001-20004).

On the first container start they import 6K of blocks from a file, these
blocks contain several transactions that transfer all NEO into one address and
claim some GAS for it. NEO/GAS owner is:
 * address: AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y
 * wif: KxDgvEKzgSBPPfuVfw67oPQBSjidEiqTHURKSDL1R7yGaGYAeYnr

and you can use it to make some transactions of your own on this privnet.

Basically, this setup is closely resembling the one `neo-local` had for C# nodes
before the switch to single-node mode.

#### Prerequisites
- `docker`
- `docker-compose`
- `go` compiler
#### Instructions
You can use existing docker-compose file located in `.docker/docker-compose.yml`:
```bash
make env_image # build image
make env_up    # start containers
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
    1. Note that it differs a bit from C# NEO node json config: our `UnlockWallet` contains
       an encrypted WIF instead of the path to the wallet. 
    2. Make sure that your `MinPeers` setting is equal to
       the number of nodes participating in consensus.
       This requirement is needed for nodes to correctly
       start and can be weakened in future.
    3. Set you `Address`, `Port` and `RPC.Port` to the appropriate values.
       They must differ between nodes.
    4. If you start binary from the same directory, you will probably want to change
       `DataDirectoryPath` from the `LevelDBOptions`. 

3. Start all nodes with `neo-go node --config-path <dir-from-step-2>`.
