# NEO-GO consensus node

Neo-go node can act as a consensus node.
It uses pure Go dBFT implementation from [nspcc-dev/dbft](https://github.com/nspcc-dev/dbft).

## How to start your own privnet with neo-go nodes
### Using existing Dockerfile
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
docker volume prune
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