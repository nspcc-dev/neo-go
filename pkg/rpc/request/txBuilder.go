package request

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// ExpandArrayIntoScript pushes all FuncParam parameters from the given array
// into the given buffer in the reverse order.
func ExpandArrayIntoScript(script *io.BinWriter, slice []Param) error {
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
			bi, err := fp.Value.GetBigInt()
			if err != nil {
				return err
			}
			emit.BigInt(script, bi)
		case smartcontract.BoolType:
			val, err := fp.Value.GetBoolean() // not GetBooleanStrict(), because that's the way C# code works
			if err != nil {
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
		case smartcontract.AnyType:
			if fp.Value.IsNull() {
				emit.Opcodes(script, opcode.PUSHNULL)
			}
		default:
			return fmt.Errorf("parameter type %v is not supported", fp.Type)
		}
	}
	return script.Err
}

// CreateFunctionInvocationScript creates a script to invoke the given contract with
// the given parameters.
func CreateFunctionInvocationScript(contract util.Uint160, method string, param *Param) ([]byte, error) {
	script := io.NewBufBinWriter()
	if param == nil {
		emit.Opcodes(script.BinWriter, opcode.NEWARRAY0)
	} else if slice, err := param.GetArray(); err == nil {
		err = ExpandArrayIntoScript(script.BinWriter, slice)
		if err != nil {
			return nil, err
		}
		emit.Int(script.BinWriter, int64(len(slice)))
		emit.Opcodes(script.BinWriter, opcode.PACK)
	} else {
		return nil, fmt.Errorf("failed to convert %s to script parameter", param)
	}

	emit.AppCallNoArgs(script.BinWriter, contract, method, callflag.All)
	return script.Bytes(), nil
}
