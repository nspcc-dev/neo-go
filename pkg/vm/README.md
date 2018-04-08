# NEO-GO-VM

A cross platform virtual machine implementation for `avm` compatible programs. 

# Installation

## With neo-go
Install dependencies.

`neo-go` uses [dep](https://github.com/golang/dep) as its dependency manager. After installing `deps` you can run:

```
make deps
```

Build the `neo-go` cli:

```
make build
```

Start the virtual machine:

```
./bin/neo-go vm
```

```
    _   ____________        __________      _    ____  ___
   / | / / ____/ __ \      / ____/ __ \    | |  / /  |/  /
  /  |/ / __/ / / / /_____/ / __/ / / /____| | / / /|_/ /
 / /|  / /___/ /_/ /_____/ /_/ / /_/ /_____/ |/ / /  / /
/_/ |_/_____/\____/      \____/\____/      |___/_/  /_/


NEO-GO-VM >
```

## Standalone
More information about standalone installation coming soon.

# Usage

```
    _   ____________        __________      _    ____  ___
   / | / / ____/ __ \      / ____/ __ \    | |  / /  |/  /
  /  |/ / __/ / / / /_____/ / __/ / / /____| | / / /|_/ /
 / /|  / /___/ /_/ /_____/ /_/ / /_/ /_____/ |/ / /  / /
/_/ |_/_____/\____/      \____/\____/      |___/_/  /_/


NEO-GO-VM > help

COMMAND    USAGE
astack     show alt stack details
break      place a breakpoint (> break 1)
cont       continue execution of the current loaded script
estack     show evaluation stack details
exit       exit the VM prompt
help       show available commands
ip         show the current instruction
istack     show invocation stack details
loadavm    load an avm script into the VM (> load /path/to/script.avm)
loadgo     compile and load a .go file into the VM (> load /path/to/file.go)
loadhex    load a hex string into the VM (> loadhex 006166 )
ops        show the opcodes of the current loaded program
run        execute the current loaded script
step       step (n) instruction in the program (> step 10)
```

### Loading in your script

To load an avm script into the VM:

```
NEO-GO-VM > loadavm ../contract.avm
READY: loaded 36 instructions
```

Run the script:

```
NEO-GO-VM > run
[
    {
        "value": 1,
        "type": "BigInteger"
    }
]
```

You can also directly compile and load `.go` files:

```
NEO-GO-VM > loadgo ../contract.go
READY: loaded 36 instructions
```

To make it even more complete, you can directly load hex strings into the VM:

```
NEO-GO-VM > loadhex 54c56b006c766b00527ac46c766b00c391640b006203005a616c756662030000616c7566
READY: loaded 36 instructions
NEO-GO-VM > run
[
    {
        "value": 10,
        "type": "BigInteger"
    }
]

```

### Running programs with arguments
You can invoke smart contracts with arguments. Take the following ***roll the dice*** smartcontract as example. 

```
package rollthedice

import "github.com/CityOfZion/neo-go/pkg/vm/api/runtime"

func Main(method string, args []interface{}) int {
    if method == "rollDice" {
        // args parameter is always of type []interface, hence we need to 
        // cast it to an int.
        rollDice(args[0].(int))
    }
    return 0
}

func rollDice(number int) {
    if number == 0 {
        runtime.Log("you rolled 0, better luck next time!")
    }
    if number == 1 {
        runtime.Log("you rolled 1, still better then 0!")
    }
    if number == 2 {
        runtime.Log("you rolled 2, coming closer..") 
    }
    if number == 3 {
        runtime.Log("Sweet you rolled 3. This dice has only 3 sides o_O")
    }
}
```

To invoke this contract we need to specify both the method and the arguments.

The first parameter (called method or operation) is always of type string. Notice that arguments can have different types, to make the VM aware of the type we need to specify it when calling `run`:

```
NEO-GO-VM > run rollDice int:1
```

> The method is always of type string, hence we don't need to specify the type.

To add more then 1 argument:

```
NEO-GO-VM > run someMethod int:1 int:2 string:foo string:bar
```

Current supported types:
- `int (int:1 int:100)`
- `string (string:foo string:this is a string)` 

### Debugging
The `neo-go-vm` provides a debugger to inspect your program in-depth.

Step 4 instructions.

```
NEO-GO-VM > step 4
at breakpoint 4 (Opush4)
NEO-GO-VM 4 >
```

Using just `step` will execute 1 instruction at a time.

```
NEO-GO-VM > step
instruction pointer at 5 (Odup)
NEO-GO-VM 5 >
```

To place breakpoints:

```
NEO-GO-VM > break 10
breakpoint added at instruction 10
NEO-GO-VM > cont
at breakpoint 10 (Osetitem)
NEO-GO-VM 10 > cont
```

Inspecting the evaluation stack:

```
NEO-GO-VM > estack
[
    {
        "value": [
            null,
            null,
            null,
            null,
            null,
            null,
            null
        ],
        "type": "Array"
    },
    {
        "value": 4,
        "type": "BigInteger"
    }
]
```

There are more stacks that you can inspect.
- `astack` alt stack
- `istack` invocation stack

