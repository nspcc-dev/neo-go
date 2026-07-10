/*
Package tempstorage provides an interface to TemporaryStorage native contract.
*/
package tempstorage

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

// Hash represents TemporaryStorage contract hash.
const Hash = "\xbb\xc2\x15\x1b\xe9\x1a\x8b\x9c\xad\x8e\xa3\x5b\xa5\xd6\x72\xfc\x01\x35\x46\x93"

// Put represents `put` method of TemporaryStorage native contract.
func Put(key, value []byte, validTill int) {
	neogointernal.CallWithTokenNoRet(Hash, "put", int(contract.WriteStates), key, value, validTill)
}

// Get represents `get` method of TemporaryStorage native contract.
func Get(key []byte) []byte {
	return neogointernal.CallWithToken(Hash, "get", int(contract.ReadStates), key).([]byte)
}

// GetByHash represents `get` method of TemporaryStorage native contract.
func GetByHash(hash interop.Hash160, key []byte) []byte {
	return neogointernal.CallWithToken(Hash, "get", int(contract.ReadStates), hash, key).([]byte)
}

// GetExpiration represents `getExpiration` method of TemporaryStorage native contract.
func GetExpiration(key []byte) int {
	return neogointernal.CallWithToken(Hash, "getExpiration", int(contract.ReadStates), key).(int)
}

// GetExpirationByHash represents `getExpiration` method of TemporaryStorage native contract.
func GetExpirationByHash(hash interop.Hash160, key []byte) int {
	return neogointernal.CallWithToken(Hash, "getExpiration", int(contract.ReadStates), hash, key).(int)
}

// Delete represents `delete` method of TemporaryStorage native contract.
func Delete(key []byte) {
	neogointernal.CallWithTokenNoRet(Hash, "delete", int(contract.WriteStates), key)
}

// Find represents `find` method of TemporaryStorage native contract.
func Find(prefix []byte, opts storage.FindFlags) iterator.Iterator {
	return neogointernal.CallWithToken(Hash, "find", int(contract.ReadStates), prefix, opts).(iterator.Iterator)
}

// FindByHash represents `find` method of TemporaryStorage native contract.
func FindByHash(hash interop.Hash160, prefix []byte, opts storage.FindFlags) iterator.Iterator {
	return neogointernal.CallWithToken(Hash, "find", int(contract.ReadStates), hash, prefix, opts).(iterator.Iterator)
}

// Renew represents `renew` method of TemporaryStorage native contract.
func Renew(key []byte, validTill int) {
	neogointernal.CallWithTokenNoRet(Hash, "renew", int(contract.WriteStates), key, validTill)
}
