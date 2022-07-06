package client

// Various non-policy things from native contracts.

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client/nns"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
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
	result, err := c.InvokeFunction(rmHash, "getDesignatedByRole", []smartcontract.Parameter{
		{
			Type:  smartcontract.IntegerType,
			Value: big.NewInt(int64(role)),
		},
		{
			Type:  smartcontract.IntegerType,
			Value: big.NewInt(int64(index)),
		},
	}, nil)
	if err != nil {
		return nil, err
	}
	err = getInvocationError(result)
	if err != nil {
		return nil, fmt.Errorf("`getDesignatedByRole`: %w", err)
	}
	return topPublicKeysFromStack(result.Stack)
}

// NNSResolve invokes `resolve` method on a NameService contract with the specified hash.
func (c *Client) NNSResolve(nnsHash util.Uint160, name string, typ nns.RecordType) (string, error) {
	if typ == nns.CNAME {
		return "", errors.New("can't resolve CNAME record type")
	}
	result, err := c.InvokeFunction(nnsHash, "resolve", []smartcontract.Parameter{
		{
			Type:  smartcontract.StringType,
			Value: name,
		},
		{
			Type:  smartcontract.IntegerType,
			Value: big.NewInt(int64(typ)),
		},
	}, nil)
	if err != nil {
		return "", err
	}
	err = getInvocationError(result)
	if err != nil {
		return "", fmt.Errorf("`resolve`: %w", err)
	}
	return topStringFromStack(result.Stack)
}

// NNSIsAvailable invokes `isAvailable` method on a NeoNameService contract with the specified hash.
func (c *Client) NNSIsAvailable(nnsHash util.Uint160, name string) (bool, error) {
	result, err := c.InvokeFunction(nnsHash, "isAvailable", []smartcontract.Parameter{
		{
			Type:  smartcontract.StringType,
			Value: name,
		},
	}, nil)
	if err != nil {
		return false, err
	}
	err = getInvocationError(result)
	if err != nil {
		return false, fmt.Errorf("`isAvailable`: %w", err)
	}
	return topBoolFromStack(result.Stack)
}

// NNSGetAllRecords returns iterator over records for a given name from NNS service.
// First return value is the session ID, the second one is Iterator itself, the
// third one is an error. Use TraverseIterator method to traverse iterator values or
// TerminateSession to terminate opened iterator session. See TraverseIterator and
// TerminateSession documentation for more details.
func (c *Client) NNSGetAllRecords(nnsHash util.Uint160, name string) (uuid.UUID, result.Iterator, error) {
	res, err := c.InvokeFunction(nnsHash, "getAllRecords", []smartcontract.Parameter{
		{
			Type:  smartcontract.StringType,
			Value: name,
		},
	}, nil)
	if err != nil {
		return uuid.UUID{}, result.Iterator{}, err
	}
	err = getInvocationError(res)
	if err != nil {
		return uuid.UUID{}, result.Iterator{}, err
	}

	iter, err := topIteratorFromStack(res.Stack)
	return res.Session, iter, err
}

// NNSUnpackedGetAllRecords returns all records for a given name from NNS service. It differs from
// NNSGetAllRecords in that no iterator session is used to retrieve values from iterator. Instead,
// unpacking VM script is created and invoked via `invokescript` JSON-RPC call.
func (c *Client) NNSUnpackedGetAllRecords(nnsHash util.Uint160, name string) ([]nns.RecordState, error) {
	result, err := c.InvokeAndPackIteratorResults(nnsHash, "getAllRecords", []smartcontract.Parameter{
		{
			Type:  smartcontract.StringType,
			Value: name,
		},
	}, nil)
	if err != nil {
		return nil, err
	}
	err = getInvocationError(result)
	if err != nil {
		return nil, err
	}

	arr, err := topIterableFromStack(result.Stack, nns.RecordState{})
	if err != nil {
		return nil, fmt.Errorf("failed to get token IDs from stack: %w", err)
	}
	rss := make([]nns.RecordState, len(arr))
	for i := range rss {
		rss[i] = arr[i].(nns.RecordState)
	}
	return rss, nil
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
