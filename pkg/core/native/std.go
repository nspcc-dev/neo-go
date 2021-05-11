package native

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"

	"github.com/mr-tron/base58"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Std represents StdLib contract.
type Std struct {
	interop.ContractMD
}

const (
	stdContractID = -2

	// stdMaxInputLength is the maximum input length for string-related methods.
	stdMaxInputLength = 1024
)

var (
	// ErrInvalidBase is returned when base is invalid.
	ErrInvalidBase = errors.New("invalid base")
	// ErrInvalidFormat is returned when string is not a number.
	ErrInvalidFormat = errors.New("invalid format")
	// ErrTooBigInput is returned when input exceeds size limit.
	ErrTooBigInput = errors.New("input is too big")
)

func newStd() *Std {
	s := &Std{ContractMD: *interop.NewContractMD(nativenames.StdLib, stdContractID)}
	defer s.UpdateHash()

	desc := newDescriptor("serialize", smartcontract.ByteArrayType,
		manifest.NewParameter("item", smartcontract.AnyType))
	md := newMethodAndPrice(s.serialize, 1<<12, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("deserialize", smartcontract.AnyType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md = newMethodAndPrice(s.deserialize, 1<<14, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("jsonSerialize", smartcontract.ByteArrayType,
		manifest.NewParameter("item", smartcontract.AnyType))
	md = newMethodAndPrice(s.jsonSerialize, 1<<12, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("jsonDeserialize", smartcontract.AnyType,
		manifest.NewParameter("json", smartcontract.ByteArrayType))
	md = newMethodAndPrice(s.jsonDeserialize, 1<<14, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("itoa", smartcontract.StringType,
		manifest.NewParameter("value", smartcontract.IntegerType),
		manifest.NewParameter("base", smartcontract.IntegerType))
	md = newMethodAndPrice(s.itoa, 1<<12, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("itoa", smartcontract.StringType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(s.itoa10, 1<<12, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("atoi", smartcontract.IntegerType,
		manifest.NewParameter("value", smartcontract.StringType),
		manifest.NewParameter("base", smartcontract.IntegerType))
	md = newMethodAndPrice(s.atoi, 1<<6, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("atoi", smartcontract.IntegerType,
		manifest.NewParameter("value", smartcontract.StringType))
	md = newMethodAndPrice(s.atoi10, 1<<6, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("base64Encode", smartcontract.StringType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md = newMethodAndPrice(s.base64Encode, 1<<5, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("base64Decode", smartcontract.ByteArrayType,
		manifest.NewParameter("s", smartcontract.StringType))
	md = newMethodAndPrice(s.base64Decode, 1<<5, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("base58Encode", smartcontract.StringType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md = newMethodAndPrice(s.base58Encode, 1<<13, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("base58Decode", smartcontract.ByteArrayType,
		manifest.NewParameter("s", smartcontract.StringType))
	md = newMethodAndPrice(s.base58Decode, 1<<10, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("memoryCompare", smartcontract.IntegerType,
		manifest.NewParameter("str1", smartcontract.ByteArrayType),
		manifest.NewParameter("str2", smartcontract.ByteArrayType))
	md = newMethodAndPrice(s.memoryCompare, 1<<5, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("memorySearch", smartcontract.IntegerType,
		manifest.NewParameter("mem", smartcontract.ByteArrayType),
		manifest.NewParameter("value", smartcontract.ByteArrayType))
	md = newMethodAndPrice(s.memorySearch2, 1<<6, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("memorySearch", smartcontract.IntegerType,
		manifest.NewParameter("mem", smartcontract.ByteArrayType),
		manifest.NewParameter("value", smartcontract.ByteArrayType),
		manifest.NewParameter("start", smartcontract.IntegerType))
	md = newMethodAndPrice(s.memorySearch3, 1<<6, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("memorySearch", smartcontract.IntegerType,
		manifest.NewParameter("mem", smartcontract.ByteArrayType),
		manifest.NewParameter("value", smartcontract.ByteArrayType),
		manifest.NewParameter("start", smartcontract.IntegerType),
		manifest.NewParameter("backward", smartcontract.BoolType))
	md = newMethodAndPrice(s.memorySearch4, 1<<6, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("stringSplit", smartcontract.ArrayType,
		manifest.NewParameter("str", smartcontract.StringType),
		manifest.NewParameter("separator", smartcontract.StringType))
	md = newMethodAndPrice(s.stringSplit2, 1<<8, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("stringSplit", smartcontract.ArrayType,
		manifest.NewParameter("str", smartcontract.StringType),
		manifest.NewParameter("separator", smartcontract.StringType),
		manifest.NewParameter("removeEmptyEntries", smartcontract.BoolType))
	md = newMethodAndPrice(s.stringSplit3, 1<<8, callflag.NoneFlag)
	s.AddMethod(md, desc)

	return s
}

func (s *Std) serialize(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	data, err := stackitem.SerializeItem(args[0])
	if err != nil {
		panic(err)
	}
	if len(data) > stackitem.MaxSize {
		panic(errors.New("too big item"))
	}

	return stackitem.NewByteArray(data)
}

func (s *Std) deserialize(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	data, err := args[0].TryBytes()
	if err != nil {
		panic(err)
	}

	item, err := stackitem.DeserializeItem(data)
	if err != nil {
		panic(err)
	}

	return item
}

func (s *Std) jsonSerialize(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	data, err := stackitem.ToJSON(args[0])
	if err != nil {
		panic(err)
	}
	if len(data) > stackitem.MaxSize {
		panic(errors.New("too big item"))
	}

	return stackitem.NewByteArray(data)
}

func (s *Std) jsonDeserialize(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	data, err := args[0].TryBytes()
	if err != nil {
		panic(err)
	}

	item, err := stackitem.FromJSON(data)
	if err != nil {
		panic(err)
	}

	return item
}

func (s *Std) itoa10(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	num := toBigInt(args[0])
	return stackitem.NewByteArray([]byte(num.Text(10)))
}

func (s *Std) itoa(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	num := toBigInt(args[0])
	base := toBigInt(args[1])
	if !base.IsInt64() {
		panic(ErrInvalidBase)
	}
	var str string
	switch b := base.Int64(); b {
	case 10:
		str = num.Text(10)
	case 16:
		if num.Sign() == 0 {
			str = "0"
			break
		}
		bs := bigint.ToBytes(num)
		reverse(bs)
		str = hex.EncodeToString(bs)
		if pad := bs[0] & 0xF8; pad == 0 || pad == 0xF8 {
			str = str[1:]
		}
		str = strings.ToUpper(str)
	default:
		panic(ErrInvalidBase)
	}
	return stackitem.NewByteArray([]byte(str))
}

func (s *Std) atoi10(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	num := s.toLimitedString(args[0])
	res := s.atoi10Aux(num)
	return stackitem.NewBigInteger(res)
}

func (s *Std) atoi10Aux(num string) *big.Int {
	bi, ok := new(big.Int).SetString(num, 10)
	if !ok {
		panic(ErrInvalidFormat)
	}
	return bi
}

func (s *Std) atoi(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	num := s.toLimitedString(args[0])
	base := toBigInt(args[1])
	if !base.IsInt64() {
		panic(ErrInvalidBase)
	}
	var bi *big.Int
	switch b := base.Int64(); b {
	case 10:
		bi = s.atoi10Aux(num)
	case 16:
		changed := len(num)%2 != 0
		if changed {
			num = "0" + num
		}
		bs, err := hex.DecodeString(num)
		if err != nil {
			panic(ErrInvalidFormat)
		}
		if changed && bs[0]&0x8 != 0 {
			bs[0] |= 0xF0
		}
		reverse(bs)
		bi = bigint.FromBytes(bs)
	default:
		panic(ErrInvalidBase)
	}

	return stackitem.NewBigInteger(bi)
}

func reverse(b []byte) {
	l := len(b)
	for i := 0; i < l/2; i++ {
		b[i], b[l-i-1] = b[l-i-1], b[i]
	}
}

func (s *Std) base64Encode(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	src := s.toLimitedBytes(args[0])
	result := base64.StdEncoding.EncodeToString(src)

	return stackitem.NewByteArray([]byte(result))
}

func (s *Std) base64Decode(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	src := s.toLimitedString(args[0])
	result, err := base64.StdEncoding.DecodeString(src)
	if err != nil {
		panic(err)
	}

	return stackitem.NewByteArray(result)
}

func (s *Std) base58Encode(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	src := s.toLimitedBytes(args[0])
	result := base58.Encode(src)

	return stackitem.NewByteArray([]byte(result))
}

func (s *Std) base58Decode(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	src := s.toLimitedString(args[0])
	result, err := base58.Decode(src)
	if err != nil {
		panic(err)
	}

	return stackitem.NewByteArray(result)
}

func (s *Std) memoryCompare(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	s1 := s.toLimitedBytes(args[0])
	s2 := s.toLimitedBytes(args[1])
	return stackitem.NewBigInteger(big.NewInt(int64(bytes.Compare(s1, s2))))
}

func (s *Std) memorySearch2(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	mem := s.toLimitedBytes(args[0])
	val := s.toLimitedBytes(args[1])
	index := s.memorySearchAux(mem, val, 0, false)
	return stackitem.NewBigInteger(big.NewInt(int64(index)))
}

func (s *Std) memorySearch3(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	mem := s.toLimitedBytes(args[0])
	val := s.toLimitedBytes(args[1])
	start := toUint32(args[2])
	index := s.memorySearchAux(mem, val, int(start), false)
	return stackitem.NewBigInteger(big.NewInt(int64(index)))
}

func (s *Std) memorySearch4(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	mem := s.toLimitedBytes(args[0])
	val := s.toLimitedBytes(args[1])
	start := toUint32(args[2])
	backward, err := args[3].TryBool()
	if err != nil {
		panic(err)
	}

	index := s.memorySearchAux(mem, val, int(start), backward)
	return stackitem.NewBigInteger(big.NewInt(int64(index)))
}

func (s *Std) memorySearchAux(mem, val []byte, start int, backward bool) int {
	if backward {
		if start > len(mem) { // panic in case if cap(mem) > len(mem) for some reasons
			panic("invalid start index")
		}
		return bytes.LastIndex(mem[:start], val)
	}

	index := bytes.Index(mem[start:], val)
	if index < 0 {
		return -1
	}
	return index + start
}

func (s *Std) stringSplit2(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	str := s.toLimitedString(args[0])
	sep := toString(args[1])
	return stackitem.NewArray(s.stringSplitAux(str, sep, false))
}

func (s *Std) stringSplit3(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	str := s.toLimitedString(args[0])
	sep := toString(args[1])
	removeEmpty, err := args[2].TryBool()
	if err != nil {
		panic(err)
	}

	return stackitem.NewArray(s.stringSplitAux(str, sep, removeEmpty))
}

func (s *Std) stringSplitAux(str, sep string, removeEmpty bool) []stackitem.Item {
	var result []stackitem.Item

	arr := strings.Split(str, sep)
	for i := range arr {
		if !removeEmpty || len(arr[i]) != 0 {
			result = append(result, stackitem.Make(arr[i]))
		}
	}

	return result
}

// Metadata implements Contract interface.
func (s *Std) Metadata() *interop.ContractMD {
	return &s.ContractMD
}

// Initialize implements Contract interface.
func (s *Std) Initialize(ic *interop.Context) error {
	return nil
}

// OnPersist implements Contract interface.
func (s *Std) OnPersist(ic *interop.Context) error {
	return nil
}

// PostPersist implements Contract interface.
func (s *Std) PostPersist(ic *interop.Context) error {
	return nil
}

func (s *Std) toLimitedBytes(item stackitem.Item) []byte {
	src, err := item.TryBytes()
	if err != nil {
		panic(err)
	}
	if len(src) > stdMaxInputLength {
		panic(ErrTooBigInput)
	}
	return src
}

func (s *Std) toLimitedString(item stackitem.Item) string {
	src := toString(item)
	if len(src) > stdMaxInputLength {
		panic(ErrTooBigInput)
	}
	return src
}
