/*
Package interop contains smart contract API functions and type synonyms.
Its subpackages can be imported into smart contracts written in Go to provide
various functionality. Upon compilation, functions from these packages will
be substituted with appropriate NeoVM system calls implemented by Neo. Usually
these system calls have additional price in NeoVM, so they're explicitly written
in the documentation of respective functions.

Types defined here are used for proper manifest generation. Here is how Go types
correspond to smartcontract and VM types:

	int-like - Integer
	bool - Boolean
	[]byte - ByteArray (Buffer in VM)
	string - String (ByteString in VM)
	(interface{})(nil) - Any
	non-byte slice - Array
	map[K]V - map

Other types are defined explicitly in this pkg:
[Hash160], [Hash256], [Interface], [PublicKey], [Signature].

Note that unless written otherwise structures defined in this packages can't be
correctly created by new() or composite literals, they should be received from
some interop functions (and then used as parameters for some other interop
functions).
*/
package interop
