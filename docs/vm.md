# NEO-GO-VM

A cross platform virtual machine implementation for `NEF` compatible programs. 

# Installation

VM is provided as part of neo-go binary, so usual neo-go build instructions
are applicable.

# Running the VM

Start the virtual machine:

```
$ ./bin/neo-go vm

    _   ____________        __________      _    ____  ___
   / | / / ____/ __ \      / ____/ __ \    | |  / /  |/  /
  /  |/ / __/ / / / /_____/ / __/ / / /____| | / / /|_/ /
 / /|  / /___/ /_/ /_____/ /_/ / /_/ /_____/ |/ / /  / /
/_/ |_/_____/\____/      \____/\____/      |___/_/  /_/


NEO-GO-VM >
```

# Usage

```
    _   ____________        __________      _    ____  ___
   / | / / ____/ __ \      / ____/ __ \    | |  / /  |/  /
  /  |/ / __/ / / / /_____/ / __/ / / /____| | / / /|_/ /
 / /|  / /___/ /_/ /_____/ /_/ / /_/ /_____/ |/ / /  / /
/_/ |_/_____/\____/      \____/\____/      |___/_/  /_/


NEO-GO-VM > help

Commands:
  astack       Show alt stack contents
  break        Place a breakpoint
  clear        clear the screen
  cont         Continue execution of the current loaded script
  estack       Show evaluation stack contents
  exit         Exit the VM prompt
  help         display help
  ip           Show current instruction
  istack       Show invocation stack contents
  loadnef      Load an avm script in NEF format into the VM
  loadgo       Compile and load a Go file into the VM
  loadhex      Load a hex-encoded script string into the VM
  ops          Dump opcodes of the current loaded program
  run          Execute the current loaded script
  step         Step (n) instruction in the program


```

You can get help for each command and its parameters adding `help` as a
parameter to the command:

```
NEO-GO-VM > step help

Usage: step [<n>]
<n> is optional parameter to specify number of instructions to run, example:
> step 10

```

## Loading in your script

To load an avm script in NEF format into the VM:

```
NEO-GO-VM > loadnef ../contract.nef
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

## Running programs with arguments
You can invoke smart contracts with arguments. Take the following ***roll the dice*** smartcontract as example. 

```
package rollthedice

import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"

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

The first parameter (called method or operation) is always of type
string. Notice that arguments can have different types, they can inferred
automatically (please refer to the `run` command help), but in you need to
pass parameter of specific type you can specify it in `run`'s arguments:

```
NEO-GO-VM > run rollDice int:1
```

> The method is always of type string, hence we don't need to specify the type.

To add more than 1 argument:

```
NEO-GO-VM > run someMethod int:1 int:2 string:foo string:bar
```

Currently supported types:
- `bool (bool:false and bool:true)`
- `int (int:1 int:100)`
- `string (string:foo string:this is a string)` 

## Debugging
The `neo-go-vm` provides a debugger to inspect your program in-depth.


### Stepping through the program
Step 4 instructions.

```
NEO-GO-VM > step 4
at breakpoint 3 (DUPFROMALTSTACK)
NEO-GO-VM 3 >
```

Using just `step` will execute 1 instruction at a time.

```
NEO-GO-VM 3 > step
at breakpoint 4 (PUSH0)
NEO-GO-VM 4 >
```

### Breakpoints

To place breakpoints:

```
NEO-GO-VM > break 10
breakpoint added at instruction 10
NEO-GO-VM > cont
at breakpoint 10 (SETITEM)
NEO-GO-VM 10 > cont
```

## Inspecting stack

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

