package client

import (
	"encoding/hex"
	"strconv"

	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/pkg/errors"
)

// GetApplicationLog returns the contract log based on the specified txid.
func (c *Client) GetApplicationLog(hash util.Uint256) (*result.ApplicationLog, error) {
	var (
		params = request.NewRawParams(hash.StringLE())
		resp   = &result.ApplicationLog{}
	)
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

// GetBlockByIndex returns a block by its height.
func (c *Client) GetBlockByIndex(index uint32) (*block.Block, error) {
	return c.getBlock(request.NewRawParams(index))
}

// GetBlockByHash returns a block by its hash.
func (c *Client) GetBlockByHash(hash util.Uint256) (*block.Block, error) {
	return c.getBlock(request.NewRawParams(hash.StringLE()))
}

func (c *Client) getBlock(params request.RawParams) (*block.Block, error) {
	var (
		resp string
		err  error
		b    *block.Block
	)
	if err = c.performRequest("getblock", params, &resp); err != nil {
		return nil, err
	}
	blockBytes, err := hex.DecodeString(resp)
	if err != nil {
		return nil, err
	}
	r := io.NewBinReaderFromBuf(blockBytes)
	b = new(block.Block)
	b.DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}
	return b, nil
}

// GetBlockByIndexVerbose returns a block wrapper with additional metadata by
// its height.
// NOTE: to get transaction.ID and transaction.Size, use t.Hash() and io.GetVarSize(t) respectively.
func (c *Client) GetBlockByIndexVerbose(index uint32) (*result.Block, error) {
	return c.getBlockVerbose(request.NewRawParams(index, 1))
}

// GetBlockByHashVerbose returns a block wrapper with additional metadata by
// its hash.
func (c *Client) GetBlockByHashVerbose(hash util.Uint256) (*result.Block, error) {
	return c.getBlockVerbose(request.NewRawParams(hash.StringLE(), 1))
}

func (c *Client) getBlockVerbose(params request.RawParams) (*result.Block, error) {
	var (
		resp = &result.Block{}
		err  error
	)
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
// according to the specified script hash.
func (c *Client) GetBlockHeader(hash util.Uint256) (*block.Header, error) {
	var (
		params = request.NewRawParams(hash.StringLE())
		resp   string
		h      *block.Header
	)
	if err := c.performRequest("getblockheader", params, &resp); err != nil {
		return nil, err
	}
	headerBytes, err := hex.DecodeString(resp)
	if err != nil {
		return nil, err
	}
	r := io.NewBinReaderFromBuf(headerBytes)
	h = new(block.Header)
	h.DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}
	return h, nil
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
func (c *Client) GetBlockSysFee(index uint32) (util.Fixed8, error) {
	var (
		params = request.NewRawParams(index)
		resp   util.Fixed8
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

// GetContractState queries contract information, according to the contract script hash.
func (c *Client) GetContractState(hash util.Uint160) (*result.ContractState, error) {
	var (
		params = request.NewRawParams(hash.StringLE())
		resp   = &result.ContractState{}
	)
	if err := c.performRequest("getcontractstate", params, resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// GetNEP5Balances is a wrapper for getnep5balances RPC.
func (c *Client) GetNEP5Balances(address util.Uint160) (*result.NEP5Balances, error) {
	params := request.NewRawParams(address.StringLE())
	resp := new(result.NEP5Balances)
	if err := c.performRequest("getnep5balances", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetNEP5Transfers is a wrapper for getnep5transfers RPC.
func (c *Client) GetNEP5Transfers(address string) (*result.NEP5Transfers, error) {
	params := request.NewRawParams(address)
	resp := new(result.NEP5Transfers)
	if err := c.performRequest("getnep5transfers", params, resp); err != nil {
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

// GetRawTransaction returns a transaction by hash.
func (c *Client) GetRawTransaction(hash util.Uint256) (*transaction.Transaction, error) {
	var (
		params = request.NewRawParams(hash.StringLE())
		resp   string
		err    error
	)
	if err = c.performRequest("getrawtransaction", params, &resp); err != nil {
		return nil, err
	}
	txBytes, err := hex.DecodeString(resp)
	if err != nil {
		return nil, err
	}
	tx, err := transaction.NewTransactionFromBytes(txBytes)
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
		params = request.NewRawParams(hash.StringLE(), 1)
		resp   = &result.TransactionOutputRaw{}
		err    error
	)
	if err = c.performRequest("getrawtransaction", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetStorage returns the stored value, according to the contract script hash and the stored key.
func (c *Client) GetStorage(hash util.Uint160, key []byte) ([]byte, error) {
	var (
		params = request.NewRawParams(hash.StringLE(), hex.EncodeToString(key))
		resp   string
	)
	if err := c.performRequest("getstorage", params, &resp); err != nil {
		return nil, err
	}
	res, err := hex.DecodeString(resp)
	if err != nil {
		return nil, err
	}
	return res, nil
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
func (c *Client) GetUnclaimedGas(address string) (util.Fixed8, error) {
	var (
		params = request.NewRawParams(address)
		resp   string
	)
	if err := c.performRequest("getunclaimedgas", params, &resp); err != nil {
		return 0, err
	}
	i, err := strconv.ParseInt(resp, 10, 64)
	if err != nil {
		return 0, err
	}
	return util.Fixed8(i), nil
}

// GetValidators returns the current NEO consensus nodes information and voting status.
func (c *Client) GetValidators() ([]result.Validator, error) {
	var (
		params = request.NewRawParams()
		resp   = new([]result.Validator)
	)
	if err := c.performRequest("getvalidators", params, resp); err != nil {
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
func (c *Client) InvokeScript(script string) (*result.Invoke, error) {
	var (
		params = request.NewRawParams(script)
		resp   = &result.Invoke{}
	)
	if err := c.performRequest("invokescript", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// InvokeFunction returns the results after calling the smart contract scripthash
// with the given operation and parameters.
// NOTE: this is test invoke and will not affect the blockchain.
func (c *Client) InvokeFunction(script, operation string, params []smartcontract.Parameter) (*result.Invoke, error) {
	var (
		p    = request.NewRawParams(script, operation, params)
		resp = &result.Invoke{}
	)
	if err := c.performRequest("invokefunction", p, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// SendRawTransaction broadcasts a transaction over the NEO network.
// The given hex string needs to be signed with a keypair.
// When the result of the response object is true, the TX has successfully
// been broadcasted to the network.
func (c *Client) SendRawTransaction(rawTX *transaction.Transaction) error {
	var (
		params = request.NewRawParams(hex.EncodeToString(rawTX.Bytes()))
		resp   bool
	)
	if err := c.performRequest("sendrawtransaction", params, &resp); err != nil {
		return err
	}
	if !resp {
		return errors.New("sendrawtransaction returned false")
	}
	return nil
}

// SubmitBlock broadcasts a raw block over the NEO network.
func (c *Client) SubmitBlock(b block.Block) error {
	var (
		params request.RawParams
		resp   bool
	)
	buf := io.NewBufBinWriter()
	b.EncodeBinary(buf.BinWriter)
	if err := buf.Err; err != nil {
		return err
	}
	params = request.NewRawParams(hex.EncodeToString(buf.Bytes()))

	if err := c.performRequest("submitblock", params, &resp); err != nil {
		return err
	}
	if !resp {
		return errors.New("submitblock returned false")
	}
	return nil
}

// SignAndPushInvocationTx signs and pushes given script as an invocation
// transaction  using given wif to sign it and spending the amount of gas
// specified. It returns a hash of the invocation transaction and an error.
func (c *Client) SignAndPushInvocationTx(script []byte, acc *wallet.Account, sysfee util.Fixed8, netfee util.Fixed8) (util.Uint256, error) {
	var txHash util.Uint256
	var err error

	tx := transaction.New(script, sysfee)
	tx.SystemFee = sysfee

	validUntilBlock, err := c.CalculateValidUntilBlock()
	if err != nil {
		return txHash, errors.Wrap(err, "failed to add validUntilBlock to transaction")
	}
	tx.ValidUntilBlock = validUntilBlock

	addr, err := address.StringToUint160(acc.Address)
	if err != nil {
		return txHash, errors.Wrap(err, "failed to get address")
	}
	tx.Sender = addr

	err = c.AddNetworkFee(tx, acc)
	if err != nil {
		return txHash, errors.Wrapf(err, "failed to add network fee")
	}

	if err = acc.SignTx(tx); err != nil {
		return txHash, errors.Wrap(err, "failed to sign tx")
	}
	txHash = tx.Hash()
	err = c.SendRawTransaction(tx)

	if err != nil {
		return txHash, errors.Wrap(err, "failed sendning tx")
	}
	return txHash, nil
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
// is the length of blockchain validators list got from GetValidators()
// method. Validators count is being cached and updated every 100 blocks.
func (c *Client) CalculateValidUntilBlock() (uint32, error) {
	var (
		result          uint32
		validatorsCount uint32
	)
	blockCount, err := c.GetBlockCount()
	if err != nil {
		return result, errors.Wrapf(err, "cannot get block count")
	}

	if c.cache.calculateValidUntilBlock.expiresAt > blockCount {
		validatorsCount = c.cache.calculateValidUntilBlock.validatorsCount
	} else {
		validators, err := c.GetValidators()
		if err != nil {
			return result, errors.Wrapf(err, "cannot get validators")
		}
		validatorsCount = uint32(len(validators))
		c.cache.calculateValidUntilBlock = calculateValidUntilBlockCache{
			validatorsCount: validatorsCount,
			expiresAt:       blockCount + cacheTimeout,
		}
	}
	return blockCount + validatorsCount, nil
}

// AddNetworkFee adds network fee for each witness script to transaction.
func (c *Client) AddNetworkFee(tx *transaction.Transaction, acc *wallet.Account) error {
	size := io.GetVarSize(tx)
	if acc.Contract != nil {
		netFee, sizeDelta := core.CalculateNetworkFee(acc.Contract.Script)
		tx.NetworkFee += netFee
		size += sizeDelta
	}
	for _, cosigner := range tx.Cosigners {
		contract, err := c.GetContractState(cosigner.Account)
		if err != nil {
			return err
		}
		if contract == nil {
			continue
		}
		netFee, sizeDelta := core.CalculateNetworkFee(contract.Script)
		tx.NetworkFee += netFee
		size += sizeDelta
	}
	tx.NetworkFee += util.Fixed8(int64(size) * int64(c.GetFeePerByte()))
	return nil
}

// GetFeePerByte returns transaction network fee per byte
func (c *Client) GetFeePerByte() util.Fixed8 {
	// TODO: make it a part of policy contract
	return util.Fixed8(1000)
}
