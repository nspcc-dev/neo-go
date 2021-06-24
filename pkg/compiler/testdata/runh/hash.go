package runh

import "github.com/nspcc-dev/neo-go/pkg/interop"

// RuntimeHash possibly returns some hash at runtime.
func RuntimeHash() interop.Hash160 {
	return nil
}

// RuntimeHashArgs possibly returns some hash at runtime.
func RuntimeHashArgs(s string) interop.Hash160 {
	return nil
}
