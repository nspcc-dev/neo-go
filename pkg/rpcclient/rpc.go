package rpcclient

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

var errNetworkNotInitialized = errors.New("RPC client network is not initialized")

// CalculateNetworkFee calculates network fee for the transaction. The transaction may
// have empty witnesses for contract signers and may have only verification scripts
// filled for standard sig/multisig signers.
func (c *Client) CalculateNetworkFee(tx *transaction.Transaction) (int64, error) {
	var (
		params = []any{tx.Bytes()}
		resp   = new(result.NetworkFee)
	)
	if err := c.performRequest("calculatenetworkfee", params, resp); err != nil {
		return 0, err
	}
	return resp.Value, nil
}

// GetApplicationLog returns a contract log based on the specified txid.
func (c *Client) GetApplicationLog(hash util.Uint256, trig *trigger.Type) (*result.ApplicationLog, error) {
	var (
		params = []any{hash.StringLE()}
		resp   = new(result.ApplicationLog)
	)
	if trig != nil {
		params = append(params, trig.String())
	}
	if err := c.performRequest("getapplicationlog", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetBestBlockHash returns the hash of the tallest block in the blockchain.
func (c *Client) GetBestBlockHash() (util.Uint256, error) {
	var resp = util.Uint256{}
	if err := c.performRequest("getbestblockhash", nil, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetBlockCount returns the number of blocks in the blockchain.
func (c *Client) GetBlockCount() (uint32, error) {
	var resp uint32
	if err := c.performRequest("getblockcount", nil, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetBlockByIndex returns a block by its height. In-header stateroot option
// must be initialized with Init before calling this method.
func (c *Client) GetBlockByIndex(index uint32) (*block.Block, error) {
	return c.getBlock(index)
}

// GetBlockByHash returns a block by its hash. In-header stateroot option
// must be initialized with Init before calling this method.
func (c *Client) GetBlockByHash(hash util.Uint256) (*block.Block, error) {
	return c.getBlock(hash.StringLE())
}

func (c *Client) getBlock(param any) (*block.Block, error) {
	var (
		resp []byte
		err  error
		b    *block.Block
	)
	if err = c.performRequest("getblock", []any{param}, &resp); err != nil {
		return nil, err
	}
	r := io.NewBinReaderFromBuf(resp)
	sr, err := c.stateRootInHeader()
	if err != nil {
		return nil, err
	}
	b = block.New(sr)
	b.DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}
	return b, nil
}

// GetBlockByIndexVerbose returns a block wrapper with additional metadata by
// its height. In-header stateroot option must be initialized with Init before
// calling this method.
// NOTE: to get transaction.ID and transaction.Size, use t.Hash() and io.GetVarSize(t) respectively.
func (c *Client) GetBlockByIndexVerbose(index uint32) (*result.Block, error) {
	return c.getBlockVerbose(index)
}

// GetBlockByHashVerbose returns a block wrapper with additional metadata by
// its hash. In-header stateroot option must be initialized with Init before
// calling this method.
func (c *Client) GetBlockByHashVerbose(hash util.Uint256) (*result.Block, error) {
	return c.getBlockVerbose(hash.StringLE())
}

func (c *Client) getBlockVerbose(param any) (*result.Block, error) {
	var (
		params = []any{param, 1} // 1 for verbose.
		resp   = &result.Block{}
		err    error
	)
	sr, err := c.stateRootInHeader()
	if err != nil {
		return nil, err
	}
	resp.Header.StateRootEnabled = sr
	if err = c.performRequest("getblock", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetBlockHash returns the hash value of the corresponding block based on the specified index.
func (c *Client) GetBlockHash(index uint32) (util.Uint256, error) {
	var (
		params = []any{index}
		resp   = util.Uint256{}
	)
	if err := c.performRequest("getblockhash", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetBlockHeader returns the corresponding block header information from a serialized hex string
// according to the specified script hash. In-header stateroot option must be
// initialized with Init before calling this method.
func (c *Client) GetBlockHeader(hash util.Uint256) (*block.Header, error) {
	var (
		params = []any{hash.StringLE()}
		resp   []byte
		h      *block.Header
	)
	if err := c.performRequest("getblockheader", params, &resp); err != nil {
		return nil, err
	}
	sr, err := c.stateRootInHeader()
	if err != nil {
		return nil, err
	}
	r := io.NewBinReaderFromBuf(resp)
	h = new(block.Header)
	h.StateRootEnabled = sr
	h.DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}
	return h, nil
}

// GetBlockHeaderCount returns the number of headers in the main chain.
func (c *Client) GetBlockHeaderCount() (uint32, error) {
	var resp uint32
	if err := c.performRequest("getblockheadercount", nil, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetBlockHeaderVerbose returns the corresponding block header information from a Json format string
// according to the specified script hash. In-header stateroot option must be
// initialized with Init before calling this method.
func (c *Client) GetBlockHeaderVerbose(hash util.Uint256) (*result.Header, error) {
	var (
		params = []any{hash.StringLE(), 1}
		resp   = &result.Header{}
	)
	if err := c.performRequest("getblockheader", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetBlockSysFee returns the system fees of the block based on the specified index.
// This method is only supported by NeoGo servers.
func (c *Client) GetBlockSysFee(index uint32) (fixedn.Fixed8, error) {
	var (
		params = []any{index}
		resp   fixedn.Fixed8
	)
	if err := c.performRequest("getblocksysfee", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetConnectionCount returns the current number of the connections for the node.
func (c *Client) GetConnectionCount() (int, error) {
	var resp int

	if err := c.performRequest("getconnectioncount", nil, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetCommittee returns the current public keys of NEO nodes in the committee.
func (c *Client) GetCommittee() (keys.PublicKeys, error) {
	var resp = new(keys.PublicKeys)

	if err := c.performRequest("getcommittee", nil, resp); err != nil {
		return nil, err
	}
	return *resp, nil
}

// GetContractStateByHash queries contract information according to the contract script hash.
func (c *Client) GetContractStateByHash(hash util.Uint160) (*state.Contract, error) {
	return c.getContractState(hash.StringLE())
}

// GetContractStateByAddressOrName queries contract information using the contract
// address or name. Notice that name-based queries work only for native contracts,
// non-native ones can't be requested this way.
func (c *Client) GetContractStateByAddressOrName(addressOrName string) (*state.Contract, error) {
	return c.getContractState(addressOrName)
}

// GetContractStateByID queries contract information according to the contract ID.
// Notice that this is supported by all servers only for native contracts,
// non-native ones can be requested only from NeoGo servers.
func (c *Client) GetContractStateByID(id int32) (*state.Contract, error) {
	return c.getContractState(id)
}

// getContractState is an internal representation of GetContractStateBy* methods.
func (c *Client) getContractState(param any) (*state.Contract, error) {
	var (
		params = []any{param}
		resp   = &state.Contract{}
	)
	if err := c.performRequest("getcontractstate", params, resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetNativeContracts queries information about native contracts.
func (c *Client) GetNativeContracts() ([]state.NativeContract, error) {
	var resp []state.NativeContract
	if err := c.performRequest("getnativecontracts", nil, &resp); err != nil {
		return resp, err
	}

	// Update native contract hashes.
	c.cacheLock.Lock()
	for _, cs := range resp {
		c.cache.nativeHashes[cs.Manifest.Name] = cs.Hash
	}
	c.cacheLock.Unlock()

	return resp, nil
}

// GetNEP11Balances is a wrapper for getnep11balances RPC.
func (c *Client) GetNEP11Balances(address util.Uint160) (*result.NEP11Balances, error) {
	params := []any{address.StringLE()}
	resp := new(result.NEP11Balances)
	if err := c.performRequest("getnep11balances", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetNEP17Balances is a wrapper for getnep17balances RPC.
func (c *Client) GetNEP17Balances(address util.Uint160) (*result.NEP17Balances, error) {
	params := []any{address.StringLE()}
	resp := new(result.NEP17Balances)
	if err := c.performRequest("getnep17balances", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetNEP11Properties is a wrapper for getnep11properties RPC. We recommend using
// nep11 package and Properties method there to receive proper VM types and work with them.
// This method is provided mostly for the sake of completeness. For well-known
// attributes like "description", "image", "name" and "tokenURI" it returns strings,
// while for all others []byte (which can be nil).
func (c *Client) GetNEP11Properties(asset util.Uint160, token []byte) (map[string]any, error) {
	params := []any{asset.StringLE(), hex.EncodeToString(token)}
	resp := make(map[string]any)
	if err := c.performRequest("getnep11properties", params, &resp); err != nil {
		return nil, err
	}
	for k, v := range resp {
		if v == nil {
			continue
		}
		str, ok := v.(string)
		if !ok {
			return nil, errors.New("value is not a string")
		}
		if result.KnownNEP11Properties[k] {
			continue
		}
		val, err := base64.StdEncoding.DecodeString(str)
		if err != nil {
			return nil, err
		}
		resp[k] = val
	}
	return resp, nil
}

// GetNEP11Transfers is a wrapper for getnep11transfers RPC. Address parameter
// is mandatory, while all others are optional. Limit and page parameters are
// only supported by NeoGo servers and can only be specified with start and stop.
func (c *Client) GetNEP11Transfers(address util.Uint160, start, stop *uint64, limit, page *int) (*result.NEP11Transfers, error) {
	params, err := packTransfersParams(address, start, stop, limit, page)
	if err != nil {
		return nil, err
	}
	resp := new(result.NEP11Transfers)
	if err := c.performRequest("getnep11transfers", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func packTransfersParams(address util.Uint160, start, stop *uint64, limit, page *int) ([]any, error) {
	params := []any{address.StringLE()}
	if start != nil {
		params = append(params, *start)
		if stop != nil {
			params = append(params, *stop)
			if limit != nil {
				params = append(params, *limit)
				if page != nil {
					params = append(params, *page)
				}
			} else if page != nil {
				return nil, errors.New("bad parameters")
			}
		} else if limit != nil || page != nil {
			return nil, errors.New("bad parameters")
		}
	} else if stop != nil || limit != nil || page != nil {
		return nil, errors.New("bad parameters")
	}
	return params, nil
}

// GetNEP17Transfers is a wrapper for getnep17transfers RPC. Address parameter
// is mandatory while all the others are optional. Start and stop parameters
// are supported since neo-go 0.77.0 and limit and page since neo-go 0.78.0.
// These parameters are positional in the JSON-RPC call. For example, you can't specify the limit
// without specifying start/stop first.
func (c *Client) GetNEP17Transfers(address util.Uint160, start, stop *uint64, limit, page *int) (*result.NEP17Transfers, error) {
	params, err := packTransfersParams(address, start, stop, limit, page)
	if err != nil {
		return nil, err
	}
	resp := new(result.NEP17Transfers)
	if err := c.performRequest("getnep17transfers", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetPeers returns a list of the nodes that the node is currently connected to/disconnected from.
func (c *Client) GetPeers() (*result.GetPeers, error) {
	var resp = &result.GetPeers{}

	if err := c.performRequest("getpeers", nil, resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetRawMemPool returns a list of unconfirmed transactions in the memory.
func (c *Client) GetRawMemPool() ([]util.Uint256, error) {
	var resp = new([]util.Uint256)

	if err := c.performRequest("getrawmempool", nil, resp); err != nil {
		return *resp, err
	}
	return *resp, nil
}

// GetRawTransaction returns a transaction by hash.
func (c *Client) GetRawTransaction(hash util.Uint256) (*transaction.Transaction, error) {
	var (
		params = []any{hash.StringLE()}
		resp   []byte
		err    error
	)
	if err = c.performRequest("getrawtransaction", params, &resp); err != nil {
		return nil, err
	}
	tx, err := transaction.NewTransactionFromBytes(resp)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// GetRawTransactionVerbose returns a transaction wrapper with additional
// metadata by transaction's hash.
// NOTE: to get transaction.ID and transaction.Size, use t.Hash() and io.GetVarSize(t) respectively.
func (c *Client) GetRawTransactionVerbose(hash util.Uint256) (*result.TransactionOutputRaw, error) {
	var (
		params = []any{hash.StringLE(), 1} // 1 for verbose.
		resp   = &result.TransactionOutputRaw{}
		err    error
	)
	if err = c.performRequest("getrawtransaction", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetProof returns existence proof of storage item state by the given stateroot
// historical contract hash and historical item key.
func (c *Client) GetProof(stateroot util.Uint256, historicalContractHash util.Uint160, historicalKey []byte) (*result.ProofWithKey, error) {
	var (
		params = []any{stateroot.StringLE(), historicalContractHash.StringLE(), historicalKey}
		resp   = &result.ProofWithKey{}
	)
	if err := c.performRequest("getproof", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// VerifyProof returns value by the given stateroot and proof.
func (c *Client) VerifyProof(stateroot util.Uint256, proof *result.ProofWithKey) ([]byte, error) {
	var (
		params = []any{stateroot.StringLE(), proof.String()}
		resp   []byte
	)
	if err := c.performRequest("verifyproof", params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetState returns historical contract storage item state by the given stateroot,
// historical contract hash and historical item key.
func (c *Client) GetState(stateroot util.Uint256, historicalContractHash util.Uint160, historicalKey []byte) ([]byte, error) {
	var (
		params = []any{stateroot.StringLE(), historicalContractHash.StringLE(), historicalKey}
		resp   []byte
	)
	if err := c.performRequest("getstate", params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// FindStates returns historical contract storage item states by the given stateroot,
// historical contract hash and historical prefix. If `start` path is specified, items
// starting from `start` path are being returned (excluding item located at the start path).
// If `maxCount` specified, the maximum number of items to be returned equals to `maxCount`.
func (c *Client) FindStates(stateroot util.Uint256, historicalContractHash util.Uint160, historicalPrefix []byte,
	start []byte, maxCount *int) (result.FindStates, error) {
	if historicalPrefix == nil {
		historicalPrefix = []byte{}
	}
	var (
		params = []any{stateroot.StringLE(), historicalContractHash.StringLE(), historicalPrefix}
		resp   result.FindStates
	)
	if start == nil && maxCount != nil {
		start = []byte{}
	}
	if start != nil {
		params = append(params, start)
	}
	if maxCount != nil {
		params = append(params, *maxCount)
	}
	if err := c.performRequest("findstates", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetStateRootByHeight returns the state root for the specified height.
func (c *Client) GetStateRootByHeight(height uint32) (*state.MPTRoot, error) {
	return c.getStateRoot(height)
}

// GetStateRootByBlockHash returns the state root for the block with the specified hash.
func (c *Client) GetStateRootByBlockHash(hash util.Uint256) (*state.MPTRoot, error) {
	return c.getStateRoot(hash)
}

func (c *Client) getStateRoot(param any) (*state.MPTRoot, error) {
	var resp = new(state.MPTRoot)
	if err := c.performRequest("getstateroot", []any{param}, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetStateHeight returns the current validated and local node state height.
func (c *Client) GetStateHeight() (*result.StateHeight, error) {
	var resp = new(result.StateHeight)

	if err := c.performRequest("getstateheight", nil, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetStorageByID returns the stored value according to the contract ID and the stored key.
func (c *Client) GetStorageByID(id int32, key []byte) ([]byte, error) {
	return c.getStorage([]any{id, key})
}

// GetStorageByHash returns the stored value according to the contract script hash and the stored key.
func (c *Client) GetStorageByHash(hash util.Uint160, key []byte) ([]byte, error) {
	return c.getStorage([]any{hash.StringLE(), key})
}

func (c *Client) getStorage(params []any) ([]byte, error) {
	var resp []byte
	if err := c.performRequest("getstorage", params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetStorageByIDHistoric returns the historical stored value according to the
// contract ID and, stored key and specified stateroot.
func (c *Client) GetStorageByIDHistoric(root util.Uint256, id int32, key []byte) ([]byte, error) {
	return c.getStorageHistoric([]any{root.StringLE(), id, key})
}

// GetStorageByHashHistoric returns the historical stored value according to the
// contract script hash, the stored key and specified stateroot.
func (c *Client) GetStorageByHashHistoric(root util.Uint256, hash util.Uint160, key []byte) ([]byte, error) {
	return c.getStorageHistoric([]any{root.StringLE(), hash.StringLE(), key})
}

func (c *Client) getStorageHistoric(params []any) ([]byte, error) {
	var resp []byte
	if err := c.performRequest("getstoragehistoric", params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// FindStorageByHash returns contract storage items by the given contract hash and prefix.
// If `start` index is specified, items starting from `start` index are being returned
// (including item located at the start index).
func (c *Client) FindStorageByHash(contractHash util.Uint160, prefix []byte, start *int) (result.FindStorage, error) {
	var params = []any{contractHash.StringLE(), prefix}
	if start != nil {
		params = append(params, *start)
	}
	return c.findStorage(params)
}

// FindStorageByID returns contract storage items by the given contract ID and prefix.
// If `start` index is specified, items starting from `start` index are being returned
// (including item located at the start index).
func (c *Client) FindStorageByID(contractID int32, prefix []byte, start *int) (result.FindStorage, error) {
	var params = []any{contractID, prefix}
	if start != nil {
		params = append(params, *start)
	}
	return c.findStorage(params)
}

func (c *Client) findStorage(params []any) (result.FindStorage, error) {
	var resp result.FindStorage
	if err := c.performRequest("findstorage", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// FindStorageByHashHistoric returns historical contract storage items by the given stateroot,
// historical contract hash and historical prefix. If `start` index is specified, then items
// starting from `start` index are being returned (including item located at the start index).
func (c *Client) FindStorageByHashHistoric(stateroot util.Uint256, historicalContractHash util.Uint160, historicalPrefix []byte,
	start *int) (result.FindStorage, error) {
	if historicalPrefix == nil {
		historicalPrefix = []byte{}
	}
	var params = []any{stateroot.StringLE(), historicalContractHash.StringLE(), historicalPrefix}
	if start != nil {
		params = append(params, start)
	}
	return c.findStorageHistoric(params)
}

// FindStorageByIDHistoric returns historical contract storage items by the given stateroot,
// historical contract ID and historical prefix. If `start` index is specified, then items
// starting from `start` index are being returned (including item located at the start index).
func (c *Client) FindStorageByIDHistoric(stateroot util.Uint256, historicalContractID int32, historicalPrefix []byte,
	start *int) (result.FindStorage, error) {
	if historicalPrefix == nil {
		historicalPrefix = []byte{}
	}
	var params = []any{stateroot.StringLE(), historicalContractID, historicalPrefix}
	if start != nil {
		params = append(params, start)
	}
	return c.findStorageHistoric(params)
}

func (c *Client) findStorageHistoric(params []any) (result.FindStorage, error) {
	var resp result.FindStorage
	if err := c.performRequest("findstoragehistoric", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetTransactionHeight returns the block index where the transaction is found.
func (c *Client) GetTransactionHeight(hash util.Uint256) (uint32, error) {
	var (
		params = []any{hash.StringLE()}
		resp   uint32
	)
	if err := c.performRequest("gettransactionheight", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetUnclaimedGas returns the unclaimed GAS amount for the specified address.
func (c *Client) GetUnclaimedGas(address string) (result.UnclaimedGas, error) {
	var (
		params = []any{address}
		resp   result.UnclaimedGas
	)
	if err := c.performRequest("getunclaimedgas", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetCandidates returns the current list of NEO candidate node with voting data and
// validator status.
func (c *Client) GetCandidates() ([]result.Candidate, error) {
	var resp = new([]result.Candidate)

	if err := c.performRequest("getcandidates", nil, resp); err != nil {
		return nil, err
	}
	return *resp, nil
}

// GetNextBlockValidators returns the current NEO consensus nodes information and voting data.
func (c *Client) GetNextBlockValidators() ([]result.Validator, error) {
	var resp = new([]result.Validator)

	if err := c.performRequest("getnextblockvalidators", nil, resp); err != nil {
		return nil, err
	}
	return *resp, nil
}

// GetVersion returns the version information about the queried node.
func (c *Client) GetVersion() (*result.Version, error) {
	var resp = &result.Version{}

	if err := c.performRequest("getversion", nil, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// InvokeScript returns the result of the given script after running it true the VM.
// NOTE: This is a test invoke and will not affect the blockchain.
func (c *Client) InvokeScript(script []byte, signers []transaction.Signer) (*result.Invoke, error) {
	var p = []any{script}
	return c.invokeSomething("invokescript", p, signers)
}

// InvokeScriptAtHeight returns the result of the given script after running it
// true the VM using the provided chain state retrieved from the specified chain
// height.
// NOTE: This is a test invoke and will not affect the blockchain.
func (c *Client) InvokeScriptAtHeight(height uint32, script []byte, signers []transaction.Signer) (*result.Invoke, error) {
	var p = []any{height, script}
	return c.invokeSomething("invokescripthistoric", p, signers)
}

// InvokeScriptWithState returns the result of the given script after running it
// true the VM using the provided chain state retrieved from the specified
// state root or block hash.
// NOTE: This is a test invoke and will not affect the blockchain.
func (c *Client) InvokeScriptWithState(stateOrBlock util.Uint256, script []byte, signers []transaction.Signer) (*result.Invoke, error) {
	var p = []any{stateOrBlock.StringLE(), script}
	return c.invokeSomething("invokescripthistoric", p, signers)
}

// InvokeFunction returns the results after calling the smart contract scripthash
// with the given operation and parameters.
// NOTE: this is test invoke and will not affect the blockchain.
func (c *Client) InvokeFunction(contract util.Uint160, operation string, params []smartcontract.Parameter, signers []transaction.Signer) (*result.Invoke, error) {
	var p = []any{contract.StringLE(), operation, params}
	return c.invokeSomething("invokefunction", p, signers)
}

// InvokeFunctionAtHeight returns the results after calling the smart contract
// with the given operation and parameters at the given blockchain state
// specified by the blockchain height.
// NOTE: this is test invoke and will not affect the blockchain.
func (c *Client) InvokeFunctionAtHeight(height uint32, contract util.Uint160, operation string, params []smartcontract.Parameter, signers []transaction.Signer) (*result.Invoke, error) {
	var p = []any{height, contract.StringLE(), operation, params}
	return c.invokeSomething("invokefunctionhistoric", p, signers)
}

// InvokeFunctionWithState returns the results after calling the smart contract
// with the given operation and parameters at the given blockchain state defined
// by the specified state root or block hash.
// NOTE: this is test invoke and will not affect the blockchain.
func (c *Client) InvokeFunctionWithState(stateOrBlock util.Uint256, contract util.Uint160, operation string, params []smartcontract.Parameter, signers []transaction.Signer) (*result.Invoke, error) {
	var p = []any{stateOrBlock.StringLE(), contract.StringLE(), operation, params}
	return c.invokeSomething("invokefunctionhistoric", p, signers)
}

// InvokeContractVerify returns the results after calling `verify` method of the smart contract
// with the given parameters under verification trigger type.
// NOTE: this is test invoke and will not affect the blockchain.
func (c *Client) InvokeContractVerify(contract util.Uint160, params []smartcontract.Parameter, signers []transaction.Signer, witnesses ...transaction.Witness) (*result.Invoke, error) {
	var p = []any{contract.StringLE(), params}
	return c.invokeSomething("invokecontractverify", p, signers, witnesses...)
}

// InvokeContractVerifyAtHeight returns the results after calling `verify` method
// of the smart contract with the given parameters under verification trigger type
// at the blockchain state specified by the blockchain height.
// NOTE: this is test invoke and will not affect the blockchain.
func (c *Client) InvokeContractVerifyAtHeight(height uint32, contract util.Uint160, params []smartcontract.Parameter, signers []transaction.Signer, witnesses ...transaction.Witness) (*result.Invoke, error) {
	var p = []any{height, contract.StringLE(), params}
	return c.invokeSomething("invokecontractverifyhistoric", p, signers, witnesses...)
}

// InvokeContractVerifyWithState returns the results after calling `verify` method
// of the smart contract with the given parameters under verification trigger type
// at the blockchain state specified by the state root or block hash.
// NOTE: this is test invoke and will not affect the blockchain.
func (c *Client) InvokeContractVerifyWithState(stateOrBlock util.Uint256, contract util.Uint160, params []smartcontract.Parameter, signers []transaction.Signer, witnesses ...transaction.Witness) (*result.Invoke, error) {
	var p = []any{stateOrBlock.StringLE(), contract.StringLE(), params}
	return c.invokeSomething("invokecontractverifyhistoric", p, signers, witnesses...)
}

// invokeSomething is an inner wrapper for Invoke* functions.
func (c *Client) invokeSomething(method string, p []any, signers []transaction.Signer, witnesses ...transaction.Witness) (*result.Invoke, error) {
	var resp = new(result.Invoke)
	if signers != nil {
		if witnesses == nil {
			p = append(p, signers)
		} else {
			if len(witnesses) != len(signers) {
				return nil, fmt.Errorf("number of witnesses should match number of signers, got %d vs %d", len(witnesses), len(signers))
			}
			signersWithWitnesses := make([]neorpc.SignerWithWitness, len(signers))
			for i := range signersWithWitnesses {
				signersWithWitnesses[i] = neorpc.SignerWithWitness{
					Signer:  signers[i],
					Witness: witnesses[i],
				}
			}
			p = append(p, signersWithWitnesses)
		}
	}
	if err := c.performRequest(method, p, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// SendRawTransaction broadcasts the given transaction to the Neo network.
// It always returns transaction hash, when successful (no error) this is the
// hash returned from server, when not it's a locally calculated rawTX hash.
func (c *Client) SendRawTransaction(rawTX *transaction.Transaction) (util.Uint256, error) {
	var (
		params = []any{rawTX.Bytes()}
		resp   = new(result.RelayResult)
	)
	if err := c.performRequest("sendrawtransaction", params, resp); err != nil {
		return rawTX.Hash(), err
	}
	return resp.Hash, nil
}

// SubmitBlock broadcasts a raw block over the NEO network.
func (c *Client) SubmitBlock(b block.Block) (util.Uint256, error) {
	var (
		params []any
		resp   = new(result.RelayResult)
	)
	buf := io.NewBufBinWriter()
	b.EncodeBinary(buf.BinWriter)
	if err := buf.Err; err != nil {
		return util.Uint256{}, err
	}
	params = []any{buf.Bytes()}

	if err := c.performRequest("submitblock", params, resp); err != nil {
		return util.Uint256{}, err
	}
	return resp.Hash, nil
}

// SubmitRawOracleResponse submits a raw oracle response to the oracle node.
// Raw params are used to avoid excessive marshalling.
func (c *Client) SubmitRawOracleResponse(ps []any) error {
	return c.performRequest("submitoracleresponse", ps, new(result.RelayResult))
}

// SubmitP2PNotaryRequest submits given P2PNotaryRequest payload to the RPC node.
func (c *Client) SubmitP2PNotaryRequest(req *payload.P2PNotaryRequest) (util.Uint256, error) {
	var resp = new(result.RelayResult)
	bytes, err := req.Bytes()
	if err != nil {
		return util.Uint256{}, fmt.Errorf("failed to encode request: %w", err)
	}
	params := []any{bytes}
	if err := c.performRequest("submitnotaryrequest", params, resp); err != nil {
		return util.Uint256{}, err
	}
	return resp.Hash, nil
}

// ValidateAddress verifies that the address is a correct NEO address.
// Consider using [address] package instead to do it locally.
func (c *Client) ValidateAddress(address string) error {
	var (
		params = []any{address}
		resp   = &result.ValidateAddress{}
	)

	if err := c.performRequest("validateaddress", params, resp); err != nil {
		return err
	}
	if !resp.IsValid {
		return errors.New("validateaddress returned false")
	}
	return nil
}

// stateRootInHeader returns true if the state root is contained in the block header.
// Requires Init() before use.
func (c *Client) stateRootInHeader() (bool, error) {
	c.cacheLock.RLock()
	defer c.cacheLock.RUnlock()

	if !c.cache.initDone {
		return false, errNetworkNotInitialized
	}
	return c.cache.stateRootInHeader, nil
}

// TraverseIterator returns a set of iterator values (maxItemsCount at max) for
// the specified iterator and session. If result contains no elements, then either
// Iterator has no elements or session was expired and terminated by the server.
// If maxItemsCount is non-positive, then config.DefaultMaxIteratorResultItems
// iterator values will be returned using single `traverseiterator` call.
// Note that iterator session lifetime is restricted by the RPC-server
// configuration and is being reset each time iterator is accessed. If session
// won't be accessed within session expiration time, then it will be terminated
// by the RPC-server automatically.
func (c *Client) TraverseIterator(sessionID, iteratorID uuid.UUID, maxItemsCount int) ([]stackitem.Item, error) {
	if maxItemsCount <= 0 {
		maxItemsCount = config.DefaultMaxIteratorResultItems
	}
	var (
		params = []any{sessionID.String(), iteratorID.String(), maxItemsCount}
		resp   []json.RawMessage
	)
	if err := c.performRequest("traverseiterator", params, &resp); err != nil {
		return nil, err
	}
	result := make([]stackitem.Item, len(resp))
	for i, iBytes := range resp {
		itm, err := stackitem.FromJSONWithTypes(iBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal %d-th iterator value: %w", i, err)
		}
		result[i] = itm
	}

	return result, nil
}

// TerminateSession tries to terminate the specified session and returns `true` iff
// the specified session was found on server.
func (c *Client) TerminateSession(sessionID uuid.UUID) (bool, error) {
	var resp bool
	params := []any{sessionID.String()}
	if err := c.performRequest("terminatesession", params, &resp); err != nil {
		return false, err
	}

	return resp, nil
}

// GetRawNotaryTransaction  returns main or fallback transaction from the
// RPC node's notary request pool.
func (c *Client) GetRawNotaryTransaction(hash util.Uint256) (*transaction.Transaction, error) {
	var (
		params = []any{hash.StringLE()}
		resp   []byte
		err    error
	)
	if err = c.performRequest("getrawnotarytransaction", params, &resp); err != nil {
		return nil, err
	}
	return transaction.NewTransactionFromBytes(resp)
}

// GetRawNotaryTransactionVerbose returns main or fallback transaction from the
// RPC node's notary request pool.
// NOTE: to get transaction.ID and transaction.Size, use t.Hash() and
// io.GetVarSize(t) respectively.
func (c *Client) GetRawNotaryTransactionVerbose(hash util.Uint256) (*transaction.Transaction, error) {
	var (
		params = []any{hash.StringLE(), 1} // 1 for verbose.
		resp   = &transaction.Transaction{}
		err    error
	)
	if err = c.performRequest("getrawnotarytransaction", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetRawNotaryPool returns hashes of main P2PNotaryRequest transactions that
// are currently in the RPC node's notary request pool with the corresponding
// hashes of fallback transactions.
func (c *Client) GetRawNotaryPool() (*result.RawNotaryPool, error) {
	resp := &result.RawNotaryPool{}
	if err := c.performRequest("getrawnotarypool", nil, resp); err != nil {
		return nil, err
	}
	return resp, nil
}
