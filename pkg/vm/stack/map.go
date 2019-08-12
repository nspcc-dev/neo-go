package stack

import (
	"errors"
	"fmt"
	"sort"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
)

// Map represents a map of key, value pair on the stack.
// Both key and value are stack Items.
type Map struct {
	*abstractItem
	val map[Item]Item
}

// NewMap returns a Map stack Item given
// a map whose keys and values are stack Items.
func NewMap(val map[Item]Item) (*Map, error) {
	return &Map{
		abstractItem: &abstractItem{},
		val:          val,
	}, nil
}

// Map will overwrite the default implementation
// to allow go to cast this item as an Map.
func (m *Map) Map() (*Map, error) {
	return m, nil
}

// Boolean overrides the default Boolean method
// to convert an Map into a Boolean StackItem
func (m *Map) Boolean() (*Boolean, error) {
	return NewBoolean(true), nil
}

// ContainsKey returns a boolean whose value is true
// iff the underlying map value contains the Item i
// as a key.
func (m *Map) ContainsKey(key Item) (*Boolean, error) {
	for k := range m.Value() {
		if ok, err := CompareHash(k, key); err != nil {
			return nil, err
		} else if ok.Value() == true {
			return ok, nil
		}

	}
	return NewBoolean(false), nil
}

// Value returns the underlying map's value
func (m *Map) Value() map[Item]Item {
	return m.val
}

// Remove removes the Item i from the
// underlying map's value.
func (m *Map) Remove(key Item) error {
	var d Item
	for k := range m.Value() {
		if ok, err := CompareHash(k, key); err != nil {
			return err
		} else if ok.Value() == true {
			d = k
		}

	}
	if d != nil {
		delete(m.Value(), d)
	}
	return nil
}

// Add inserts a new key, value pair of Items into
// the underlying map's value.
func (m *Map) Add(key Item, value Item) error {
	for k := range m.Value() {
		if ok, err := CompareHash(k, key); err != nil {
			return err
		} else if ok.Value() == true {
			return errors.New("try to insert duplicate key! ")
		}
	}
	m.Value()[key] = value
	return nil
}

// ValueOfKey tries to get the value of the key Item
// from the map's underlying value.
func (m *Map) ValueOfKey(key Item) (Item, error) {
	for k, v := range m.Value() {
		if ok, err := CompareHash(k, key); err != nil {
			return nil, err
		} else if ok.Value() == true {
			return v, nil
		}

	}
	return nil, nil

}

// Clear empties the the underlying map's value.
func (m *Map) Clear() {
	m.val = map[Item]Item{}
}

// CompareHash compare the the Hashes of two items.
// If they are equal it returns a true boolean. Otherwise
// it returns  false boolean. Item whose hashes are equal are
// to be considered equal.
func CompareHash(i1 Item, i2 Item) (*Boolean, error) {
	hash1, err := i1.Hash()
	if err != nil {
		return nil, err
	}
	hash2, err := i2.Hash()
	if err != nil {
		return nil, err
	}
	if hash1 == hash2 {
		return NewBoolean(true), nil
	}

	return NewBoolean(false), nil
}

// Hash overrides the default abstract hash method.
func (m *Map) Hash() (string, error) {
	var hashSlice sort.StringSlice = []string{}
	var data = fmt.Sprintf("%T ", m)

	for k, v := range m.Value() {
		hk, err := k.Hash()
		if err != nil {
			return "", err
		}
		hv, err := v.Hash()

		if err != nil {
			return "", err
		}

		hashSlice = append(hashSlice, hk)
		hashSlice = append(hashSlice, hv)
	}
	hashSlice.Sort()

	for _, h := range hashSlice {
		data += h
	}

	return KeyGenerator([]byte(data))
}

// KeyGenerator hashes a byte slice to obtain a unique identifier.
func KeyGenerator(data []byte) (string, error) {
	h, err := hash.Sha256([]byte(data))
	if err != nil {
		return "", err
	}
	return h.String(), nil
}
