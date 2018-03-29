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
ip         show the current instruction
load       load a script into the VM (> load /path/to/script.avm)
resume     resume the current loaded script
help       show available commands
exit       exit the VM prompt
break      place a breakpoint (> break 1)
stack      show stack details
run        execute the current loaded script
step       step (n) instruction in the program (> step 10)
```

### Loading in your script

To load a script into the VM:

```
NEO-GO-VM > load ../contract.avm
READY
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

### Debugging
The `neo-go-vm` provides a debugger to inspect your program in-depth.

Step 4 instructions.

```
NEO-GO-VM > step 4
at breakpoint 4 (Opush4)
```

Using just `step` will execute 1 instruction at a time.

```
NEO-GO-VM > step
instruction pointer at 5 (Odup)
```

To place breakpoints:

```
NEO-GO-VM > break 10
breakpoint added at instruction 10
NEO-GO-VM > resume
at breakpoint 10 (Osetitem)
```

Inspecting the stack:

```
NEO-GO-VM > stack
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

And a lot more features coming next weeks..
