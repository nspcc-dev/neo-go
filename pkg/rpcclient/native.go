package rpcclient

// Various non-policy things from native contracts.

import (
	"crypto/elliptic"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nns"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// GetOraclePrice invokes `getPrice` method on a native Oracle contract.
func (c *Client) GetOraclePrice() (int64, error) {
	oracleHash, err := c.GetNativeContractHash(nativenames.Oracle)
	if err != nil {
		return 0, fmt.Errorf("failed to get native Oracle hash: %w", err)
	}
	return c.invokeNativeGetMethod(oracleHash, "getPrice")
}

// GetNNSPrice invokes `getPrice` method on a NeoNameService contract with the specified hash.
func (c *Client) GetNNSPrice(nnsHash util.Uint160) (int64, error) {
	return c.invokeNativeGetMethod(nnsHash, "getPrice")
}

// GetGasPerBlock invokes `getGasPerBlock` method on a native NEO contract.
func (c *Client) GetGasPerBlock() (int64, error) {
	return c.getFromNEO("getGasPerBlock")
}

// GetCandidateRegisterPrice invokes `getRegisterPrice` method on native NEO contract.
func (c *Client) GetCandidateRegisterPrice() (int64, error) {
	return c.getFromNEO("getRegisterPrice")
}

func (c *Client) getFromNEO(meth string) (int64, error) {
	neoHash, err := c.GetNativeContractHash(nativenames.Neo)
	if err != nil {
		return 0, fmt.Errorf("failed to get native NEO hash: %w", err)
	}
	return c.invokeNativeGetMethod(neoHash, meth)
}

// GetDesignatedByRole invokes `getDesignatedByRole` method on a native RoleManagement contract.
func (c *Client) GetDesignatedByRole(role noderoles.Role, index uint32) (keys.PublicKeys, error) {
	rmHash, err := c.GetNativeContractHash(nativenames.Designation)
	if err != nil {
		return nil, fmt.Errorf("failed to get native RoleManagement hash: %w", err)
	}
	arr, err := unwrap.Array(c.reader.Call(rmHash, "getDesignatedByRole", int64(role), index))
	if err != nil {
		return nil, err
	}
	pks := make(keys.PublicKeys, len(arr))
	for i, item := range arr {
		val, err := item.TryBytes()
		if err != nil {
			return nil, fmt.Errorf("invalid array element #%d: %s", i, item.Type())
		}
		pks[i], err = keys.NewPublicKeyFromBytes(val, elliptic.P256())
		if err != nil {
			return nil, err
		}
	}
	return pks, nil
}

// NNSResolve invokes `resolve` method on a NameService contract with the specified hash.
func (c *Client) NNSResolve(nnsHash util.Uint160, name string, typ nns.RecordType) (string, error) {
	if typ == nns.CNAME {
		return "", errors.New("can't resolve CNAME record type")
	}
	return unwrap.UTF8String(c.reader.Call(nnsHash, "resolve", name, int64(typ)))
}

// NNSIsAvailable invokes `isAvailable` method on a NeoNameService contract with the specified hash.
func (c *Client) NNSIsAvailable(nnsHash util.Uint160, name string) (bool, error) {
	return unwrap.Bool(c.reader.Call(nnsHash, "isAvailable", name))
}

// NNSGetAllRecords returns iterator over records for a given name from NNS service.
// First return value is the session ID, the second one is Iterator itself, the
// third one is an error. Use TraverseIterator method to traverse iterator values or
// TerminateSession to terminate opened iterator session. See TraverseIterator and
// TerminateSession documentation for more details.
func (c *Client) NNSGetAllRecords(nnsHash util.Uint160, name string) (uuid.UUID, result.Iterator, error) {
	return unwrap.SessionIterator(c.reader.Call(nnsHash, "getAllRecords", name))
}

// NNSUnpackedGetAllRecords returns a set of records for a given name from NNS service
// (config.DefaultMaxIteratorResultItems at max). It differs from NNSGetAllRecords in
// that no iterator session is used to retrieve values from iterator. Instead, unpacking
// VM script is created and invoked via `invokescript` JSON-RPC call.
func (c *Client) NNSUnpackedGetAllRecords(nnsHash util.Uint160, name string) ([]nns.RecordState, error) {
	arr, err := unwrap.Array(c.reader.CallAndExpandIterator(nnsHash, "getAllRecords", config.DefaultMaxIteratorResultItems, name))
	if err != nil {
		return nil, err
	}
	res := make([]nns.RecordState, len(arr))
	for i := range arr {
		rs, ok := arr[i].Value().([]stackitem.Item)
		if !ok {
			return nil, fmt.Errorf("failed to decode RecordState from stackitem #%d: not a struct", i)
		}
		if len(rs) != 3 {
			return nil, fmt.Errorf("failed to decode RecordState from stackitem #%d: wrong number of elements", i)
		}
		name, err := rs[0].TryBytes()
		if err != nil {
			return nil, fmt.Errorf("failed to decode RecordState from stackitem #%d: %w", i, err)
		}
		typ, err := rs[1].TryInteger()
		if err != nil {
			return nil, fmt.Errorf("failed to decode RecordState from stackitem #%d: %w", i, err)
		}
		data, err := rs[2].TryBytes()
		if err != nil {
			return nil, fmt.Errorf("failed to decode RecordState from stackitem #%d: %w", i, err)
		}
		u64Typ := typ.Uint64()
		if !typ.IsUint64() || u64Typ > 255 {
			return nil, fmt.Errorf("failed to decode RecordState from stackitem #%d: bad type", i)
		}
		res[i] = nns.RecordState{
			Name: string(name),
			Type: nns.RecordType(u64Typ),
			Data: string(data),
		}
	}
	return res, nil
}

// GetNotaryServiceFeePerKey returns a reward per notary request key for the designated
// notary nodes. It doesn't cache the result.
func (c *Client) GetNotaryServiceFeePerKey() (int64, error) {
	notaryHash, err := c.GetNativeContractHash(nativenames.Notary)
	if err != nil {
		return 0, fmt.Errorf("failed to get native Notary hash: %w", err)
	}
	return c.invokeNativeGetMethod(notaryHash, "getNotaryServiceFeePerKey")
}
