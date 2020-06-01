package request

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// DetailsToSCProperties extract the fields needed from ContractDetails
// and converts them to smartcontract.PropertyState.
func DetailsToSCProperties(contract *smartcontract.ContractDetails) smartcontract.PropertyState {
	var props smartcontract.PropertyState
	if contract.HasStorage {
		props |= smartcontract.HasStorage
	}
	if contract.HasDynamicInvocation {
		props |= smartcontract.HasDynamicInvoke
	}
	if contract.IsPayable {
		props |= smartcontract.IsPayable
	}
	return props
}

// CreateDeploymentScript returns a script that deploys given smart contract
// with its metadata.
func CreateDeploymentScript(avm []byte, contract *smartcontract.ContractDetails) ([]byte, error) {
	script := io.NewBufBinWriter()
	emit.Bytes(script.BinWriter, []byte(contract.Description))
	emit.Bytes(script.BinWriter, []byte(contract.Email))
	emit.Bytes(script.BinWriter, []byte(contract.Author))
	emit.Bytes(script.BinWriter, []byte(contract.Version))
	emit.Bytes(script.BinWriter, []byte(contract.ProjectName))
	emit.Int(script.BinWriter, int64(DetailsToSCProperties(contract)))
	emit.Int(script.BinWriter, int64(contract.ReturnType))
	params := make([]byte, len(contract.Parameters))
	for k := range contract.Parameters {
		params[k] = byte(contract.Parameters[k])
	}
	emit.Bytes(script.BinWriter, params)
	emit.Bytes(script.BinWriter, avm)
	emit.Syscall(script.BinWriter, "Neo.Contract.Create")
	return script.Bytes(), nil
}

// expandArrayIntoScript pushes all FuncParam parameters from the given array
// into the given buffer in reverse order.
func expandArrayIntoScript(script *io.BinWriter, slice []Param) error {
	for j := len(slice) - 1; j >= 0; j-- {
		fp, err := slice[j].GetFuncParam()
		if err != nil {
			return err
		}
		switch fp.Type {
		case smartcontract.ByteArrayType, smartcontract.SignatureType:
			str, err := fp.Value.GetBytesHex()
			if err != nil {
				return err
			}
			emit.Bytes(script, str)
		case smartcontract.StringType:
			str, err := fp.Value.GetString()
			if err != nil {
				return err
			}
			emit.String(script, str)
		case smartcontract.Hash160Type:
			hash, err := fp.Value.GetUint160FromHex()
			if err != nil {
				return err
			}
			emit.Bytes(script, hash.BytesBE())
		case smartcontract.Hash256Type:
			hash, err := fp.Value.GetUint256()
			if err != nil {
				return err
			}
			emit.Bytes(script, hash.BytesBE())
		case smartcontract.PublicKeyType:
			str, err := fp.Value.GetString()
			if err != nil {
				return err
			}
			key, err := keys.NewPublicKeyFromString(string(str))
			if err != nil {
				return err
			}
			emit.Bytes(script, key.Bytes())
		case smartcontract.IntegerType:
			val, err := fp.Value.GetInt()
			if err != nil {
				return err
			}
			emit.Int(script, int64(val))
		case smartcontract.BoolType:
			str, err := fp.Value.GetString()
			if err != nil {
				return err
			}
			switch str {
			case "true":
				emit.Int(script, 1)
			case "false":
				emit.Int(script, 0)
			default:
				return errors.New("wrong boolean value")
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
	script := io.NewBufBinWriter()
	for i := len(params) - 1; i >= 0; i-- {
		switch params[i].Type {
		case StringT:
			emit.String(script.BinWriter, params[i].String())
		case NumberT:
			num, err := params[i].GetInt()
			if err != nil {
				return nil, err
			}
			emit.String(script.BinWriter, strconv.Itoa(num))
		case ArrayT:
			slice, err := params[i].GetArray()
			if err != nil {
				return nil, err
			}
			err = expandArrayIntoScript(script.BinWriter, slice)
			if err != nil {
				return nil, err
			}
			emit.Int(script.BinWriter, int64(len(slice)))
			emit.Opcode(script.BinWriter, opcode.PACK)
		}
	}

	emit.AppCall(script.BinWriter, contract)
	return script.Bytes(), nil
}

// CreateInvocationScript creates a script to invoke given contract with
// given parameters. It differs from CreateFunctionInvocationScript in that it
// expects one array of FuncParams and expands it onto the stack as independent
// elements.
func CreateInvocationScript(contract util.Uint160, funcParams []Param) ([]byte, error) {
	script := io.NewBufBinWriter()
	err := expandArrayIntoScript(script.BinWriter, funcParams)
	if err != nil {
		return nil, err
	}
	emit.AppCall(script.BinWriter, contract)
	return script.Bytes(), nil
}
