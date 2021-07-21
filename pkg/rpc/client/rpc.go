package client

import (
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativeprices"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

var errNetworkNotInitialized = errors.New("RPC client network is not initialized")

// CalculateNetworkFee calculates network fee for transaction. The transaction may
// have empty witnesses for contract signers and may have only verification scripts
// filled for standard sig/multisig signers.
func (c *Client) CalculateNetworkFee(tx *transaction.Transaction) (int64, error) {
	var (
		params = request.NewRawParams(tx.Bytes())
		resp   = new(result.NetworkFee)
	)
	if err := c.performRequest("calculatenetworkfee", params, resp); err != nil {
		return 0, err
	}
	return resp.Value, nil
}

// GetApplicationLog returns the contract log based on the specified txid.
func (c *Client) GetApplicationLog(hash util.Uint256, trig *trigger.Type) (*result.ApplicationLog, error) {
	var (
		params = request.NewRawParams(hash.StringLE())
		resp   = new(result.ApplicationLog)
	)
	if trig != nil {
		params.Values = append(params.Values, trig.String())
	}
	if err := c.performRequest("getapplicationlog", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetBestBlockHash returns the hash of the tallest block in the main chain.
func (c *Client) GetBestBlockHash() (util.Uint256, error) {
	var resp = util.Uint256{}
	if err := c.performRequest("getbestblockhash", request.NewRawParams(), &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetBlockCount returns the number of blocks in the main chain.
func (c *Client) GetBlockCount() (uint32, error) {
	var resp uint32
	if err := c.performRequest("getblockcount", request.NewRawParams(), &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetBlockByIndex returns a block by its height. You should initialize network magic
// with Init before calling GetBlockByIndex.
func (c *Client) GetBlockByIndex(index uint32) (*block.Block, error) {
	return c.getBlock(request.NewRawParams(index))
}

// GetBlockByHash returns a block by its hash. You should initialize network magic
// with Init before calling GetBlockByHash.
func (c *Client) GetBlockByHash(hash util.Uint256) (*block.Block, error) {
	return c.getBlock(request.NewRawParams(hash.StringLE()))
}

func (c *Client) getBlock(params request.RawParams) (*block.Block, error) {
	var (
		resp []byte
		err  error
		b    *block.Block
	)
	if !c.initDone {
		return nil, errNetworkNotInitialized
	}
	if err = c.performRequest("getblock", params, &resp); err != nil {
		return nil, err
	}
	r := io.NewBinReaderFromBuf(resp)
	b = block.New(c.StateRootInHeader())
	b.DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}
	return b, nil
}

// GetBlockByIndexVerbose returns a block wrapper with additional metadata by
// its height. You should initialize network magic with Init before calling GetBlockByIndexVerbose.
// NOTE: to get transaction.ID and transaction.Size, use t.Hash() and io.GetVarSize(t) respectively.
func (c *Client) GetBlockByIndexVerbose(index uint32) (*result.Block, error) {
	return c.getBlockVerbose(request.NewRawParams(index, 1))
}

// GetBlockByHashVerbose returns a block wrapper with additional metadata by
// its hash. You should initialize network magic with Init before calling GetBlockByHashVerbose.
func (c *Client) GetBlockByHashVerbose(hash util.Uint256) (*result.Block, error) {
	return c.getBlockVerbose(request.NewRawParams(hash.StringLE(), 1))
}

func (c *Client) getBlockVerbose(params request.RawParams) (*result.Block, error) {
	var (
		resp = &result.Block{}
		err  error
	)
	if !c.initDone {
		return nil, errNetworkNotInitialized
	}
	if err = c.performRequest("getblock", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetBlockHash returns the hash value of the corresponding block, based on the specified index.
func (c *Client) GetBlockHash(index uint32) (util.Uint256, error) {
	var (
		params = request.NewRawParams(index)
		resp   = util.Uint256{}
	)
	if err := c.performRequest("getblockhash", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetBlockHeader returns the corresponding block header information from serialized hex string
// according to the specified script hash. You should initialize network magic
// // with Init before calling GetBlockHeader.
func (c *Client) GetBlockHeader(hash util.Uint256) (*block.Header, error) {
	var (
		params = request.NewRawParams(hash.StringLE())
		resp   []byte
		h      *block.Header
	)
	if !c.initDone {
		return nil, errNetworkNotInitialized
	}
	if err := c.performRequest("getblockheader", params, &resp); err != nil {
		return nil, err
	}
	r := io.NewBinReaderFromBuf(resp)
	h = new(block.Header)
	h.DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}
	return h, nil
}

// GetBlockHeaderCount returns the number of headers in the main chain.
func (c *Client) GetBlockHeaderCount() (uint32, error) {
	var resp uint32
	if err := c.performRequest("getblockheadercount", request.NewRawParams(), &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetBlockHeaderVerbose returns the corresponding block header information from Json format string
// according to the specified script hash.
func (c *Client) GetBlockHeaderVerbose(hash util.Uint256) (*result.Header, error) {
	var (
		params = request.NewRawParams(hash.StringLE(), 1)
		resp   = &result.Header{}
	)
	if err := c.performRequest("getblockheader", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetBlockSysFee returns the system fees of the block, based on the specified index.
func (c *Client) GetBlockSysFee(index uint32) (fixedn.Fixed8, error) {
	var (
		params = request.NewRawParams(index)
		resp   fixedn.Fixed8
	)
	if err := c.performRequest("getblocksysfee", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetConnectionCount returns the current number of connections for the node.
func (c *Client) GetConnectionCount() (int, error) {
	var (
		params = request.NewRawParams()
		resp   int
	)
	if err := c.performRequest("getconnectioncount", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetCommittee returns the current public keys of NEO nodes in committee.
func (c *Client) GetCommittee() (keys.PublicKeys, error) {
	var (
		params = request.NewRawParams()
		resp   = new(keys.PublicKeys)
	)
	if err := c.performRequest("getcommittee", params, resp); err != nil {
		return nil, err
	}
	return *resp, nil
}

// GetContractStateByHash queries contract information, according to the contract script hash.
func (c *Client) GetContractStateByHash(hash util.Uint160) (*state.Contract, error) {
	return c.getContractState(hash.StringLE())
}

// GetContractStateByAddressOrName queries contract information, according to the contract address or name.
func (c *Client) GetContractStateByAddressOrName(addressOrName string) (*state.Contract, error) {
	return c.getContractState(addressOrName)
}

// GetContractStateByID queries contract information, according to the contract ID.
func (c *Client) GetContractStateByID(id int32) (*state.Contract, error) {
	return c.getContractState(id)
}

// getContractState is an internal representation of GetContractStateBy* methods.
func (c *Client) getContractState(param interface{}) (*state.Contract, error) {
	var (
		params = request.NewRawParams(param)
		resp   = &state.Contract{}
	)
	if err := c.performRequest("getcontractstate", params, resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetNativeContracts queries information about native contracts.
func (c *Client) GetNativeContracts() ([]state.NativeContract, error) {
	var (
		params = request.NewRawParams()
		resp   []state.NativeContract
	)
	if err := c.performRequest("getnativecontracts", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetNEP17Balances is a wrapper for getnep17balances RPC.
func (c *Client) GetNEP17Balances(address util.Uint160) (*result.NEP17Balances, error) {
	params := request.NewRawParams(address.StringLE())
	resp := new(result.NEP17Balances)
	if err := c.performRequest("getnep17balances", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetNEP17Transfers is a wrapper for getnep17transfers RPC. Address parameter
// is mandatory, while all the others are optional. Start and stop parameters
// are supported since neo-go 0.77.0 and limit and page since neo-go 0.78.0.
// These parameters are positional in the JSON-RPC call, you can't specify limit
// and not specify start/stop for example.
func (c *Client) GetNEP17Transfers(address string, start, stop *uint32, limit, page *int) (*result.NEP17Transfers, error) {
	params := request.NewRawParams(address)
	if start != nil {
		params.Values = append(params.Values, *start)
		if stop != nil {
			params.Values = append(params.Values, *stop)
			if limit != nil {
				params.Values = append(params.Values, *limit)
				if page != nil {
					params.Values = append(params.Values, *page)
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
	resp := new(result.NEP17Transfers)
	if err := c.performRequest("getnep17transfers", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetPeers returns the list of nodes that the node is currently connected/disconnected from.
func (c *Client) GetPeers() (*result.GetPeers, error) {
	var (
		params = request.NewRawParams()
		resp   = &result.GetPeers{}
	)
	if err := c.performRequest("getpeers", params, resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetRawMemPool returns the list of unconfirmed transactions in memory.
func (c *Client) GetRawMemPool() ([]util.Uint256, error) {
	var (
		params = request.NewRawParams()
		resp   = new([]util.Uint256)
	)
	if err := c.performRequest("getrawmempool", params, resp); err != nil {
		return *resp, err
	}
	return *resp, nil
}

// GetRawTransaction returns a transaction by hash. You should initialize network magic
// with Init before calling GetRawTransaction.
func (c *Client) GetRawTransaction(hash util.Uint256) (*transaction.Transaction, error) {
	var (
		params = request.NewRawParams(hash.StringLE())
		resp   []byte
		err    error
	)
	if !c.initDone {
		return nil, errNetworkNotInitialized
	}
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
// metadata by transaction's hash. You should initialize network magic
// with Init before calling GetRawTransactionVerbose.
// NOTE: to get transaction.ID and transaction.Size, use t.Hash() and io.GetVarSize(t) respectively.
func (c *Client) GetRawTransactionVerbose(hash util.Uint256) (*result.TransactionOutputRaw, error) {
	var (
		params = request.NewRawParams(hash.StringLE(), 1)
		resp   = &result.TransactionOutputRaw{}
		err    error
	)
	if !c.initDone {
		return nil, errNetworkNotInitialized
	}
	if err = c.performRequest("getrawtransaction", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetStorageByID returns the stored value, according to the contract ID and the stored key.
func (c *Client) GetStorageByID(id int32, key []byte) ([]byte, error) {
	return c.getStorage(request.NewRawParams(id, base64.StdEncoding.EncodeToString(key)))
}

// GetStorageByHash returns the stored value, according to the contract script hash and the stored key.
func (c *Client) GetStorageByHash(hash util.Uint160, key []byte) ([]byte, error) {
	return c.getStorage(request.NewRawParams(hash.StringLE(), base64.StdEncoding.EncodeToString(key)))
}

func (c *Client) getStorage(params request.RawParams) ([]byte, error) {
	var resp []byte
	if err := c.performRequest("getstorage", params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetTransactionHeight returns the block index in which the transaction is found.
func (c *Client) GetTransactionHeight(hash util.Uint256) (uint32, error) {
	var (
		params = request.NewRawParams(hash.StringLE())
		resp   uint32
	)
	if err := c.performRequest("gettransactionheight", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetUnclaimedGas returns unclaimed GAS amount for the specified address.
func (c *Client) GetUnclaimedGas(address string) (result.UnclaimedGas, error) {
	var (
		params = request.NewRawParams(address)
		resp   result.UnclaimedGas
	)
	if err := c.performRequest("getunclaimedgas", params, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetNextBlockValidators returns the current NEO consensus nodes information and voting status.
func (c *Client) GetNextBlockValidators() ([]result.Validator, error) {
	var (
		params = request.NewRawParams()
		resp   = new([]result.Validator)
	)
	if err := c.performRequest("getnextblockvalidators", params, resp); err != nil {
		return nil, err
	}
	return *resp, nil
}

// GetVersion returns the version information about the queried node.
func (c *Client) GetVersion() (*result.Version, error) {
	var (
		params = request.NewRawParams()
		resp   = &result.Version{}
	)
	if err := c.performRequest("getversion", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// InvokeScript returns the result of the given script after running it true the VM.
// NOTE: This is a test invoke and will not affect the blockchain.
func (c *Client) InvokeScript(script []byte, signers []transaction.Signer) (*result.Invoke, error) {
	var p = request.NewRawParams(script)
	return c.invokeSomething("invokescript", p, signers)
}

// InvokeFunction returns the results after calling the smart contract scripthash
// with the given operation and parameters.
// NOTE: this is test invoke and will not affect the blockchain.
func (c *Client) InvokeFunction(contract util.Uint160, operation string, params []smartcontract.Parameter, signers []transaction.Signer) (*result.Invoke, error) {
	var p = request.NewRawParams(contract.StringLE(), operation, params)
	return c.invokeSomething("invokefunction", p, signers)
}

// InvokeContractVerify returns the results after calling `verify` method of the smart contract
// with the given parameters under verification trigger type.
// NOTE: this is test invoke and will not affect the blockchain.
func (c *Client) InvokeContractVerify(contract util.Uint160, params []smartcontract.Parameter, signers []transaction.Signer, witnesses ...transaction.Witness) (*result.Invoke, error) {
	var p = request.NewRawParams(contract.StringLE(), params)
	return c.invokeSomething("invokecontractverify", p, signers, witnesses...)
}

// invokeSomething is an inner wrapper for Invoke* functions.
func (c *Client) invokeSomething(method string, p request.RawParams, signers []transaction.Signer, witnesses ...transaction.Witness) (*result.Invoke, error) {
	var resp = new(result.Invoke)
	if signers != nil {
		if witnesses == nil {
			p.Values = append(p.Values, signers)
		} else {
			if len(witnesses) != len(signers) {
				return nil, fmt.Errorf("number of witnesses should match number of signers, got %d vs %d", len(witnesses), len(signers))
			}
			signersWithWitnesses := make([]request.SignerWithWitness, len(signers))
			for i := range signersWithWitnesses {
				signersWithWitnesses[i] = request.SignerWithWitness{
					Signer:  signers[i],
					Witness: witnesses[i],
				}
			}
			p.Values = append(p.Values, signersWithWitnesses)
		}
	}
	if err := c.performRequest(method, p, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// SendRawTransaction broadcasts a transaction over the NEO network.
// The given hex string needs to be signed with a keypair.
// When the result of the response object is true, the TX has successfully
// been broadcasted to the network.
func (c *Client) SendRawTransaction(rawTX *transaction.Transaction) (util.Uint256, error) {
	var (
		params = request.NewRawParams(rawTX.Bytes())
		resp   = new(result.RelayResult)
	)
	if err := c.performRequest("sendrawtransaction", params, resp); err != nil {
		return util.Uint256{}, err
	}
	return resp.Hash, nil
}

// SubmitBlock broadcasts a raw block over the NEO network.
func (c *Client) SubmitBlock(b block.Block) (util.Uint256, error) {
	var (
		params request.RawParams
		resp   = new(result.RelayResult)
	)
	buf := io.NewBufBinWriter()
	b.EncodeBinary(buf.BinWriter)
	if err := buf.Err; err != nil {
		return util.Uint256{}, err
	}
	params = request.NewRawParams(buf.Bytes())

	if err := c.performRequest("submitblock", params, resp); err != nil {
		return util.Uint256{}, err
	}
	return resp.Hash, nil
}

// SubmitRawOracleResponse submits raw oracle response to the oracle node.
// Raw params are used to avoid excessive marshalling.
func (c *Client) SubmitRawOracleResponse(ps request.RawParams) error {
	return c.performRequest("submitoracleresponse", ps, new(result.RelayResult))
}

// SignAndPushInvocationTx signs and pushes given script as an invocation
// transaction using given wif to sign it and given cosigners to cosign it if
// possible. It spends the amount of gas specified. It returns a hash of the
// invocation transaction and an error. If one of the cosigners accounts is
// neither contract-based nor unlocked an error is returned.
func (c *Client) SignAndPushInvocationTx(script []byte, acc *wallet.Account, sysfee int64, netfee fixedn.Fixed8, cosigners []SignerAccount) (util.Uint256, error) {
	tx, err := c.CreateTxFromScript(script, acc, sysfee, int64(netfee), cosigners)
	if err != nil {
		return util.Uint256{}, fmt.Errorf("failed to create tx: %w", err)
	}
	return c.SignAndPushTx(tx, acc, cosigners)
}

// SignAndPushTx signs given transaction using given wif and cosigners and pushes
// it to the chain. It returns a hash of the transaction and an error. If one of
// the cosigners accounts is neither contract-based nor unlocked an error is
// returned.
func (c *Client) SignAndPushTx(tx *transaction.Transaction, acc *wallet.Account, cosigners []SignerAccount) (util.Uint256, error) {
	var (
		txHash util.Uint256
		err    error
	)
	if err = acc.SignTx(c.GetNetwork(), tx); err != nil {
		return txHash, fmt.Errorf("failed to sign tx: %w", err)
	}
	// try to add witnesses for the rest of the signers
	for i, signer := range tx.Signers[1:] {
		var isOk bool
		for _, cosigner := range cosigners {
			if signer.Account == cosigner.Signer.Account {
				err = cosigner.Account.SignTx(c.GetNetwork(), tx)
				if err != nil { // then account is non-contract-based and locked, but let's provide more detailed error
					if paramNum := len(cosigner.Account.Contract.Parameters); paramNum != 0 && cosigner.Account.Contract.Deployed {
						return txHash, fmt.Errorf("failed to add contract-based witness for signer #%d (%s): "+
							"%d parameters must be provided to construct invocation script", i, address.Uint160ToString(signer.Account), paramNum)
					}
					return txHash, fmt.Errorf("failed to add witness for signer #%d (%s): account should be unlocked to add the signature. "+
						"Store partially-signed transaction and then use 'wallet sign' command to cosign it", i, address.Uint160ToString(signer.Account))
				}
				isOk = true
				break
			}
		}
		if !isOk {
			return txHash, fmt.Errorf("failed to add witness for signer #%d (%s): account wasn't provided", i, address.Uint160ToString(signer.Account))
		}
	}
	txHash = tx.Hash()
	actualHash, err := c.SendRawTransaction(tx)
	if err != nil {
		return txHash, fmt.Errorf("failed to send tx: %w", err)
	}
	if !actualHash.Equals(txHash) {
		return actualHash, fmt.Errorf("sent and actual tx hashes mismatch:\n\tsent: %v\n\tactual: %v", txHash.StringLE(), actualHash.StringLE())
	}
	return txHash, nil
}

// getSigners returns an array of transaction signers and corresponding accounts from
// given sender and cosigners. If cosigners list already contains sender, the sender
// will be placed at the start of the list.
func getSigners(sender *wallet.Account, cosigners []SignerAccount) ([]transaction.Signer, []*wallet.Account, error) {
	var (
		signers  []transaction.Signer
		accounts []*wallet.Account
	)
	from, err := address.StringToUint160(sender.Address)
	if err != nil {
		return nil, nil, fmt.Errorf("bad sender account address: %v", err)
	}
	s := transaction.Signer{
		Account: from,
		Scopes:  transaction.None,
	}
	for _, c := range cosigners {
		if c.Signer.Account == from {
			s.Scopes = c.Signer.Scopes
			continue
		}
		signers = append(signers, c.Signer)
		accounts = append(accounts, c.Account)
	}
	signers = append([]transaction.Signer{s}, signers...)
	accounts = append([]*wallet.Account{sender}, accounts...)
	return signers, accounts, nil
}

// SignAndPushP2PNotaryRequest creates and pushes P2PNotary request constructed from the main
// and fallback transactions using given wif to sign it. It returns the request and an error.
// Fallback transaction is constructed from the given script using the amount of gas specified.
// For successful fallback transaction validation at least 2*transaction.NotaryServiceFeePerKey
// GAS should be deposited to Notary contract.
// Main transaction should be constructed by the user. Several rules need to be met for
// successful main transaction acceptance:
// 1. Native Notary contract should be a signer of the main transaction.
// 2. Notary signer should have None scope.
// 3. Main transaction should have dummy contract witness for Notary signer.
// 4. Main transaction should have NotaryAssisted attribute with NKeys specified.
// 5. NotaryAssisted attribute and dummy Notary witness (as long as the other incomplete witnesses)
//    should be paid for. Use CalculateNotaryWitness to calculate the amount of network fee to pay
//    for the attribute and Notary witness.
// 6. Main transaction either shouldn't have all witnesses attached (in this case none of them
//	  can be multisignature), or it only should have a partial multisignature.
// Note: client should be initialized before SignAndPushP2PNotaryRequest call.
func (c *Client) SignAndPushP2PNotaryRequest(mainTx *transaction.Transaction, fallbackScript []byte, fallbackSysFee int64, fallbackNetFee int64, fallbackValidFor uint32, acc *wallet.Account) (*payload.P2PNotaryRequest, error) {
	var err error
	if !c.initDone {
		return nil, errNetworkNotInitialized
	}
	notaryHash, err := c.GetNativeContractHash(nativenames.Notary)
	if err != nil {
		return nil, fmt.Errorf("failed to get native Notary hash: %w", err)
	}
	from, err := address.StringToUint160(acc.Address)
	if err != nil {
		return nil, fmt.Errorf("bad account address: %v", err)
	}
	signers := []transaction.Signer{{Account: notaryHash}, {Account: from}}
	if fallbackSysFee < 0 {
		result, err := c.InvokeScript(fallbackScript, signers)
		if err != nil {
			return nil, fmt.Errorf("can't add system fee to fallback transaction: %w", err)
		}
		if result.State != "HALT" {
			return nil, fmt.Errorf("can't add system fee to fallback transaction: bad vm state %s due to an error: %s", result.State, result.FaultException)
		}
		fallbackSysFee = result.GasConsumed
	}

	maxNVBDelta, err := c.GetMaxNotValidBeforeDelta()
	if err != nil {
		return nil, fmt.Errorf("failed to get MaxNotValidBeforeDelta")
	}
	if int64(fallbackValidFor) > maxNVBDelta {
		return nil, fmt.Errorf("fallback transaction should be valid for not more than %d blocks", maxNVBDelta)
	}
	fallbackTx := transaction.New(fallbackScript, fallbackSysFee)
	fallbackTx.Signers = signers
	fallbackTx.ValidUntilBlock = mainTx.ValidUntilBlock
	fallbackTx.Attributes = []transaction.Attribute{
		{
			Type:  transaction.NotaryAssistedT,
			Value: &transaction.NotaryAssisted{NKeys: 0},
		},
		{
			Type:  transaction.NotValidBeforeT,
			Value: &transaction.NotValidBefore{Height: fallbackTx.ValidUntilBlock - fallbackValidFor + 1},
		},
		{
			Type:  transaction.ConflictsT,
			Value: &transaction.Conflicts{Hash: mainTx.Hash()},
		},
	}
	extraNetFee, err := c.CalculateNotaryFee(0)
	if err != nil {
		return nil, err
	}
	fallbackNetFee += extraNetFee

	dummyAccount := &wallet.Account{Contract: &wallet.Contract{Deployed: false}} // don't call `verify` for Notary contract witness, because it will fail
	err = c.AddNetworkFee(fallbackTx, fallbackNetFee, dummyAccount, acc)
	if err != nil {
		return nil, fmt.Errorf("failed to add network fee: %w", err)
	}
	fallbackTx.Scripts = []transaction.Witness{
		{
			InvocationScript:   append([]byte{byte(opcode.PUSHDATA1), 64}, make([]byte, 64)...),
			VerificationScript: []byte{},
		},
	}
	if err = acc.SignTx(c.GetNetwork(), fallbackTx); err != nil {
		return nil, fmt.Errorf("failed to sign fallback tx: %w", err)
	}
	fallbackHash := fallbackTx.Hash()
	req := &payload.P2PNotaryRequest{
		MainTransaction:     mainTx,
		FallbackTransaction: fallbackTx,
	}
	req.Witness = transaction.Witness{
		InvocationScript:   append([]byte{byte(opcode.PUSHDATA1), 64}, acc.PrivateKey().SignHashable(uint32(c.GetNetwork()), req)...),
		VerificationScript: acc.GetVerificationScript(),
	}
	actualHash, err := c.SubmitP2PNotaryRequest(req)
	if err != nil {
		return req, fmt.Errorf("failed to submit notary request: %w", err)
	}
	if !actualHash.Equals(fallbackHash) {
		return req, fmt.Errorf("sent and actual fallback tx hashes mismatch:\n\tsent: %v\n\tactual: %v", fallbackHash.StringLE(), actualHash.StringLE())
	}
	return req, nil
}

// CalculateNotaryFee calculates network fee for one dummy Notary witness and NotaryAssisted attribute with NKeys specified.
// The result should be added to the transaction's net fee for successful verification.
func (c *Client) CalculateNotaryFee(nKeys uint8) (int64, error) {
	baseExecFee, err := c.GetExecFeeFactor()
	if err != nil {
		return 0, fmt.Errorf("failed to get BaseExecFeeFactor: %w", err)
	}
	feePerByte, err := c.GetFeePerByte()
	if err != nil {
		return 0, fmt.Errorf("failed to get FeePerByte: %w", err)
	}
	return int64((nKeys+1))*transaction.NotaryServiceFeePerKey + // fee for NotaryAssisted attribute
			fee.Opcode(baseExecFee, // Notary node witness
				opcode.PUSHDATA1, opcode.RET, // invocation script
				opcode.PUSH0, opcode.SYSCALL, opcode.RET) + // System.Contract.CallNative
			nativeprices.NotaryVerificationPrice*baseExecFee + // Notary witness verification price
			feePerByte*int64(io.GetVarSize(make([]byte, 66))) + // invocation script per-byte fee
			feePerByte*int64(io.GetVarSize([]byte{})), // verification script per-byte fee
		nil
}

// SubmitP2PNotaryRequest submits given P2PNotaryRequest payload to the RPC node.
func (c *Client) SubmitP2PNotaryRequest(req *payload.P2PNotaryRequest) (util.Uint256, error) {
	var resp = new(result.RelayResult)
	bytes, err := req.Bytes()
	if err != nil {
		return util.Uint256{}, fmt.Errorf("failed to encode request: %w", err)
	}
	params := request.NewRawParams(bytes)
	if err := c.performRequest("submitnotaryrequest", params, resp); err != nil {
		return util.Uint256{}, err
	}
	return resp.Hash, nil
}

// ValidateAddress verifies that the address is a correct NEO address.
func (c *Client) ValidateAddress(address string) error {
	var (
		params = request.NewRawParams(address)
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

// CalculateValidUntilBlock calculates ValidUntilBlock field for tx as
// current blockchain height + number of validators. Number of validators
// is the length of blockchain validators list got from GetNextBlockValidators()
// method. Validators count is being cached and updated every 100 blocks.
func (c *Client) CalculateValidUntilBlock() (uint32, error) {
	var (
		result          uint32
		validatorsCount uint32
	)
	blockCount, err := c.GetBlockCount()
	if err != nil {
		return result, fmt.Errorf("can't get block count: %w", err)
	}

	if c.cache.calculateValidUntilBlock.expiresAt > blockCount {
		validatorsCount = c.cache.calculateValidUntilBlock.validatorsCount
	} else {
		validators, err := c.GetNextBlockValidators()
		if err != nil {
			return result, fmt.Errorf("can't get validators: %w", err)
		}
		validatorsCount = uint32(len(validators))
		c.cache.calculateValidUntilBlock = calculateValidUntilBlockCache{
			validatorsCount: validatorsCount,
			expiresAt:       blockCount + cacheTimeout,
		}
	}
	return blockCount + validatorsCount + 1, nil
}

// AddNetworkFee adds network fee for each witness script and optional extra
// network fee to transaction. `accs` is an array of signer's accounts with
// matching order.
func (c *Client) AddNetworkFee(tx *transaction.Transaction, extraFee int64, accs ...*wallet.Account) error {
	if len(tx.Signers) != len(accs) {
		return errors.New("number of signers must match number of scripts")
	}
	oldScripts := tx.Scripts
	tx.Scripts = make([]transaction.Witness, len(tx.Signers))
	for i := range tx.Signers {
		if accs[i].Contract != nil {
			if accs[i].Contract.Deployed == true {
				tx.Scripts[i] = transaction.Witness{InvocationScript: []byte{}, VerificationScript: []byte{}}
				continue
			}
			nativeNotaryHashableScript := state.CreateContractHashableScript(util.Uint160{}, 0, nativenames.Notary)
			if accs[i].Contract.Script == nil && tx.Signers[i].Account == hash.Hash160(nativeNotaryHashableScript) {
				// This is a hack for Notary contract witness in uncompleted Notary request transactions.
				// Witness check for such uncompleted transactions will fail, but we need the tx to pass
				// witness check for the rest of witnesses. We're OK with 0 netfee for Notary witness,
				// so all we need is just to skip Notary witness check. Given `CalculateNetworkFee`, the
				// most simple way to do it is to pretend that Notary witness is an unusual witness
				// (neither of sig/multisig/contract-based) with hash(verificationScript) == Notary contract hash.
				tx.Scripts[i] = transaction.Witness{InvocationScript: []byte{}, VerificationScript: nativeNotaryHashableScript}
				continue
			}
		}
		tx.Scripts[i] = transaction.Witness{InvocationScript: []byte{}, VerificationScript: accs[i].GetVerificationScript()}
	}
	netFee, err := c.CalculateNetworkFee(tx)
	if err != nil {
		return fmt.Errorf("`calculatenetworkfee` RPC request returned an error: %w", err)
	}
	tx.NetworkFee += netFee + extraFee
	tx.Scripts = oldScripts
	return nil
}

// GetNetwork returns the network magic of the RPC node client connected to.
func (c *Client) GetNetwork() netmode.Magic {
	return c.network
}

// StateRootInHeader returns true if state root is contained in block header.
func (c *Client) StateRootInHeader() bool {
	return c.stateRootInHeader
}

// GetNativeContractHash returns native contract hash by its name.
func (c *Client) GetNativeContractHash(name string) (util.Uint160, error) {
	hash, ok := c.cache.nativeHashes[name]
	if ok {
		return hash, nil
	}
	cs, err := c.GetContractStateByAddressOrName(name)
	if err != nil {
		return util.Uint160{}, err
	}
	c.cache.nativeHashes[name] = cs.Hash
	return cs.Hash, nil
}
