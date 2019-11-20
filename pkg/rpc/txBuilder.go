package rpc

import (
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	errs "github.com/pkg/errors"
)

// CreateRawContractTransaction returns contract-type Transaction built from specified parameters.
func CreateRawContractTransaction(params ContractTxParams) (*transaction.Transaction, error) {
	var (
		err                            error
		tx                             = transaction.NewContractTX()
		toAddressHash, fromAddressHash util.Uint160
		fromAddress                    string
		receiverOutput                 *transaction.Output

		wif, assetID, address, amount, balancer = params.wif, params.assetID, params.address, params.value, params.balancer
	)

	fromAddress = wif.PrivateKey.Address()

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

	if err = AddInputsAndUnspentsToTx(tx, fromAddress, assetID, amount, balancer); err != nil {
		return nil, errs.Wrap(err, "failed to add inputs and unspents to transaction")
	}
	receiverOutput = transaction.NewOutput(assetID, amount, toAddressHash)
	tx.AddOutput(receiverOutput)
	if err = SignTx(tx, &wif); err != nil {
		return nil, errs.Wrap(err, "failed to sign tx")
	}

	return tx, nil
}

// AddInputsAndUnspentsToTx adds inputs needed to transaction and one output
// with change.
func AddInputsAndUnspentsToTx(tx *transaction.Transaction, address string, assetID util.Uint256, amount util.Fixed8, balancer BalanceGetter) error {
	scriptHash, err := crypto.Uint160DecodeAddress(address)
	if err != nil {
		return errs.Wrapf(err, "failed to take script hash from address: %v", address)
	}
	inputs, spent, err := balancer.CalculateInputs(address, assetID, amount)
	if err != nil {
		return errs.Wrap(err, "failed to get inputs")
	}
	for _, input := range inputs {
		tx.AddInput(&input)
	}

	if senderUnspent := spent - amount; senderUnspent > 0 {
		senderOutput := transaction.NewOutput(assetID, senderUnspent, scriptHash)
		tx.AddOutput(senderOutput)
	}
	return nil
}

// SignTx signs given transaction in-place using given key.
func SignTx(tx *transaction.Transaction, wif *keys.WIF) error {
	var witness transaction.Witness
	var err error

	if witness.InvocationScript, err = GetInvocationScript(tx, wif); err != nil {
		return errs.Wrap(err, "failed to create invocation script")
	}
	witness.VerificationScript = wif.GetVerificationScript()
	tx.Scripts = append(tx.Scripts, &witness)
	tx.Hash()

	return nil
}

// GetInvocationScript returns NEO VM script containing transaction signature.
func GetInvocationScript(tx *transaction.Transaction, wif *keys.WIF) ([]byte, error) {
	const (
		pushbytes64 = 0x40
	)
	var (
		err       error
		buf       = io.NewBufBinWriter()
		signature []byte
	)
	tx.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return nil, errs.Wrap(buf.Err, "Failed to encode transaction to binary")
	}
	data := buf.Bytes()
	signature, err = wif.PrivateKey.Sign(data[:(len(data) - 1)])
	if err != nil {
		return nil, errs.Wrap(err, "Failed ti sign transaction with private key")
	}
	return append([]byte{pushbytes64}, signature...), nil
}
