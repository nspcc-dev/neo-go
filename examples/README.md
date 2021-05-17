# NEO-GO smart contract examples

`examples` directory contains smart contract examples written in Go. 
These examples are aimed to demonstrate the basic usage of Go programming 
language to write NEO smart contracts as far as to provide a brief introduction
to the NEO-specific interop package application.

## Examples structure

Each example represents a separate folder with the smart contract content inside.
The content is presented by the smart contract code written in Go and the smart
contract configuration file with `.yml` extension.

Some contracts have a contract owner, which is needed for witness checking and 
represented by the `owner` string constant inside the smart contract code. The 
owner account address is taken from the [my_wallet.json](my_wallet.json). The 
wallet is located under the `examples` directory, has a single account inside 
with the `my_account` label which can be decrypted with the `qwerty` password. 
You can use `my_wallet.json` to deploy example contracts.

See the table below for the detailed examples description.

| Example | Description |
| --- | --- |
| [engine](engine) | This contract demonstrates how to use `runtime` interop package which implements an API for `System.Runtime.*` NEO system calls. Please, refer to the `runtime` [package documentation](../pkg/interop/doc.go) for details. |
| [events](events) | The contract shows how execution notifications with the different arguments types can be sent with the help of `runtime.Notify` function of the `runtime` interop package. Please, refer to the `runtime.Notify` [function documentation](../pkg/interop/runtime/runtime.go) for details. |
| [iterator](iterator) | This example describes a way to work with NEO iterators. Please, refer to the `iterator` [package documentation](../pkg/interop/iterator/iterator.go) for details. |
| [nft-nd](nft-nd) | NEP-11 non-divisible NFT. See NEP-11 token standard [specification](https://github.com/neo-project/proposals/pull/130) for details. |
| [nft-nd-nns](nft-nd-nns) | Neo Name Service contract which is NEP-11 non-divisible NFT. The contract implements methods for Neo domain name system managing such as domains registration/transferring, records addition and names resolving. |
| [oracle](oracle) | Oracle demo contract exposing two methods that you can use to process URLs. It uses oracle native contract, see [interop package documentation](../pkg/interop/native/oracle/oracle.go) also. |
| [runtime](runtime) | This contract demonstrates how to use special `_initialize` and `_deploy` methods. See the [compiler documentation](../docs/compiler.md#vm-api-interop-layer ) for methods details. It also shows the pattern for checking owner witness inside the contract with the help of `runtime.CheckWitness` interop [function](../pkg/interop/runtime/runtime.go). |
| [storage](storage) | The contract implements API for basic operations with a contract storage. It shows hos to use `storage` interop package. See the `storage` [package documentation](../pkg/interop/storage/storage.go). |
| [timer](timer) | The idea of the contract is to count `tick` method invocations and destroy itself after the third invocation. It shows how to use `contract.Call` interop function to call, update (migrate) and destroy the contract. Please, refer to the `contract.Call` [function documentation](../pkg/interop/contract/contract.go) |
| [token](token) | This contract implements NEP17 token standard (like NEO and GAS tokens) with all required methods and operations. See the NEP17 token standard [specification](https://github.com/neo-project/proposals/pull/126) for details. |
| [token-sale](token-sale) | The contract represents a token with `allowance`. It means that the token owner should approve token withdrawing before the transfer. The contract demonstrates how interop packages can be combined to work together. |

## Compile

Please, refer to the neo-go
[smart contract compiler documentation](../docs/compiler.md) to compile example
smart contracts.

## Deploy

You can set up neo-go private network to deploy the example contracts. Please, 
refer to the [consensus documentation](../docs/consensus.md) to start your own 
privnet with neo-go nodes.

To deploy smart contracts, refer to the 
[Deploying section](../docs/compiler.md#deploying) of the compiler documentation.

## Where to start

Feel free to explore neo-go smart contract development 
[workshop](https://github.com/nspcc-dev/neo-go-sc-wrkshp) to get the basic
concepts of how to develop, compile, debug and deploy NEO smart contracts written
in go.



