# NeoGo smart contract compiler

The neo-go compiler compiles Go programs to a bytecode that the Neo virtual machine can understand.

## Language compatibility

The compiler is mostly compatible with regular Go language specification. However,
there are some important deviations that you need to be aware of that make it
a dialect of Go rather than a complete port of the language:
 * `new()` is not supported, most of the time you can substitute structs with composite literals
 * `make()` is supported for maps and slices with elements of basic types
 * `copy()` is supported only for byte slices because of the underlying `MEMCPY` opcode
 * pointers are supported only for struct literals, one can't take an address
   of an arbitrary variable
 * there is no real distinction between different integer types, all of them
   work as big.Int in Go with a limit of 256 bit in width; so you can use
   `int` for just about anything. This is the way integers work in Neo VM and
   adding proper Go types emulation is considered to be too costly.
 * goroutines, channels and garbage collection are not supported and will
   never be because emulating that aspects of Go runtime on top of Neo VM is
   close to impossible
 * `defer` and `recover` are supported except for the cases where panic occurs in
   `return` statement because this complicates implementation and imposes runtime
    overhead for all contracts. This can easily be mitigated by first storing values
    in variables and returning the result.
 * lambdas are supported, but closures are not.
 * maps are supported, but valid map keys are booleans, integers and strings with length <= 64
 * converting value to interface type doesn't change the underlying type,
   original value will always be used, therefore it never panics and always "succeeds";
   it's up to the programmer whether it's a correct use of a value
 * type assertion with two return values is not supported; single return value (of the desired type)
   is supported; type assertion panics if value can't be asserted to the desired type, therefore
   it's up to the programmer whether assert can be performed successfully.
 * type aliases including the built-in `any` alias are supported.
 * generics are not supported, but eventually will be (at least, partially), ref. https://github.com/nspcc-dev/neo-go/issues/2376.
 * `~` token is not supported
 * `comparable` is not supported
 * arrays (`[4]byte`) are not supported (https://github.com/nspcc-dev/neo-go/issues/3524)
 * `min()` and `max()` are supported for integer types only.
 * ranging over integers in `for` is not supported (https://github.com/nspcc-dev/neo-go/issues/3525)
 * `for` loop variables are treated in pre-Go 1.22 way: a single instance is created for the whole loop

## VM API (interop layer)
Compiler translates interop function calls into Neo VM syscalls or (for custom
functions) into Neo VM instructions. [Refer to
pkg.go.dev](https://pkg.go.dev/github.com/nspcc-dev/neo-go/pkg/interop)
for full API documentation. In general it provides the same level of
functionality as Neo .net Framework library.

Compiler provides some helpful builtins in `util`, `convert` and `math` packages.
Refer to them for detailed documentation. 

`_deploy()` function has a special meaning and is executed when contract is deployed.
It should return no value and accept two arguments: the first one is `data` containing
all values `deploy` is aware of and able to make use of; the second one is a bool
argument which will be true on contract update.
`_deploy()` functions are called for every imported package in the same order as `init()`. 

## Quick start

### Go setup

The compiler uses Go parser internally and depends on regular Go compiler
presence, so make sure you have it installed and set up. On some distributions
this requires you to set proper `GOROOT` environment variable, like
```
export GOROOT=/usr/lib64/go/1.15
```

The best way to create a new contract is to use `contract init` command. This will
create an example source file, a config file and `go.mod` with `github.com/nspcc-dev/neo-go/pkg/interop` dependency.
```
$ ./bin/neo-go contract init --name MyAwesomeContract
$ cd MyAwesomeContract
```

You'll also need to download dependency modules for your contract like this (in the
directory containing contract package):
```
$ go mod tidy
```

### Compiling

```
./bin/neo-go contract compile -i contract.go
```

By default, the filename will be the name of your .go file with the .nef
extension, the file will be located in the same directory with your Go contract.
Along with the compiled contract and if the contract configuration file
`contract.yml` exist, the following files will be generated:
* smart-contract manifest file (`contract.manifest.json`) that is needed to deploy
  the contract to the network
* bindings configuration file (`contract.bindings.yml`) that is needed to generate
  code-based or RPC contract bindings
All of them will be located in the same directory with your Go contract.

If you want another location for your compiled contract:

```
./bin/neo-go contract compile -i contract.go --out /Users/foo/bar/contract.nef
```

If your contract is split across multiple files, you must provide a path
to the directory where package files are contained instead of a single Go file
(`out.nef` will be used as the default output file in this case):
```
./bin/neo-go contract compile -i ./path/to/contract
```

### Debugging
You can dump the opcodes generated by the compiler with the following command:

```
./bin/neo-go contract inspect -i contract.go -c
```

This will result in something like this:

```
INDEX    OPCODE        PARAMETER                                                                                                                              
0        INITSLOT      4 local, 2 arg                                                                                                                         <<
3        LDARG1                                                                                                                                               
4        NOT                                                                                                                                                  
5        JMPIFNOT_L    151 (146/92000000)                                                                                                                     
10       SYSCALL       System.Storage.GetContext (9bf667ce)                                                                                                   
15       NOP                                                                                                                                                  
16       STLOC0                                                                                                                                               
17       PUSHDATA1     53746f72616765206b6579206e6f7420796574207365742e2053657474696e6720746f2030 ("Storage key not yet set. Setting to 0")                   
56       CONVERT       Buffer (30)                                                                                                                            
58       PUSH1                                                                                                                                                
59       PACK                                                                                                                                                 
60       STLOC1                                                                                                                                               
61       PUSHDATA1     696e666f ("info")                                                                                                                      
67       LDLOC1                                                                                                                                               
68       SWAP                                                                                                                                                 
69       SYSCALL       System.Runtime.Notify (95016f61)                                                                                                       
74       NOP                                                                                                                                                  
75       PUSH0                                                                                                                                                
76       STLOC2                                                                                                                                               
77       LDLOC0                                                                                                                                               
78       PUSHDATA1     746573742d73746f726167652d6b6579 ("test-storage-key")                                                                                  
96       LDLOC2                                                                                                                                               
97       REVERSE3                                                                                                                                             
98       SYSCALL       System.Storage.Put (e63f1884)                                                                                                          
103      NOP                                                                                                                                                  
104      PUSHDATA1     53746f72616765206b657920697320696e697469616c69736564 ("Storage key is initialised")                                                    
132      CONVERT       Buffer (30)                                                                                                                            
134      PUSH1                                                                                                                                                
135      PACK                                                                                                                                                 
136      STLOC3                                                                                                                                               
137      PUSHDATA1     696e666f ("info")                                                                                                                      
143      LDLOC3                                                                                                                                               
144      SWAP                                                                                                                                                 
145      SYSCALL       System.Runtime.Notify (95016f61)                                                                                                       
150      NOP                                                                                                                                                  
151      RET                                                                                                                                                  
152      INITSLOT      5 local, 0 arg                                                                                                                         
155      SYSCALL       System.Storage.GetContext (9bf667ce)                                                                                                   
160      NOP                                                                                                                                                  
161      STLOC0                                                                                                                                               
162      LDLOC0                                                                                                                                               
163      PUSHDATA1     746573742d73746f726167652d6b6579 ("test-storage-key")                                                                                  
181      SWAP                                                                                                                                                 
182      SYSCALL       System.Storage.Get (925de831)                                                                                                          
187      NOP                                                                                                                                                  
188      STLOC1                                                                                                                                               
189      PUSHDATA1     56616c756520726561642066726f6d2073746f72616765 ("Value read from storage")                                                             
214      CONVERT       Buffer (30)                                                                                                                            
216      PUSH1                                                                                                                                                
217      PACK                                                                                                                                                 
218      STLOC2                                                                                                                                               
219      PUSHDATA1     696e666f ("info")                                                                                                                      
225      LDLOC2                                                                                                                                               
226      SWAP                                                                                                                                                 
227      SYSCALL       System.Runtime.Notify (95016f61)                                                                                                       
232      NOP                                                                                                                                                  
233      PUSHDATA1     53746f72616765206b657920616c7265616479207365742e20496e6372656d656e74696e672062792031 ("Storage key already set. Incrementing by 1")    
277      CONVERT       Buffer (30)                                                                                                                            
279      PUSH1                                                                                                                                                
280      PACK                                                                                                                                                 
281      STLOC3                                                                                                                                               
282      PUSHDATA1     696e666f ("info")                                                                                                                      
288      LDLOC3                                                                                                                                               
289      SWAP                                                                                                                                                 
290      SYSCALL       System.Runtime.Notify (95016f61)                                                                                                       
295      NOP                                                                                                                                                  
296      LDLOC1                                                                                                                                               
297      CONVERT       Integer (21)                                                                                                                           
299      PUSH1                                                                                                                                                
300      ADD                                                                                                                                                  
301      STLOC1                                                                                                                                               
302      LDLOC0                                                                                                                                               
303      PUSHDATA1     746573742d73746f726167652d6b6579 ("test-storage-key")                                                                                  
321      LDLOC1                                                                                                                                               
322      REVERSE3                                                                                                                                             
323      SYSCALL       System.Storage.Put (e63f1884)                                                                                                          
328      NOP                                                                                                                                                  
329      PUSHDATA1     4e65772076616c7565207772697474656e20696e746f2073746f72616765 ("New value written into storage")                                        
361      CONVERT       Buffer (30)                                                                                                                            
363      PUSH1                                                                                                                                                
364      PACK                                                                                                                                                 
365      STLOC4                                                                                                                                               
366      PUSHDATA1     696e666f ("info")                                                                                                                      
372      LDLOC4                                                                                                                                               
373      SWAP                                                                                                                                                 
374      SYSCALL       System.Runtime.Notify (95016f61)                                                                                                       
379      NOP                                                                                                                                                  
380      LDLOC1                                                                                                                                               
381      RET                         
```

#### Neo Smart Contract Debugger support

It's possible to debug contracts written in Go using standard [Neo Smart
Contract Debugger](https://github.com/neo-project/neo-debugger/) which is a
part of [Neo Blockchain
Toolkit](https://github.com/neo-project/neo-blockchain-toolkit/). To do that
you need to generate debug information using `--debug` option, like this:

```
$ ./bin/neo-go contract compile -i contract.go -c contract.yml -m contract.manifest.json -o contract.nef --debug contract.debug.json
```

This file can then be used by debugger and set up to work just like for any
other supported language.

### Deploying

Deploying a contract to blockchain with neo-go requires both NEF and JSON
manifest generated by the compiler from a configuration file provided in YAML
format. To create contract manifest, pass a YAML file with `-c` parameter and
specify the manifest output file with `-m`:
```
./bin/neo-go contract compile -i contract.go -c config.yml -m contract.manifest.json
```

Example of such YAML file contents:
```
name: Contract
safemethods: []
supportedstandards: []
events:
  - name: info
    parameters:
      - name: message
        type: String
```

Then, the manifest can be passed to the `deploy` command via `-m` option:

```
$ ./bin/neo-go contract deploy -i contract.nef -m contract.manifest.json -r http://localhost:20331 -w wallet.json
```

Deployment works via an RPC server, an address of which is passed via `-r`
option, and should be signed using a wallet from `-w` option. More details can
be found in `deploy` command help.

### Updating

Updating a deployed smart contract on the blockchain requires the updated NEF file,
the manifest, and the target contract hash. Either NEF file or the manifest
may be omitted, but not both. Additional data parameter may be specified.

```
$ ./bin/neo-go contract update -i contract.nef -m contract.manifest.json -r http://localhost:20331 -w wallet.json 0x4bfd65abeac8f85931f2d85cfdbc61de3dd03b94
```

Updating works via an RPC server, an address of which is passed via `-r`
option, and should be signed using a wallet from `-w` option. More details can
be found in `update` command help.

### Destroying

Destroying a deployed smart contract. Requires the contract hash.

```bash
$ ./bin/neo-go contract destroy -r http://localhost:20331 -w wallet.json 0x4bfd65abeac8f85931f2d85cfdbc61de3dd03b94
```

Destroying works via an RPC server, an address of which is passed via `-r`
option, and should be signed using a wallet from `-w` option. More details can
be found in `destroy` command help.

#### Config file
Configuration file contains following options:

| Parameter | Description | Example |
| --- | --- | --- |
| `name` | Contract name in the manifest. | `"My awesome contract"`
| `safemethods` | List of methods which don't change contract state, don't emit notifications and are available for anyone to call. | `["balanceOf", "decimals"]`
| `supportedstandards` | List of standards this contract implements. For example, `NEP-11` or `NEP-17` token standard. This will enable additional checks in compiler. The check can be disabled with `--no-standards` flag. | `["NEP-17"]`
| `events` | Notifications emitted by this contract. | See [Events](#Events). |
| `permissions` | Foreign calls allowed for this contract. | See [Permissions](#Permissions). |
| `overloads` | Custom method names for this contract. | See [Overloads](#Overloads). |

##### Events
Each event must have a name and 0 or more parameters. Parameters are specified using their name and type.
Both event and parameter names must be strings.
Parameter type can be one of the following:

Type in code | Type in config file
--- | ---
`bool` | `Boolean` 
`int`, `int64` etc.| `Integer`
`[]byte` | `ByteArray` 
`string` | `String` 
Any non-byte slice `[]T`| `Array` 
`map[K]V` | `Map` 
`interop.Hash160` | `Hash160`
`interop.Hash256` | `Hash256`
`interop.Interface` | `InteropInterface`
`interop.PublicKey` | `PublicKey`
`interop.Signature` | `Signature`
anything else | `Any` 

`interop.*` types are defined as aliases in `github.com/nspcc-dev/neo-go/pkg/interop` module
with the sole purpose of correct manifest generation.

As an example, consider `Transfer` event from `NEP-17` standard:
```
- name: Transfer
  parameters:
    - name: from
      type: Hash160
    - name: to
      type: Hash160
    - name: amount
      type: Integer
```

By default, compiler performs some sanity checks. Most of the time
it will report missing events and/or parameter type mismatch.
It isn't prohibited to use a variable as an event name in code, but it will prevent
the compiler from analyzing the event. It is better to use either constant or string literal.
It isn't prohibited to use ellipsis expression as an event arguments, but it will also
prevent the compiler from analyzing the event. It is better to provide arguments directly
without `...`. The type conversion code will be emitted for checked events, it will cast
argument types to ones specified in the contract manifest. These checks and conversion can
be disabled with `--no-events` flag.

##### Permissions
Each permission specifies contracts and methods allowed for this permission.
If a contract is not specified in a rule, specified set of methods can be called on any contract.
By default, no calls are allowed. The simplest permission is to allow everything:
```
- methods: '*'
```

Another common case is to allow calling `onNEP17Payment`, which is necessary
for most of the NEP-17 token implementations:
```
- methods: ["onNEP17Payment"]
```

In addition to `methods`, permission can have one of these fields:
1. `hash` contains hash and restricts a set of contracts to a single contract.
2. `group` contains public key and restricts a set of contracts to those that
have the corresponding group in their manifest.

Consider an example:
```
- methods: ["onNEP17Payment"]
- hash: fffdc93764dbaddd97c48f252a53ea4643faa3fd
  methods: ["start", "stop"]
- group: 03184b018d6b2bc093e535519732b3fd3f7551c8cffaf4621dd5a0b89482ca66c9
  methods: ["update"]
```

This set of permissions allows calling:
- `onNEP17Payment` method of any contract
- `start` and `stop` methods of contract with hash `fffdc93764dbaddd97c48f252a53ea4643faa3fd`
- `update` method of contract in group with public key `03184b018d6b2bc093e535519732b3fd3f7551c8cffaf4621dd5a0b89482ca66c9`

Also note that a native contract must be included here too. For example, if your contract
transfers NEO/GAS or gets some info from the `Ledger` contract, all of these
calls must be allowed in permissions.

The compiler does its best to ensure that correct permissions are specified in the config.
Incorrect permissions will result in runtime invocation failures.
Using either constant or literal for contract hash and method will allow the compiler
to perform more extensive analysis.
This check can be disabled with `--no-permissions` flag.

##### Overloads
NeoVM allows a contract to have multiple methods with the same name
but different parameters number. Go lacks this feature, but this can be circumvented
with `overloads` section. Essentially, it is a mapping from default contract method names
to the new ones.
```
- overloads:
    oldName1: newName
    oldName2: newName
```
Since the use-case for this is to provide multiple implementations with the same ABI name,
`newName` is required to be already present in the compiled contract.

As an example, consider [`NEP-11` standard](https://github.com/neo-project/proposals/blob/master/nep-11.mediawiki#transfer).
It requires a divisible NFT contract to have 2 `transfer` methods. To achieve this, we might implement
`Transfer` and `TransferDivisible` and specify the emitted name in the config:
```
- overloads:
    transferDivisible:transfer
```


#### Manifest file
Any contract can be included in a group identified by a public key which is used in [permissions](#Permissions).
This is achieved with `manifest add-group` command.
```
./bin/neo-go contract manifest add-group -n contract.nef -m contract.manifest.json --sender <sender> --wallet /path/to/wallet.json --account <account>
```
It accepts contract `.nef` and manifest files emitted by `compile` command as well as
sender and signer accounts. `--sender` is the account that will send deploy transaction later (not necessarily in wallet).
`--account` is the wallet account which signs contract hash using group private key.

#### Neo Express support

It's possible to deploy contracts written in Go using [Neo
Express](https://github.com/neo-project/neo-express), which is a part of [Neo
Blockchain
Toolkit](https://github.com/neo-project/neo-blockchain-toolkit/). To do that,
you need to generate a different metadata file using YAML written for
deployment with neo-go. It's done in the same step with compilation via
`--config` input parameter and `--abi` output parameter, combined with debug
support the command line will look like this:

```
$ ./bin/neo-go contract compile -i contract.go --config contract.yml -o contract.nef --debug contract.debug.json --abi contract.abi.json 
```

This file can then be used by toolkit to deploy contract the same way
contracts in other languages are deployed.


### Invoking
You can import your contract into a standalone VM and run it there (see [VM
documentation](vm.md) for more info), but that only works for simple contracts
that don't use blockchain a lot. For more real contracts you need to deploy
them first and then do test invocations and regular invocations with `contract
testinvokefunction` and `contract invokefunction` commands (or their variants,
see `contract` command help for more details. They all work via RPC, so it's a
mandatory parameter.

Example call (contract `f84d6a337fbc3d3a201d41da99e86b479e7a2554` with method
`balanceOf` and method's parameter `NVTiAjNgagDkTr5HTzDmQP9kPwPHN5BgVq` using
given RPC server and wallet and paying 0.00001 extra GAS for this transaction):

```
$ ./bin/neo-go contract invokefunction -r http://localhost:20331 -w my_wallet.json -g 0.00001 f84d6a337fbc3d3a201d41da99e86b479e7a2554 balanceOf NVTiAjNgagDkTr5HTzDmQP9kPwPHN5BgVq
```

### Generating contract bindings
To be able to use deployed contract from another contract one needs to have
its interface definition (exported methods and hash). While it is possible to
use generic contract.Call interop interface, it's not very convenient and
efficient. NeoGo can autogenerate contract bindings in Go language for any
deployed contract based on its manifest, it creates a Go source file with all
of the contract's methods that then can be imported and used as a regular Go
package.

```
$ ./bin/neo-go contract generate-wrapper --manifest manifest.json --out wrapper.go --hash 0x1b4357bff5a01bdf2a6581247cf9ed1e24629176
```

Notice that some structured types can be omitted this way (when a function
returns some structure it's just an "Array" type in the manifest with no
internal details), but if the contract you're using is written in Go
originally you can create a specific configuration file during compilation
that will add this data for wrapper generator to use:

```
$ ./bin/neo-go contract compile -i contract.go --config contract.yml -o contract.nef --manifest manifest.json --bindings contract.bindings.yml
$ ./bin/neo-go contract generate-wrapper --manifest manifest.json --config contract.bindings.yml --out wrapper.go --hash 0x1b4357bff5a01bdf2a6581247cf9ed1e24629176
```

### Generating RPC contract bindings
To simplify interacting with the contract via RPC you can generate
contract-specific RPC bindings with the "generate-rpcwrapper" command. It
generates ContractReader structure for safe methods that accept appropriate
data for input and return things returned by the contract. State-changing
methods are contained in Contract structure with each contract method
represented by three wrapper methods that create/send transaction with a
script performing appropriate action. This script invokes contract method and
does not do anything else unless the method's returned value is of a boolean
type, in this case an ASSERT is added to script making it fail when the method
returns false.

```
$ ./bin/neo-go contract generate-rpcwrapper --manifest manifest.json --out rpcwrapper.go --hash 0x1b4357bff5a01bdf2a6581247cf9ed1e24629176
```

If your contract is NEP-11 or NEP-17 that's autodetected and an appropriate
package is included as well. Notice that the type data available in the
manifest is limited, so in some cases the interface generated may use generic
stackitem types. Any InteropInterface returned from a method is treated as
iterator and an appropriate unwrapper is used with UUID and iterator structure
result. This pair can then be used in Invoker `TraverseIterator` method to
retrieve actual resulting items.

Go contracts can also make use of additional type data from bindings
configuration file generated during compilation. This can cover arrays, maps
and structures. Notice that structured types returned by methods can't be Null
at the moment (see #2795).

```
$ ./bin/neo-go contract compile -i contract.go --config contract.yml -o contract.nef --manifest manifest.json --bindings contract.bindings.yml --guess-eventtypes
$ ./bin/neo-go contract generate-rpcwrapper --manifest manifest.json --config contract.bindings.yml --out rpcwrapper.go --hash 0x1b4357bff5a01bdf2a6581247cf9ed1e24629176
```

Contract-specific RPC-bindings generated by "generate-rpcwrapper" command include
structure wrappers for each event declared in the contract manifest as far as the
set of helpers that allow to retrieve emitted event from the application log or
from stackitem. By default, event wrappers builder use event structure that was
described in the manifest. Since the type data available in the manifest is
limited, in some cases the resulting generated event structure may use generic
go types. Go contracts can make use of additional type data from bindings
configuration file generated during compilation. Like for any other contract
types, this can cover arrays, maps and structures. To reach the maximum
resemblance between the emitted events and the generated event wrappers, we
recommend either to fill in the extended events type information in the contract
configuration file before the compilation or to use `--guess-eventtypes`
compilation option.

If using `--guess-eventtypes` compilation option, event parameter types will be
guessed from the arguments of `runtime.Notify` calls for each emitted event. If
multiple calls of `runtime.Notify` are found, then argument types will be checked
for matching (guessed types must be the same across the particular event usages).
After that, the extended types binding configuration will be generated according
to the emitted events parameter types. `--guess-eventtypes` compilation option
is able to recognize those events that has a constant name known at a compilation
time and do not include variadic arguments usage. Thus, use this option if your
contract suites these requirements. Otherwise, we recommend to manually specify
extended event parameter types information in the contract configuration file.

Extended event parameter type information can be provided manually via contract
configuration file under the `events` section. Each event parameter specified in
this section may be supplied with additional parameter type information specified
under `extendedtype` subsection. The extended type information (`ExtendedType`)
has the following structure:

| Field       | Type                                                                                                                                  | Required                                                                             | Meaning                                                                                                                                          |
|-------------|---------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------|
| `base`      | Any valid [NEP-14 parameter type](https://github.com/neo-project/proposals/blob/master/nep-14.mediawiki#parametertype) except `Void`. | Always required.                                                                              | The base type of a parameter, e.g. `Array` for go structures and any nested arrays, `Map` for nested maps, `Hash160` for 160-bits integers, etc. |
| `name`      | `string`                                                                                                                              | Required for structures, omitted for arrays, interfaces and maps.                    | Name of a structure that will be used in the resulting RPC binding.                                                                              |
| `interface` | `string`                                                                                                                              | Required for `InteropInterface`-based types, currently `iterator` only is supported. | Underlying value of the `InteropInterface`.                                                                                                      |
| `key`       | Any simple [NEP-14 parameter type](https://github.com/neo-project/proposals/blob/master/nep-14.mediawiki#parametertype).              | Required for `Map`-based types.                                                      | Key type for maps.                                                                                                                               |
| `value`     | `ExtendedType`.                                                        | Required for iterators, arrays and maps.                                             | Value type of iterators, arrays and maps.                                                                                                        |
| `fields`    | Array of `FieldExtendedType`.                                                                                                         | Required for structures.                                                             | Ordered type data for structure fields.                                                                                                          |

The structure's field extended information (`FieldExtendedType`) has the following structure:

| Field                  | Type           | Required         | Meaning                                                                     |
|------------------------|----------------|------------------|-----------------------------------------------------------------------------|
| `field`                | `string`       | Always required. | Name of the structure field that will be used in the resulting RPC binding. |
| Inlined `ExtendedType` | `ExtendedType` | Always required. | The extended type information about structure field.                        |


Any named structures used in the `ExtendedType` description must be manually
specified in the contract configuration file under top-level `namedtypes` section
in the form of `map[string]ExtendedType`, where the map key is a name of the
described named structure that matches the one provided in the `name` field of
the event parameter's extended type.

Here's the example of manually-created contract configuration file that uses
extended types for event parameters description:

```
name: "HelloWorld contract"
supportedstandards: []
events:
  - name: Some simple notification
    parameters:
      - name: intP
        type: Integer
      - name: boolP
        type: Boolean
      - name: stringP
        type: String
  - name: Structure notification
    parameters:
      - name: structure parameter
        type: Array
        extendedtype:
          base: Array
          name: transferData
  - name: Map of structures notification
    parameters:
      - name: map parameter
        type: Map
        extendedtype:
          base: Map
          key: Integer
          value:
            base: Array
            name: transferData
  - name: Iterator notification
    parameters:
      - name: data
        type: InteropInterface
        extendedtype:
          base: InteropInterface
          interface: iterator
namedtypes:
  transferData:
    base: Array
    fields:
      - field: IntField
        base: Integer
      - field: BoolField
        base: Boolean
```

## Smart contract examples

Some examples are provided in the [examples directory](../examples). For more
sophisticated real-world contracts written in Go check out [NeoFS
contracts](https://github.com/nspcc-dev/neofs-contract/).

## How to report compiler bugs 
1. Make a proper testcase (example testcases can be found in the tests folder)
2. Create an issue on Github 
3. Make a PR with a reference to the created issue, containing the testcase that proves the bug
4. Either you fix the bug yourself or wait for patch that solves the problem
