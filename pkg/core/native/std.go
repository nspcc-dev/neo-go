package native

import (
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

const stdContractID = -2

var (
	// ErrInvalidBase is returned when base is invalid.
	ErrInvalidBase = errors.New("invalid base")
	// ErrInvalidFormat is returned when string is not a number.
	ErrInvalidFormat = errors.New("invalid format")
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

	desc = newDescriptor("atoi", smartcontract.IntegerType,
		manifest.NewParameter("value", smartcontract.StringType),
		manifest.NewParameter("base", smartcontract.IntegerType))
	md = newMethodAndPrice(s.atoi, 1<<12, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("base64Encode", smartcontract.StringType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md = newMethodAndPrice(s.base64Encode, 1<<12, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("base64Decode", smartcontract.ByteArrayType,
		manifest.NewParameter("s", smartcontract.StringType))
	md = newMethodAndPrice(s.base64Decode, 1<<12, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("base58Encode", smartcontract.StringType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md = newMethodAndPrice(s.base58Encode, 1<<12, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = newDescriptor("base58Decode", smartcontract.ByteArrayType,
		manifest.NewParameter("s", smartcontract.StringType))
	md = newMethodAndPrice(s.base58Decode, 1<<12, callflag.NoneFlag)
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

func (s *Std) atoi(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	num := toString(args[0])
	base := toBigInt(args[1])
	if !base.IsInt64() {
		panic(ErrInvalidBase)
	}
	var bi *big.Int
	switch b := base.Int64(); b {
	case 10:
		var ok bool
		bi, ok = new(big.Int).SetString(num, int(b))
		if !ok {
			panic(ErrInvalidFormat)
		}
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
	src, err := args[0].TryBytes()
	if err != nil {
		panic(err)
	}
	result := base64.StdEncoding.EncodeToString(src)

	return stackitem.NewByteArray([]byte(result))
}

func (s *Std) base64Decode(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	src := toString(args[0])
	result, err := base64.StdEncoding.DecodeString(src)
	if err != nil {
		panic(err)
	}

	return stackitem.NewByteArray(result)
}

func (s *Std) base58Encode(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	src, err := args[0].TryBytes()
	if err != nil {
		panic(err)
	}
	result := base58.Encode(src)

	return stackitem.NewByteArray([]byte(result))
}

func (s *Std) base58Decode(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	src := toString(args[0])
	result, err := base58.Decode(src)
	if err != nil {
		panic(err)
	}

	return stackitem.NewByteArray(result)
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
