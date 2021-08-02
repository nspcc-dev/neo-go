package stackitem

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	gio "io"
	"math"
	"math/big"
)

// decoder is a wrapper around json.Decoder helping to mimic C# json decoder behaviour.
type decoder struct {
	json.Decoder

	count int
	depth int
}

// MaxAllowedInteger is the maximum integer allowed to be encoded.
const MaxAllowedInteger = 2<<53 - 1

// MaxJSONDepth is the maximum allowed nesting level of encoded/decoded JSON.
const MaxJSONDepth = 10

// ErrInvalidValue is returned when item value doesn't fit some constraints
// during serialization or deserialization.
var ErrInvalidValue = errors.New("invalid value")

// ErrTooDeep is returned when JSON encoder/decoder goes beyond MaxJSONDepth in
// its processing.
var ErrTooDeep = errors.New("too deep")

// ToJSON encodes Item to JSON.
// It behaves as following:
//   ByteArray -> base64 string
//   BigInteger -> number
//   Bool -> bool
//   Null -> null
//   Array, Struct -> array
//   Map -> map with keys as UTF-8 bytes
func ToJSON(item Item) ([]byte, error) {
	seen := make(map[Item]sliceNoPointer)
	return toJSON(nil, seen, item)
}

// sliceNoPointer represents sub-slice of a known slice.
// It doesn't contain pointer and uses less memory than `[]byte`.
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
		if it.value.CmpAbs(big.NewInt(MaxAllowedInteger)) == 1 {
			return nil, fmt.Errorf("%w (MaxAllowedInteger)", ErrInvalidValue)
		}
		data = append(data, it.value.String()...)
	case *ByteArray, *Buffer:
		raw, err := itemToJSONString(it)
		if err != nil {
			return nil, err
		}
		data = append(data, raw...)
	case *Bool:
		if it.value {
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

// itemToJSONString converts it to string
// surrounded in quotes with control characters escaped.
func itemToJSONString(it Item) ([]byte, error) {
	s, err := ToString(it)
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(s) // error never occurs because `ToString` checks for validity

	// ref https://github.com/neo-project/neo-modules/issues/375 and https://github.com/dotnet/runtime/issues/35281
	return bytes.Replace(data, []byte{'+'}, []byte("\\u002B"), -1), nil
}

// FromJSON decodes Item from JSON.
// It behaves as following:
//   string -> ByteArray from base64
//   number -> BigInteger
//   bool -> Bool
//   null -> Null
//   array -> Array
//   map -> Map, keys are UTF-8
func FromJSON(data []byte, maxCount int) (Item, error) {
	d := decoder{
		Decoder: *json.NewDecoder(bytes.NewReader(data)),
		count:   maxCount,
	}
	if item, err := d.decode(); err != nil {
		return nil, err
	} else if _, err := d.Token(); err != gio.EOF {
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
	case float64:
		if math.Floor(t) != t {
			return nil, fmt.Errorf("%w (real value for int)", ErrInvalidValue)
		}
		return NewBigInteger(big.NewInt(int64(t))), nil
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
	result, err := toJSONWithTypes(item, make(map[Item]bool))
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func toJSONWithTypes(item Item, seen map[Item]bool) (interface{}, error) {
	if len(seen) > MaxJSONDepth {
		return "", ErrTooDeep
	}
	var value interface{}
	switch it := item.(type) {
	case *Array, *Struct:
		if seen[item] {
			return "", ErrRecursive
		}
		seen[item] = true
		arr := []interface{}{}
		for _, elem := range it.Value().([]Item) {
			s, err := toJSONWithTypes(elem, seen)
			if err != nil {
				return "", err
			}
			arr = append(arr, s)
		}
		value = arr
		delete(seen, item)
	case *Bool:
		value = it.value
	case *Buffer, *ByteArray:
		value = base64.StdEncoding.EncodeToString(it.Value().([]byte))
	case *BigInteger:
		value = it.value.String()
	case *Map:
		if seen[item] {
			return "", ErrRecursive
		}
		seen[item] = true
		arr := []interface{}{}
		for i := range it.value {
			// map keys are primitive types and can always be converted to json
			key, _ := toJSONWithTypes(it.value[i].Key, seen)
			val, err := toJSONWithTypes(it.value[i].Value, seen)
			if err != nil {
				return "", err
			}
			arr = append(arr, map[string]interface{}{
				"key":   key,
				"value": val,
			})
		}
		value = arr
		delete(seen, item)
	case *Pointer:
		value = it.pos
	case nil:
		return "", fmt.Errorf("%w: nil", ErrUnserializable)
	}
	result := map[string]interface{}{
		"type": item.Type().String(),
	}
	if value != nil {
		result["value"] = value
	}
	return result, nil
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
	return fmt.Errorf("%w: %v", ErrInvalidValue, err)
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
		return NewBigInteger(val), nil
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
