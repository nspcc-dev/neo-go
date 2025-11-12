package neofs

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/oracle"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/std"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

const (
	ownerKey  = "owner"
	objectKey = "object"
)

// _deploy is called after contract deployment or update. It initializes the
// contract owner from the deployment data.
func _deploy(data any, isUpdate bool) { // nolint: unused
	if isUpdate {
		return
	}
	owner := data.(interop.Hash160)
	if len(owner) != interop.Hash160Len {
		panic("invalid owner address")
	}
	storage.Put(storage.GetContext(), ownerKey, owner)
}

// GetObject returns the NeoFS object stored in the contract storage.
func GetObject() []byte {
	return storage.Get(storage.GetReadOnlyContext(), objectKey).([]byte)
}

// SaveObject requests a NeoFS object specified by the container ID (cid) and
// object ID (oid) using the Oracle service. The object data will be saved to
// the contract storage once the oracle response is received. Only the contract
// owner can call this method.
func SaveObject(cid, oid string) {
	checkAccess()

	url := "neofs:" + cid + "/" + oid
	oracle.Request(url, nil, "saveObjectCB", nil, 15*oracle.MinimumResponseGas)
}

// SaveObjectCB is called by the Oracle native contract when the NeoFS object
// request is finished. It validates that the caller is the oracle contract,
// checks the response code, and saves the object data to contract storage.
func SaveObjectCB(url string, data any, code int, result []byte) {
	if string(runtime.GetCallingScriptHash()) != oracle.Hash {
		panic("called by non-oracle contract")
	}
	if code != oracle.Success {
		panic("request failed for " + url + " with code " + std.Itoa(code, 10))
	}

	storage.Put(storage.GetContext(), objectKey, result)
}

// DeleteObject removes the stored NeoFS object from the contract storage.
// Only the contract owner can call this method.
func DeleteObject() {
	checkAccess()

	storage.Delete(storage.GetContext(), objectKey)
}

// Owner returns the contract owner's script hash.
func Owner() interop.Hash160 {
	return storage.Get(storage.GetReadOnlyContext(), ownerKey).(interop.Hash160)
}

// checkAccess verifies that the contract owner has witnessed the transaction.
// It panics if the witness check fails.
func checkAccess() {
	owner := storage.Get(storage.GetReadOnlyContext(), ownerKey).(interop.Hash160)
	if !runtime.CheckWitness(owner) {
		panic("not allowed user")
	}
}
