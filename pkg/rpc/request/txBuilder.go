package request

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// CreateDeploymentScript returns a script that deploys given smart contract
// with its metadata.
func CreateDeploymentScript(avm []byte, manif *manifest.Manifest) ([]byte, error) {
	script := io.NewBufBinWriter()
	rawManifest, err := manif.MarshalJSON()
	if err != nil {
		return nil, err
	}
	emit.Bytes(script.BinWriter, rawManifest)
	emit.Bytes(script.BinWriter, avm)
	emit.Syscall(script.BinWriter, interopnames.SystemContractCreate)
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
		case smartcontract.ByteArrayType:
			str, err := fp.Value.GetBytesBase64()
			if err != nil {
				return err
			}
			emit.Bytes(script, str)
		case smartcontract.SignatureType:
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
		case smartcontract.ArrayType:
			val, err := fp.Value.GetArray()
			if err != nil {
				return err
			}
			err = expandArrayIntoScript(script, val)
			if err != nil {
				return err
			}
			emit.Int(script, int64(len(val)))
			emit.Opcode(script, opcode.PACK)
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
