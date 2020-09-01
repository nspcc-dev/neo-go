/*
Package interop contains smart contract API functions.
Its subpackages can be imported into smart contracts written in Go to provide
various functionality. Upon compilation, functions from these packages will
be substituted with appropriate NeoVM system calls implemented by Neo. Usually
these system calls have additional price in NeoVM, so they're explicitly written
in the documentation of respective functions.

Note that unless written otherwise structures defined in this packages can't be
correctly created by new() or composite literals, they should be received from
some interop functions (and then used as parameters for some other interop
functions).
*/
package interop
