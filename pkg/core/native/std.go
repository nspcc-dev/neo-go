package native

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mr-tron/base58"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativeids"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	base58neogo "github.com/nspcc-dev/neo-go/pkg/encoding/base58"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Std represents an StdLib contract.
type Std struct {
	interop.ContractMD
}

// stdMaxInputLength is the maximum input length for string-related methods.
const stdMaxInputLength = 1024

var (
	// ErrInvalidBase is returned when the base is invalid.
	ErrInvalidBase = errors.New("invalid base")
	// ErrInvalidFormat is returned when the string is not a number.
	ErrInvalidFormat = errors.New("invalid format")
	// ErrTooBigInput is returned when the input exceeds the size limit.
	ErrTooBigInput = errors.New("input is too big")
)

func newStd() *Std {
	s := &Std{ContractMD: *interop.NewContractMD(nativenames.StdLib, nativeids.StdLib)}
	defer s.BuildHFSpecificMD(s.ActiveIn())

	desc := NewDescriptor("serialize", smartcontract.ByteArrayType,
		manifest.NewParameter("item", smartcontract.AnyType))
	md := NewMethodAndPrice(s.serialize, 1<<12, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("deserialize", smartcontract.AnyType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(s.deserialize, 1<<14, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("jsonSerialize", smartcontract.ByteArrayType,
		manifest.NewParameter("item", smartcontract.AnyType))
	md = NewMethodAndPrice(s.jsonSerialize, 1<<12, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("jsonDeserialize", smartcontract.AnyType,
		manifest.NewParameter("json", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(s.jsonDeserialize, 1<<14, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("itoa", smartcontract.StringType,
		manifest.NewParameter("value", smartcontract.IntegerType),
		manifest.NewParameter("base", smartcontract.IntegerType))
	md = NewMethodAndPrice(s.itoa, 1<<12, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("itoa", smartcontract.StringType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = NewMethodAndPrice(s.itoa10, 1<<12, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("atoi", smartcontract.IntegerType,
		manifest.NewParameter("value", smartcontract.StringType),
		manifest.NewParameter("base", smartcontract.IntegerType))
	md = NewMethodAndPrice(s.atoi, 1<<6, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("atoi", smartcontract.IntegerType,
		manifest.NewParameter("value", smartcontract.StringType))
	md = NewMethodAndPrice(s.atoi10, 1<<6, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("base64Encode", smartcontract.StringType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(s.base64Encode, 1<<5, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("base64Decode", smartcontract.ByteArrayType,
		manifest.NewParameter("s", smartcontract.StringType))
	md = NewMethodAndPrice(s.base64Decode, 1<<5, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("base64UrlEncode", smartcontract.StringType,
		manifest.NewParameter("data", smartcontract.StringType))
	md = NewMethodAndPrice(s.base64UrlEncode, 1<<5, callflag.NoneFlag, config.HFEchidna)
	s.AddMethod(md, desc)

	desc = NewDescriptor("base64UrlDecode", smartcontract.StringType,
		manifest.NewParameter("s", smartcontract.StringType))
	md = NewMethodAndPrice(s.base64UrlDecode, 1<<5, callflag.NoneFlag, config.HFEchidna)
	s.AddMethod(md, desc)

	desc = NewDescriptor("base58Encode", smartcontract.StringType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(s.base58Encode, 1<<13, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("base58Decode", smartcontract.ByteArrayType,
		manifest.NewParameter("s", smartcontract.StringType))
	md = NewMethodAndPrice(s.base58Decode, 1<<10, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("base58CheckEncode", smartcontract.StringType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(s.base58CheckEncode, 1<<16, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("base58CheckDecode", smartcontract.ByteArrayType,
		manifest.NewParameter("s", smartcontract.StringType))
	md = NewMethodAndPrice(s.base58CheckDecode, 1<<16, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("memoryCompare", smartcontract.IntegerType,
		manifest.NewParameter("str1", smartcontract.ByteArrayType),
		manifest.NewParameter("str2", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(s.memoryCompare, 1<<5, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("memorySearch", smartcontract.IntegerType,
		manifest.NewParameter("mem", smartcontract.ByteArrayType),
		manifest.NewParameter("value", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(s.memorySearch2, 1<<6, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("memorySearch", smartcontract.IntegerType,
		manifest.NewParameter("mem", smartcontract.ByteArrayType),
		manifest.NewParameter("value", smartcontract.ByteArrayType),
		manifest.NewParameter("start", smartcontract.IntegerType))
	md = NewMethodAndPrice(s.memorySearch3, 1<<6, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("memorySearch", smartcontract.IntegerType,
		manifest.NewParameter("mem", smartcontract.ByteArrayType),
		manifest.NewParameter("value", smartcontract.ByteArrayType),
		manifest.NewParameter("start", smartcontract.IntegerType),
		manifest.NewParameter("backward", smartcontract.BoolType))
	md = NewMethodAndPrice(s.memorySearch4, 1<<6, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("stringSplit", smartcontract.ArrayType,
		manifest.NewParameter("str", smartcontract.StringType),
		manifest.NewParameter("separator", smartcontract.StringType))
	md = NewMethodAndPrice(s.stringSplit2, 1<<8, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("stringSplit", smartcontract.ArrayType,
		manifest.NewParameter("str", smartcontract.StringType),
		manifest.NewParameter("separator", smartcontract.StringType),
		manifest.NewParameter("removeEmptyEntries", smartcontract.BoolType))
	md = NewMethodAndPrice(s.stringSplit3, 1<<8, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("strLen", smartcontract.IntegerType,
		manifest.NewParameter("str", smartcontract.StringType))
	md = NewMethodAndPrice(s.strLen, 1<<8, callflag.NoneFlag)
	s.AddMethod(md, desc)

	desc = NewDescriptor("hexEncode", smartcontract.StringType,
		manifest.NewParameter("bytes", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(s.hexEncode, 1<<5, callflag.NoneFlag, config.HFFaun)
	s.AddMethod(md, desc)

	desc = NewDescriptor("hexDecode", smartcontract.ByteArrayType,
		manifest.NewParameter("str", smartcontract.StringType))
	md = NewMethodAndPrice(s.hexDecode, 1<<5, callflag.NoneFlag, config.HFFaun)
	s.AddMethod(md, desc)

	return s
}

func (s *Std) serialize(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	data, err := ic.DAO.GetItemCtx().Serialize(args[0], false)
	if err != nil {
		panic(err)
	}
	if len(data) > stackitem.MaxSize {
		panic(errors.New("too big item"))
	}

	return stackitem.NewByteArray(bytes.Clone(data)) // Serialization context can be reused.
}

func (s *Std) deserialize(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	data, err := args[0].TryBytes()
	if err != nil {
		panic(err)
	}

	item, err := stackitem.Deserialize(data)
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

func (s *Std) jsonDeserialize(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	data, err := args[0].TryBytes()
	if err != nil {
		panic(err)
	}

	item, err := stackitem.FromJSON(data, stackitem.MaxDeserialized, ic.IsHardforkEnabled(config.HFBasilisk))
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
		slices.Reverse(bs)
		str = hex.EncodeToString(bs)
		if pad := bs[0] & 0xF8; pad == 0 || pad == 0xF8 {
			str = str[1:]
		}
	default:
		panic(ErrInvalidBase)
	}
	return stackitem.NewByteArray([]byte(str))
}

func (s *Std) atoi10(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	num := toLimitedString(args[0])
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
	num := toLimitedString(args[0])
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
		slices.Reverse(bs)
		bi = bigint.FromBytes(bs)
	default:
		panic(ErrInvalidBase)
	}

	return stackitem.NewBigInteger(bi)
}

func (s *Std) base64Encode(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	src := toLimitedBytes(args[0])
	result := base64.StdEncoding.EncodeToString(src)

	return stackitem.NewByteArray([]byte(result))
}

func (s *Std) base64Decode(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	src := stripSpaces(toLimitedString(args[0]))
	result, err := base64.StdEncoding.DecodeString(src)
	if err != nil {
		panic(err)
	}

	return stackitem.NewByteArray(result)
}

func (s *Std) hexEncode(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	src := toLimitedBytes(args[0])
	return stackitem.NewByteArray([]byte(hex.EncodeToString(src)))
}

func (s *Std) hexDecode(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	src := toLimitedString(args[0])
	dst, err := hex.DecodeString(src)
	if err != nil {
		panic(fmt.Errorf("hexDecode: invalid hex string %q: %w", src, err))
	}
	return stackitem.NewByteArray(dst)
}

func (s *Std) base64UrlEncode(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	src := toLimitedBytes(args[0])
	result := base64.URLEncoding.EncodeToString(src)

	return stackitem.NewByteArray([]byte(result))
}

func (s *Std) base64UrlDecode(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	src := stripSpaces(toLimitedString(args[0]))
	result, err := base64.URLEncoding.DecodeString(src)
	if err != nil {
		panic(err)
	}

	return stackitem.NewByteArray(result)
}

func (s *Std) base58Encode(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	src := toLimitedBytes(args[0])
	result := base58.Encode(src)

	return stackitem.NewByteArray([]byte(result))
}

func (s *Std) base58Decode(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	src := toLimitedString(args[0])
	result, err := base58.Decode(src)
	if err != nil {
		panic(err)
	}

	return stackitem.NewByteArray(result)
}

func (s *Std) base58CheckEncode(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	src := toLimitedBytes(args[0])
	result := base58neogo.CheckEncode(src)

	return stackitem.NewByteArray([]byte(result))
}

func (s *Std) base58CheckDecode(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	src := toLimitedString(args[0])
	result, err := base58neogo.CheckDecode(src)
	if err != nil {
		panic(err)
	}

	return stackitem.NewByteArray(result)
}

func (s *Std) memoryCompare(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	s1 := toLimitedBytes(args[0])
	s2 := toLimitedBytes(args[1])
	return stackitem.NewBigInteger(big.NewInt(int64(bytes.Compare(s1, s2))))
}

func (s *Std) memorySearch2(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	mem := toLimitedBytes(args[0])
	val := toLimitedBytes(args[1])
	index := s.memorySearchAux(mem, val, 0, false)
	return stackitem.NewBigInteger(big.NewInt(int64(index)))
}

func (s *Std) memorySearch3(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	mem := toLimitedBytes(args[0])
	val := toLimitedBytes(args[1])
	start := toUint32(args[2])
	index := s.memorySearchAux(mem, val, int(start), false)
	return stackitem.NewBigInteger(big.NewInt(int64(index)))
}

func (s *Std) memorySearch4(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	mem := toLimitedBytes(args[0])
	val := toLimitedBytes(args[1])
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
	str := toLimitedString(args[0])
	sep := toString(args[1])
	return stackitem.NewArray(s.stringSplitAux(str, sep, false))
}

func (s *Std) stringSplit3(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	str := toLimitedString(args[0])
	sep := toString(args[1])
	removeEmpty, err := args[2].TryBool()
	if err != nil {
		panic(err)
	}

	return stackitem.NewArray(s.stringSplitAux(str, sep, removeEmpty))
}

func (s *Std) stringSplitAux(str, sep string, removeEmpty bool) []stackitem.Item {
	var result []stackitem.Item

	for s := range strings.SplitSeq(str, sep) {
		if !removeEmpty || len(s) != 0 {
			result = append(result, stackitem.Make(s))
		}
	}

	return result
}

func (s *Std) strLen(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	str := toLimitedString(args[0])

	return stackitem.NewBigInteger(big.NewInt(int64(utf8.RuneCountInString(str))))
}

// Metadata implements the Contract interface.
func (s *Std) Metadata() *interop.ContractMD {
	return &s.ContractMD
}

// Initialize implements the Contract interface.
func (s *Std) Initialize(ic *interop.Context, hf *config.Hardfork, newMD *interop.HFSpecificContractMD) error {
	return nil
}

// InitializeCache implements the Contract interface.
func (s *Std) InitializeCache(_ interop.IsHardforkEnabled, blockHeight uint32, d *dao.Simple) error {
	return nil
}

// OnPersist implements the Contract interface.
func (s *Std) OnPersist(ic *interop.Context) error {
	return nil
}

// PostPersist implements the Contract interface.
func (s *Std) PostPersist(ic *interop.Context) error {
	return nil
}

// ActiveIn implements the Contract interface.
func (s *Std) ActiveIn() *config.Hardfork {
	return nil
}

func toLimitedBytes(item stackitem.Item) []byte {
	src := toBytes(item)
	if len(src) > stdMaxInputLength {
		panic(ErrTooBigInput)
	}
	return src
}

func toBytes(item stackitem.Item) []byte {
	src, err := item.TryBytes()
	if err != nil {
		panic(err)
	}
	return src
}

func toLimitedString(item stackitem.Item) string {
	src := toString(item)
	if len(src) > stdMaxInputLength {
		panic(ErrTooBigInput)
	}
	return src
}

// stripSpaces removes all whitespace characters and tabulation characters from
// string, ref. https://learn.microsoft.com/ru-ru/dotnet/api/system.convert.frombase64string?view=net-8.0#remarks.
func stripSpaces(str string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t':
			return -1
		default:
			return r
		}
	}, str)
}
