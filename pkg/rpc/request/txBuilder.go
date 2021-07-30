package request

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// ExpandArrayIntoScript pushes all FuncParam parameters from the given array
// into the given buffer in reverse order.
func ExpandArrayIntoScript(script io.BinaryWriter, slice []Param) error {
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
			val, ok := fp.Value.Value.(bool)
			if !ok {
				return errors.New("not a bool")
			}
			if val {
				emit.Int(script, 1)
			} else {
				emit.Int(script, 0)
			}
		case smartcontract.ArrayType:
			val, err := fp.Value.GetArray()
			if err != nil {
				return err
			}
			err = ExpandArrayIntoScript(script, val)
			if err != nil {
				return err
			}
			emit.Int(script, int64(len(val)))
			emit.Opcodes(script, opcode.PACK)
		default:
			return fmt.Errorf("parameter type %v is not supported", fp.Type)
		}
	}
	return nil
}

// CreateFunctionInvocationScript creates a script to invoke given contract with
// given parameters.
func CreateFunctionInvocationScript(contract util.Uint160, method string, params Params) ([]byte, error) {
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
		case BooleanT:
			val := params[i].GetBoolean()
			emit.Bool(script.BinWriter, val)
		case ArrayT:
			slice, err := params[i].GetArray()
			if err != nil {
				return nil, err
			}
			err = ExpandArrayIntoScript(script.BinWriter, slice)
			if err != nil {
				return nil, err
			}
			emit.Int(script.BinWriter, int64(len(slice)))
			emit.Opcodes(script.BinWriter, opcode.PACK)
		}
	}

	emit.AppCallNoArgs(script.BinWriter, contract, method, callflag.All)
	return script.Bytes(), nil
}
