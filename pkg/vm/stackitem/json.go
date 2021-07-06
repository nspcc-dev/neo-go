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

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// decoder is a wrapper around json.Decoder helping to mimic C# json decoder behaviour.
type decoder struct {
	json.Decoder

	depth int
}

// MaxAllowedInteger is the maximum integer allowed to be encoded.
const MaxAllowedInteger = 2<<53 - 1

// maxJSONDepth is a maximum allowed depth-level of decoded JSON.
const maxJSONDepth = 10

// ToJSON encodes Item to JSON.
// It behaves as following:
//   ByteArray -> base64 string
//   BigInteger -> number
//   Bool -> bool
//   Null -> null
//   Array, Struct -> array
//   Map -> map with keys as UTF-8 bytes
func ToJSON(item Item) ([]byte, error) {
	buf := io.NewBufBinWriter()
	toJSON(buf, item)
	if buf.Err != nil {
		return nil, buf.Err
	}
	return buf.Bytes(), nil
}

func toJSON(buf *io.BufBinWriter, item Item) {
	w := buf.BinWriter
	if w.Err != nil {
		return
	} else if buf.Len() > MaxSize {
		w.Err = errors.New("item is too big")
	}
	switch it := item.(type) {
	case *Array, *Struct:
		w.WriteB('[')
		items := it.Value().([]Item)
		for i, v := range items {
			toJSON(buf, v)
			if i < len(items)-1 {
				w.WriteB(',')
			}
		}
		w.WriteB(']')
	case *Map:
		w.WriteB('{')
		for i := range it.value {
			// map key can always be converted to []byte
			// but are not always a valid UTF-8.
			writeJSONString(buf.BinWriter, it.value[i].Key)
			w.WriteBytes([]byte(`:`))
			toJSON(buf, it.value[i].Value)
			if i < len(it.value)-1 {
				w.WriteB(',')
			}
		}
		w.WriteB('}')
	case *BigInteger:
		if it.value.CmpAbs(big.NewInt(MaxAllowedInteger)) == 1 {
			w.Err = errors.New("too big integer")
			return
		}
		w.WriteBytes([]byte(it.value.String()))
	case *ByteArray, *Buffer:
		writeJSONString(w, it)
	case *Bool:
		if it.value {
			w.WriteBytes([]byte("true"))
		} else {
			w.WriteBytes([]byte("false"))
		}
	case Null:
		w.WriteBytes([]byte("null"))
	default:
		w.Err = fmt.Errorf("invalid item: %s", it.String())
		return
	}
	if w.Err == nil && buf.Len() > MaxSize {
		w.Err = errors.New("item is too big")
	}
}

// writeJSONString converts it to string and writes it to w as JSON value
// surrounded in quotes with control characters escaped.
func writeJSONString(w *io.BinWriter, it Item) {
	if w.Err != nil {
		return
	}
	s, err := ToString(it)
	if err != nil {
		w.Err = err
		return
	}
	data, _ := json.Marshal(s) // error never occurs because `ToString` checks for validity

	// ref https://github.com/neo-project/neo-modules/issues/375 and https://github.com/dotnet/runtime/issues/35281
	data = bytes.Replace(data, []byte{'+'}, []byte("\\u002B"), -1)

	w.WriteBytes(data)
}

// FromJSON decodes Item from JSON.
// It behaves as following:
//   string -> ByteArray from base64
//   number -> BigInteger
//   bool -> Bool
//   null -> Null
//   array -> Array
//   map -> Map, keys are UTF-8
func FromJSON(data []byte) (Item, error) {
	d := decoder{Decoder: *json.NewDecoder(bytes.NewReader(data))}
	if item, err := d.decode(); err != nil {
		return nil, err
	} else if _, err := d.Token(); err != gio.EOF {
		return nil, errors.New("unexpected items")
	} else {
		return item, nil
	}
}

func (d *decoder) decode() (Item, error) {
	tok, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case json.Delim('{'), json.Delim('['):
			if d.depth == maxJSONDepth {
				return nil, errors.New("JSON depth limit exceeded")
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
			// no error above means corresponding closing token
			// was encountered for map or array respectively
			return nil, nil
		}
	case string:
		return NewByteArray([]byte(t)), nil
	case float64:
		if math.Floor(t) != t {
			return nil, fmt.Errorf("real value is not allowed: %v", t)
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
	if len(seen) > maxJSONDepth {
		return "", errors.New("too deep structure")
	}
	typ := item.Type()
	result := map[string]interface{}{
		"type": typ.String(),
	}
	var value interface{}
	switch it := item.(type) {
	case *Array, *Struct:
		if seen[item] {
			return "", errors.New("recursive structures can't be serialized to json")
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
			return "", errors.New("recursive structures can't be serialized to json")
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

// FromJSONWithTypes deserializes an item from typed-json representation.
func FromJSONWithTypes(data []byte) (Item, error) {
	raw := new(rawItem)
	if err := json.Unmarshal(data, raw); err != nil {
		return nil, err
	}
	typ, err := FromString(raw.Type)
	if err != nil {
		return nil, errors.New("invalid type")
	}
	switch typ {
	case AnyT:
		return Null{}, nil
	case PointerT:
		var pos int
		if err := json.Unmarshal(raw.Value, &pos); err != nil {
			return nil, err
		}
		return NewPointer(pos, nil), nil
	case BooleanT:
		var b bool
		if err := json.Unmarshal(raw.Value, &b); err != nil {
			return nil, err
		}
		return NewBool(b), nil
	case IntegerT:
		var s string
		if err := json.Unmarshal(raw.Value, &s); err != nil {
			return nil, err
		}
		val, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, errors.New("invalid integer")
		}
		return NewBigInteger(val), nil
	case ByteArrayT, BufferT:
		var s string
		if err := json.Unmarshal(raw.Value, &s); err != nil {
			return nil, err
		}
		val, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, err
		}
		if typ == ByteArrayT {
			return NewByteArray(val), nil
		}
		return NewBuffer(val), nil
	case ArrayT, StructT:
		var arr []json.RawMessage
		if err := json.Unmarshal(raw.Value, &arr); err != nil {
			return nil, err
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
			return nil, err
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
		return nil, errors.New("unexpected type")
	}
}
