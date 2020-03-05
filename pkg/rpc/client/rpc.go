package client

import (
	"encoding/hex"

	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/pkg/errors"
)

// GetAccountState returns detailed information about a NEO account.
func (c *Client) GetAccountState(address string) (*result.AccountState, error) {
	var (
		params = request.NewRawParams(address)
		resp   = &result.AccountState{}
	)
	if err := c.performRequest("getaccountstate", params, resp); err != nil {
		return nil, err
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
	if err = c.performRequest("getblock", params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetClaimable returns tx outputs which can be claimed.
func (c *Client) GetClaimable(address string) (*result.ClaimableInfo, error) {
	params := request.NewRawParams(address)
	resp := new(result.ClaimableInfo)
	if err := c.performRequest("getclaimable", params, resp); err != nil {
		return nil, err
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
	r := io.NewBinReaderFromBuf(txBytes)
	tx := new(transaction.Transaction)
	tx.DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}
	return tx, nil
}

// GetRawTransactionVerbose returns a transaction wrapper with additional
// metadata by transaction's hash.
func (c *Client) GetRawTransactionVerbose(hash util.Uint256) (*result.TransactionOutputRaw, error) {
	var (
		params = request.NewRawParams(hash.StringLE(), 1)
		resp   = &result.TransactionOutputRaw{}
		err    error
	)
	if err = c.performRequest("getrawtransaction", params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetUnspents returns UTXOs for the given NEO account.
func (c *Client) GetUnspents(address string) (*result.Unspents, error) {
	var (
		params = request.NewRawParams(address)
		resp   = &result.Unspents{}
	)
	if err := c.performRequest("getunspents", params, resp); err != nil {
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

// Invoke returns the results after calling the smart contract scripthash
// with the given parameters.
func (c *Client) Invoke(script string, params []smartcontract.Parameter) (*result.Invoke, error) {
	var (
		p    = request.NewRawParams(script, params)
		resp = &result.Invoke{}
	)
	if err := c.performRequest("invoke", p, resp); err != nil {
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

// TransferAsset sends an amount of specific asset to a given address.
// This call requires open wallet. (`wif` key in client struct.)
// If response.Result is `true` then transaction was formed correctly and was written in blockchain.
func (c *Client) TransferAsset(asset util.Uint256, address string, amount util.Fixed8) (util.Uint256, error) {
	var (
		err      error
		rawTx    *transaction.Transaction
		txParams = request.ContractTxParams{
			AssetID:  asset,
			Address:  address,
			Value:    amount,
			WIF:      c.WIF(),
			Balancer: c.Balancer(),
		}
		resp util.Uint256
	)

	if rawTx, err = request.CreateRawContractTransaction(txParams); err != nil {
		return resp, errors.Wrap(err, "failed to create raw transaction")
	}
	if err = c.SendRawTransaction(rawTx); err != nil {
		return resp, errors.Wrap(err, "failed to send raw transaction")
	}
	return rawTx.Hash(), nil
}

// SignAndPushInvocationTx signs and pushes given script as an invocation
// transaction  using given wif to sign it and spending the amount of gas
// specified. It returns a hash of the invocation transaction and an error.
func (c *Client) SignAndPushInvocationTx(script []byte, wif *keys.WIF, gas util.Fixed8) (util.Uint256, error) {
	var txHash util.Uint256
	var err error

	tx := transaction.NewInvocationTX(script, gas)

	fromAddress := wif.PrivateKey.Address()

	if gas > 0 {
		if err = request.AddInputsAndUnspentsToTx(tx, fromAddress, core.UtilityTokenID(), gas, c); err != nil {
			return txHash, errors.Wrap(err, "failed to add inputs and unspents to transaction")
		}
	}

	acc, err := wallet.NewAccountFromWIF(wif.S)
	if err != nil {
		return txHash, err
	} else if err = acc.SignTx(tx); err != nil {
		return txHash, errors.Wrap(err, "failed to sign tx")
	}
	txHash = tx.Hash()
	err = c.SendRawTransaction(tx)

	if err != nil {
		return txHash, errors.Wrap(err, "failed sendning tx")
	}
	return txHash, nil
}
