# NEO-GO Compiler

The neo-go compiler compiles Go programs to bytecode that the NEO virtual machine can understand.

> The neo-go compiler is under very active development and will be updated on a weekly basis.

## Usage

```
./bin/neo-go contract compile -i mycontract.go --out /Users/foo/bar/contract.avm
```

## Currently supported
- type checker
- multiple assigns
- types int, string and bool
- struct types + method receives
- functions
- composite literals `[]int, []string`
- basic if statements
- binary expressions.
- return statements

## Not yet implemented
- for loops
- ranges
- builtins (append, len, ..)
- blockchain helpers (sha256, storage, ..)
- import packages

## Not supported
Due to the limitations of the NEO virtual machine, features listed below will not be supported.
- channels 
- goroutines
- multiple returns 

## How to report bugs
1. Make a proper testcase (example testcases can be found in the tests folder)
2. Create an issue on Github 
3. Make a PR with a reference to the created issue, containing the testcase that proves the bug
4. Either you fix the bug yourself or wait for patch that solves the problem
