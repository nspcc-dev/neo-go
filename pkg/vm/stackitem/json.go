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
			key, err := ToString(it.value[i].Key)
			if err != nil {
				if buf.Err == nil {
					buf.Err = err
				}
				return
			}
			w.WriteB('"')
			w.WriteBytes([]byte(key))
			w.WriteBytes([]byte(`":`))
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
	case *ByteArray:
		w.WriteB('"')
		val := it.Value().([]byte)
		b := make([]byte, base64.StdEncoding.EncodedLen(len(val)))
		base64.StdEncoding.Encode(b, val)
		w.WriteBytes(b)
		w.WriteB('"')
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
		b, err := base64.StdEncoding.DecodeString(t)
		if err != nil {
			return nil, err
		}
		return NewByteArray(b), nil
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
