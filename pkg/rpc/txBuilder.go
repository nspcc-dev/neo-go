package rpc

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/encoding/address"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/CityOfZion/neo-go/pkg/vm/opcode"
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

		wif, assetID, toAddress, amount, balancer = params.wif, params.assetID, params.address, params.value, params.balancer
	)

	fromAddress = wif.PrivateKey.Address()

	if fromAddressHash, err = address.DecodeUint160(fromAddress); err != nil {
		return nil, errs.Wrapf(err, "Failed to take script hash from address: %v", fromAddress)
	}

	if toAddressHash, err = address.DecodeUint160(toAddress); err != nil {
		return nil, errs.Wrapf(err, "Failed to take script hash from address: %v", toAddress)
	}
	tx.Attributes = append(tx.Attributes,
		transaction.Attribute{
			Usage: transaction.Script,
			Data:  fromAddressHash.BytesBE(),
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
func AddInputsAndUnspentsToTx(tx *transaction.Transaction, addr string, assetID util.Uint256, amount util.Fixed8, balancer BalanceGetter) error {
	scriptHash, err := address.DecodeUint160(addr)
	if err != nil {
		return errs.Wrapf(err, "failed to take script hash from address: %v", addr)
	}
	inputs, spent, err := balancer.CalculateInputs(addr, assetID, amount)
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
	witness.VerificationScript = wif.PrivateKey.PublicKey().GetVerificationScript()
	tx.Scripts = append(tx.Scripts, witness)
	tx.Hash()

	return nil
}

// GetInvocationScript returns NEO VM script containing transaction signature.
func GetInvocationScript(tx *transaction.Transaction, wif *keys.WIF) ([]byte, error) {
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
	return append([]byte{byte(opcode.PUSHBYTES64)}, signature...), nil
}

// CreateDeploymentScript returns a script that deploys given smart contract
// with its metadata.
func CreateDeploymentScript(avm []byte, contract *ContractDetails) ([]byte, error) {
	var props smartcontract.PropertyState

	script := new(bytes.Buffer)
	if err := vm.EmitBytes(script, []byte(contract.Description)); err != nil {
		return nil, err
	}
	if err := vm.EmitBytes(script, []byte(contract.Email)); err != nil {
		return nil, err
	}
	if err := vm.EmitBytes(script, []byte(contract.Author)); err != nil {
		return nil, err
	}
	if err := vm.EmitBytes(script, []byte(contract.Version)); err != nil {
		return nil, err
	}
	if err := vm.EmitBytes(script, []byte(contract.ProjectName)); err != nil {
		return nil, err
	}
	if contract.HasStorage {
		props |= smartcontract.HasStorage
	}
	if contract.HasDynamicInvocation {
		props |= smartcontract.HasDynamicInvoke
	}
	if contract.IsPayable {
		props |= smartcontract.IsPayable
	}
	if err := vm.EmitInt(script, int64(props)); err != nil {
		return nil, err
	}
	if err := vm.EmitInt(script, int64(contract.ReturnType)); err != nil {
		return nil, err
	}
	params := make([]byte, len(contract.Parameters))
	for k := range contract.Parameters {
		params[k] = byte(contract.Parameters[k])
	}
	if err := vm.EmitBytes(script, params); err != nil {
		return nil, err
	}
	if err := vm.EmitBytes(script, avm); err != nil {
		return nil, err
	}
	if err := vm.EmitSyscall(script, "Neo.Contract.Create"); err != nil {
		return nil, err
	}
	return script.Bytes(), nil
}

// expandArrayIntoScript pushes all FuncParam parameters from the given array
// into the given buffer in reverse order.
func expandArrayIntoScript(script *bytes.Buffer, slice []Param) error {
	for j := len(slice) - 1; j >= 0; j-- {
		fp, err := slice[j].GetFuncParam()
		if err != nil {
			return err
		}
		switch fp.Type {
		case ByteArray, Signature:
			str, err := fp.Value.GetBytesHex()
			if err != nil {
				return err
			}
			if err := vm.EmitBytes(script, str); err != nil {
				return err
			}
		case String:
			str, err := fp.Value.GetString()
			if err != nil {
				return err
			}
			if err := vm.EmitString(script, str); err != nil {
				return err
			}
		case Hash160:
			hash, err := fp.Value.GetUint160FromHex()
			if err != nil {
				return err
			}
			if err := vm.EmitBytes(script, hash.BytesBE()); err != nil {
				return err
			}
		case Hash256:
			hash, err := fp.Value.GetUint256()
			if err != nil {
				return err
			}
			if err := vm.EmitBytes(script, hash.BytesBE()); err != nil {
				return err
			}
		case PublicKey:
			str, err := fp.Value.GetString()
			if err != nil {
				return err
			}
			key, err := keys.NewPublicKeyFromString(string(str))
			if err != nil {
				return err
			}
			if err := vm.EmitBytes(script, key.Bytes()); err != nil {
				return err
			}
		case Integer:
			val, err := fp.Value.GetInt()
			if err != nil {
				return err
			}
			if err := vm.EmitInt(script, int64(val)); err != nil {
				return err
			}
		case Boolean:
			str, err := fp.Value.GetString()
			if err != nil {
				return err
			}
			switch str {
			case "true":
				err = vm.EmitInt(script, 1)
			case "false":
				err = vm.EmitInt(script, 0)
			default:
				err = errors.New("wrong boolean value")
			}
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("parameter type %v is not supported", fp.Type)
		}
	}
	return nil
}

// CreateFunctionInvocationScript creates a script to invoke given contract with
// given parameters.
func CreateFunctionInvocationScript(contract util.Uint160, params Params) ([]byte, error) {
	script := new(bytes.Buffer)
	for i := len(params) - 1; i >= 0; i-- {
		switch params[i].Type {
		case stringT:
			if err := vm.EmitString(script, params[i].String()); err != nil {
				return nil, err
			}
		case numberT:
			num, err := params[i].GetInt()
			if err != nil {
				return nil, err
			}
			if err := vm.EmitString(script, strconv.Itoa(num)); err != nil {
				return nil, err
			}
		case arrayT:
			slice, err := params[i].GetArray()
			if err != nil {
				return nil, err
			}
			err = expandArrayIntoScript(script, slice)
			if err != nil {
				return nil, err
			}
			err = vm.EmitInt(script, int64(len(slice)))
			if err != nil {
				return nil, err
			}
			err = vm.EmitOpcode(script, opcode.PACK)
			if err != nil {
				return nil, err
			}
		}
	}

	if err := vm.EmitAppCall(script, contract, false); err != nil {
		return nil, err
	}
	return script.Bytes(), nil
}

// CreateInvocationScript creates a script to invoke given contract with
// given parameters. It differs from CreateFunctionInvocationScript in that it
// expects one array of FuncParams and expands it onto the stack as independent
// elements.
func CreateInvocationScript(contract util.Uint160, funcParams []Param) ([]byte, error) {
	script := new(bytes.Buffer)
	err := expandArrayIntoScript(script, funcParams)
	if err != nil {
		return nil, err
	}
	if err = vm.EmitAppCall(script, contract, false); err != nil {
		return nil, err
	}
	return script.Bytes(), nil
}
