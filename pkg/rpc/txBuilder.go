package rpc

import (
	"bytes"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/wallet"
	errs "github.com/pkg/errors"
)

func CreateRawContractTransaction(params ContractTxParams) (*transaction.Transaction, error) {
	var (
		err                            error
		tx                             = transaction.NewContractTX()
		toAddressHash, fromAddressHash util.Uint160
		fromAddress                    string
		senderOutput, receiverOutput   *transaction.Output
		inputs                         []transaction.Input
		spent                          util.Fixed8
		witness                        transaction.Witness

		wif, assetID, address, amount, balancer = params.wif, params.assetId, params.address, params.value, params.balancer
	)

	if fromAddress, err = wif.PrivateKey.Address(); err != nil {
		return nil, errs.Wrapf(err, "Failed to take address from WIF: %v", wif.S)
	}

	if fromAddressHash, err = crypto.Uint160DecodeAddress(fromAddress); err != nil {
		return nil, errs.Wrapf(err, "Failed to take script hash from address: %v", fromAddress)
	}

	if toAddressHash, err = crypto.Uint160DecodeAddress(address); err != nil {
		return nil, errs.Wrapf(err, "Failed to take script hash from address: %v", address)
	}
	tx.Attributes = append(tx.Attributes,
		&transaction.Attribute{
			Usage: transaction.Script,
			Data:  fromAddressHash.Bytes(),
		})

	if inputs, spent, err = balancer.CalculateInputs(fromAddress, assetID, amount); err != nil {
		return nil, errs.Wrap(err, "Failed to get inputs for transaction")
	}
	for _, input := range inputs {
		tx.AddInput(&input)
	}

	if senderUnspent := spent - amount; senderUnspent > 0 {
		senderOutput = transaction.NewOutput(assetID, senderUnspent, fromAddressHash)
		tx.AddOutput(senderOutput)
	}
	receiverOutput = transaction.NewOutput(assetID, amount, toAddressHash)
	tx.AddOutput(receiverOutput)

	if witness.InvocationScript, err = GetInvocationScript(tx, wif); err != nil {
		return nil, errs.Wrap(err, "Failed to create invocation script")
	}
	if witness.VerificationScript, err = wif.GetVerificationScript(); err != nil {
		return nil, errs.Wrap(err, "Failed to create verification script")
	}
	tx.Scripts = append(tx.Scripts, &witness)
	tx.Hash()

	return tx, nil
}

func GetInvocationScript(tx *transaction.Transaction, wif wallet.WIF) ([]byte, error) {
	const (
		pushbytes64 = 0x40
	)
	var (
		err       error
		buf       = new(bytes.Buffer)
		signature []byte
	)
	if err = tx.EncodeBinary(buf); err != nil {
		return nil, errs.Wrap(err, "Failed to encode transaction to binary")
	}
	data := buf.Bytes()
	signature, err = wif.PrivateKey.Sign(data[:(len(data) - 1)])
	if err != nil {
		return nil, errs.Wrap(err, "Failed ti sign transaction with private key")
	}
	return append([]byte{pushbytes64}, signature...), nil
}
