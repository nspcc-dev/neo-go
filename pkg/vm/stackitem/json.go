package stackitem

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	gio "io"
	"math/big"
	"strconv"
	"strings"

	"github.com/holiman/uint256"
)

// decoder is a wrapper around json.Decoder helping to mimic C# json decoder behavior.
type decoder struct {
	json.Decoder

	count int
	depth int
}

// MaxAllowedInteger is the maximum integer allowed to be encoded.
const MaxAllowedInteger = 2<<53 - 1

// MaxJSONDepth is the maximum allowed nesting level of an encoded/decoded JSON.
const MaxJSONDepth = 10

// ErrInvalidValue is returned when an item value doesn't fit some constraints
// during serialization or deserialization.
var ErrInvalidValue = errors.New("invalid value")

// ErrTooDeep is returned when JSON encoder/decoder goes beyond MaxJSONDepth in
// its processing.
var ErrTooDeep = errors.New("too deep")

// ToJSON encodes Item to JSON.
// It behaves as following:
//
//	ByteArray -> base64 string
//	BigInteger -> number
//	Bool -> bool
//	Null -> null
//	Array, Struct -> array
//	Map -> map with keys as UTF-8 bytes
func ToJSON(item Item) ([]byte, error) {
	seen := make(map[Item]sliceNoPointer, typicalNumOfItems)
	return toJSON(nil, seen, item)
}

// sliceNoPointer represents a sub-slice of a known slice.
// It doesn't contain any pointer and uses less memory than `[]byte`.
type sliceNoPointer struct {
	start, end int
}

func toJSON(data []byte, seen map[Item]sliceNoPointer, item Item) ([]byte, error) {
	if len(data) > MaxSize {
		return nil, errTooBigSize
	}

	if old, ok := seen[item]; ok {
		if len(data)+old.end-old.start > MaxSize {
			return nil, errTooBigSize
		}
		return append(data, data[old.start:old.end]...), nil
	}

	start := len(data)
	var err error

	switch it := item.(type) {
	case *Array, *Struct:
		var items []Item
		if a, ok := it.(*Array); ok {
			items = a.value
		} else {
			items = it.(*Struct).value
		}

		data = append(data, '[')
		for i, v := range items {
			data, err = toJSON(data, seen, v)
			if err != nil {
				return nil, err
			}
			if i < len(items)-1 {
				data = append(data, ',')
			}
		}
		data = append(data, ']')
		seen[item] = sliceNoPointer{start, len(data)}
	case *Map:
		data = append(data, '{')
		for i := range it.value {
			// map key can always be converted to []byte
			// but are not always a valid UTF-8.
			raw, err := itemToJSONString(it.value[i].Key)
			if err != nil {
				return nil, err
			}
			data = append(data, raw...)
			data = append(data, ':')
			data, err = toJSON(data, seen, it.value[i].Value)
			if err != nil {
				return nil, err
			}
			if i < len(it.value)-1 {
				data = append(data, ',')
			}
		}
		data = append(data, '}')
		seen[item] = sliceNoPointer{start, len(data)}
	case *BigInteger:
		if uint256.NewInt(0).Abs(it.Big()).Cmp(uint256.NewInt(MaxAllowedInteger)) == 1 {
			return nil, fmt.Errorf("%w (MaxAllowedInteger)", ErrInvalidValue)
		}
		data = append(data, it.Big().String()...)
	case *ByteArray, *Buffer:
		raw, err := itemToJSONString(it)
		if err != nil {
			return nil, err
		}
		data = append(data, raw...)
	case Bool:
		if it {
			data = append(data, "true"...)
		} else {
			data = append(data, "false"...)
		}
	case Null:
		data = append(data, "null"...)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnserializable, it.String())
	}
	if len(data) > MaxSize {
		return nil, errTooBigSize
	}
	return data, nil
}

// itemToJSONString converts it to a string
// in quotation marks with control characters escaped.
func itemToJSONString(it Item) ([]byte, error) {
	s, err := ToString(it)
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(s) // error never occurs because `ToString` checks for validity

	// ref https://github.com/neo-project/neo-modules/issues/375 and https://github.com/dotnet/runtime/issues/35281
	return bytes.Replace(data, []byte{'+'}, []byte("\\u002B"), -1), nil
}

// FromJSON decodes an Item from JSON.
// It behaves as following:
//
//	string -> ByteArray from base64
//	number -> BigInteger
//	bool -> Bool
//	null -> Null
//	array -> Array
//	map -> Map, keys are UTF-8
func FromJSON(data []byte, maxCount int) (Item, error) {
	d := decoder{
		Decoder: *json.NewDecoder(bytes.NewReader(data)),
		count:   maxCount,
	}
	d.UseNumber()
	if item, err := d.decode(); err != nil {
		return nil, err
	} else if _, err := d.Token(); !errors.Is(err, gio.EOF) {
		return nil, fmt.Errorf("%w: unexpected items", ErrInvalidValue)
	} else {
		return item, nil
	}
}

func (d *decoder) decode() (Item, error) {
	tok, err := d.Token()
	if err != nil {
		return nil, err
	}

	d.count--
	if d.count < 0 && tok != json.Delim('}') && tok != json.Delim(']') {
		return nil, errTooBigElements
	}

	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case json.Delim('{'), json.Delim('['):
			if d.depth == MaxJSONDepth {
				return nil, ErrTooDeep
			}
			d.depth++
			var item Item
			if t == json.Delim('{') {
				item, err = d.decodeMap()
			} else {
				item, err = d.decodeArray()
			}
			d.depth--
			return item, err
		default:
			d.count++
			// no error above means corresponding closing token
			// was encountered for map or array respectively
			return nil, nil
		}
	case string:
		return NewByteArray([]byte(t)), nil
	case json.Number:
		ts := t.String()
		dot := strings.IndexByte(ts, '.')
		if dot != -1 {
			// As a special case numbers like 123.000 are allowed (SetString rejects them).
			// And yes, that's the way C# code works also.
			for _, r := range ts[dot+1:] {
				if r != '0' {
					return nil, fmt.Errorf("%w (real value for int)", ErrInvalidValue)
				}
			}
			ts = ts[:dot]
		}
		num, ok := new(big.Int).SetString(ts, 10)
		if !ok {
			return nil, fmt.Errorf("%w (integer)", ErrInvalidValue)
		}
		return NewBigIntegerFromBig(num), nil
	case bool:
		return NewBool(t), nil
	default:
		// it can be only `nil`
		return Null{}, nil
	}
}

func (d *decoder) decodeArray() (*Array, error) {
	items := []Item{}
	for {
		item, err := d.decode()
		if err != nil {
			return nil, err
		}
		if item == nil {
			return NewArray(items), nil
		}
		items = append(items, item)
	}
}

func (d *decoder) decodeMap() (*Map, error) {
	m := NewMap()
	for {
		key, err := d.Token()
		if err != nil {
			return nil, err
		}
		k, ok := key.(string)
		if !ok {
			return m, nil
		}

		d.count--
		if d.count < 0 {
			return nil, errTooBigElements
		}
		val, err := d.decode()
		if err != nil {
			return nil, err
		}
		m.Add(NewByteArray([]byte(k)), val)
	}
}

// ToJSONWithTypes serializes any stackitem to JSON in a lossless way.
func ToJSONWithTypes(item Item) ([]byte, error) {
	return toJSONWithTypes(nil, item, make(map[Item]sliceNoPointer, typicalNumOfItems))
}

func toJSONWithTypes(data []byte, item Item, seen map[Item]sliceNoPointer) ([]byte, error) {
	if item == nil {
		return nil, fmt.Errorf("%w: nil", ErrUnserializable)
	}
	if old, ok := seen[item]; ok {
		if old.end == 0 {
			// Compound item marshaling which has not yet finished.
			return nil, ErrRecursive
		}
		if len(data)+old.end-old.start > MaxSize {
			return nil, errTooBigSize
		}
		return append(data, data[old.start:old.end]...), nil
	}

	var val string
	var hasValue bool
	switch item.(type) {
	case Null:
		val = `{"type":"Any"}`
	case *Interop:
		val = `{"type":"InteropInterface"}`
	default:
		val = `{"type":"` + item.Type().String() + `","value":`
		hasValue = true
	}

	if len(data)+len(val) > MaxSize {
		return nil, errTooBigSize
	}

	start := len(data)

	data = append(data, val...)
	if !hasValue {
		return data, nil
	}

	// Primitive stack items are appended after the switch
	// to reduce the amount of size checks.
	var primitive string
	var isBuffer bool
	var err error

	switch it := item.(type) {
	case *Array, *Struct:
		seen[item] = sliceNoPointer{}
		data = append(data, '[')
		for i, elem := range it.Value().([]Item) {
			if i != 0 {
				data = append(data, ',')
			}
			data, err = toJSONWithTypes(data, elem, seen)
			if err != nil {
				return nil, err
			}
		}
	case Bool:
		if it {
			primitive = "true"
		} else {
			primitive = "false"
		}
	case *ByteArray:
		primitive = `"` + base64.StdEncoding.EncodeToString(it.Value().([]byte)) + `"`
	case *Buffer:
		isBuffer = true
		primitive = `"` + base64.StdEncoding.EncodeToString(it.Value().([]byte)) + `"`
	case *BigInteger:
		primitive = `"` + it.Big().String() + `"`
	case *Map:
		seen[item] = sliceNoPointer{}
		data = append(data, '[')
		for i := range it.value {
			if i != 0 {
				data = append(data, ',')
			}
			data = append(data, `{"key":`...)
			data, err = toJSONWithTypes(data, it.value[i].Key, seen)
			if err != nil {
				return nil, err
			}
			data = append(data, `,"value":`...)
			data, err = toJSONWithTypes(data, it.value[i].Value, seen)
			if err != nil {
				return nil, err
			}
			data = append(data, '}')
		}
	case *Pointer:
		primitive = strconv.Itoa(it.pos)
	}
	if len(primitive) != 0 {
		if len(data)+len(primitive)+1 > MaxSize {
			return nil, errTooBigSize
		}
		data = append(data, primitive...)
		data = append(data, '}')

		if isBuffer {
			seen[item] = sliceNoPointer{start, len(data)}
		}
	} else {
		if len(data)+2 > MaxSize { // also take care of '}'
			return nil, errTooBigSize
		}
		data = append(data, ']', '}')

		seen[item] = sliceNoPointer{start, len(data)}
	}
	return data, nil
}

type (
	rawItem struct {
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value,omitempty"`
	}

	rawMapElement struct {
		Key   json.RawMessage `json:"key"`
		Value json.RawMessage `json:"value"`
	}
)

func mkErrValue(err error) error {
	return fmt.Errorf("%w: %v", ErrInvalidValue, err) //nolint:errorlint // errorlint: non-wrapping format verb for fmt.Errorf. Use `%w` to format errors
}

// FromJSONWithTypes deserializes an item from typed-json representation.
func FromJSONWithTypes(data []byte) (Item, error) {
	raw := new(rawItem)
	if err := json.Unmarshal(data, raw); err != nil {
		return nil, err
	}
	typ, err := FromString(raw.Type)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidType, raw.Type)
	}
	switch typ {
	case AnyT:
		return Null{}, nil
	case PointerT:
		var pos int
		if err := json.Unmarshal(raw.Value, &pos); err != nil {
			return nil, mkErrValue(err)
		}
		return NewPointer(pos, nil), nil
	case BooleanT:
		var b bool
		if err := json.Unmarshal(raw.Value, &b); err != nil {
			return nil, mkErrValue(err)
		}
		return NewBool(b), nil
	case IntegerT:
		var s string
		if err := json.Unmarshal(raw.Value, &s); err != nil {
			return nil, mkErrValue(err)
		}
		val, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, mkErrValue(errors.New("not an integer"))
		}
		return NewBigIntegerFromBig(val), nil
	case ByteArrayT, BufferT:
		var s string
		if err := json.Unmarshal(raw.Value, &s); err != nil {
			return nil, mkErrValue(err)
		}
		val, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, mkErrValue(err)
		}
		if typ == ByteArrayT {
			return NewByteArray(val), nil
		}
		return NewBuffer(val), nil
	case ArrayT, StructT:
		var arr []json.RawMessage
		if err := json.Unmarshal(raw.Value, &arr); err != nil {
			return nil, mkErrValue(err)
		}
		items := make([]Item, len(arr))
		for i := range arr {
			it, err := FromJSONWithTypes(arr[i])
			if err != nil {
				return nil, err
			}
			items[i] = it
		}
		if typ == ArrayT {
			return NewArray(items), nil
		}
		return NewStruct(items), nil
	case MapT:
		var arr []rawMapElement
		if err := json.Unmarshal(raw.Value, &arr); err != nil {
			return nil, mkErrValue(err)
		}
		m := NewMap()
		for i := range arr {
			key, err := FromJSONWithTypes(arr[i].Key)
			if err != nil {
				return nil, err
			} else if err = IsValidMapKey(key); err != nil {
				return nil, err
			}
			value, err := FromJSONWithTypes(arr[i].Value)
			if err != nil {
				return nil, err
			}
			m.Add(key, value)
		}
		return m, nil
	case InteropT:
		return NewInterop(nil), nil
	default:
		return nil, fmt.Errorf("%w: %v", ErrInvalidType, typ)
	}
}
