/*
Package unwrap provides a set of proxy methods to process invocation results.

Functions implemented there are intended to be used as wrappers for other
functions that return (*result.Invoke, error) pair (of which there are many).
These functions will check for error, check for VM state, check the number
of results, cast them to appropriate type (if everything is OK) and then
return a result or error. They're mostly useful for other higher-level
contract-specific packages.
*/
package unwrap

import (
	"crypto/elliptic"
	"errors"
	"fmt"
	"math/big"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
)

// BigInt expects correct execution (HALT state) with a single stack item
// returned. A big.Int is extracted from this item and returned.
func BigInt(r *result.Invoke, err error) (*big.Int, error) {
	itm, err := Item(r, err)
	if err != nil {
		return nil, err
	}
	return itm.TryInteger()
}

// Bool expects correct execution (HALT state) with a single stack item
// returned. A bool is extracted from this item and returned.
func Bool(r *result.Invoke, err error) (bool, error) {
	itm, err := Item(r, err)
	if err != nil {
		return false, err
	}
	return itm.TryBool()
}

// Int64 expects correct execution (HALT state) with a single stack item
// returned. An int64 is extracted from this item and returned.
func Int64(r *result.Invoke, err error) (int64, error) {
	itm, err := Item(r, err)
	if err != nil {
		return 0, err
	}
	i, err := itm.TryInteger()
	if err != nil {
		return 0, err
	}
	if !i.IsInt64() {
		return 0, errors.New("int64 overflow")
	}
	return i.Int64(), nil
}

// LimitedInt64 is similar to Int64 except it allows to set minimum and maximum
// limits to be checked, so if it doesn't return an error the value is more than
// min and less than max.
func LimitedInt64(r *result.Invoke, err error, min int64, max int64) (int64, error) {
	i, err := Int64(r, err)
	if err != nil {
		return 0, err
	}
	if i < min {
		return 0, errors.New("too small value")
	}
	if i > max {
		return 0, errors.New("too big value")
	}
	return i, nil
}

// Bytes expects correct execution (HALT state) with a single stack item
// returned. A slice of bytes is extracted from this item and returned.
func Bytes(r *result.Invoke, err error) ([]byte, error) {
	itm, err := Item(r, err)
	if err != nil {
		return nil, err
	}
	return itm.TryBytes()
}

// UTF8String expects correct execution (HALT state) with a single stack item
// returned. A string is extracted from this item and checked for UTF-8
// correctness, valid strings are then returned.
func UTF8String(r *result.Invoke, err error) (string, error) {
	b, err := Bytes(r, err)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(b) {
		return "", errors.New("not a UTF-8 string")
	}
	return string(b), nil
}

// PrintableASCIIString expects correct execution (HALT state) with a single
// stack item returned. A string is extracted from this item and checked to
// only contain ASCII characters in printable range, valid strings are then
// returned.
func PrintableASCIIString(r *result.Invoke, err error) (string, error) {
	s, err := UTF8String(r, err)
	if err != nil {
		return "", err
	}
	for _, c := range s {
		if c < 32 || c >= 127 {
			return "", errors.New("not a printable ASCII string")
		}
	}
	return s, nil
}

// Uint160 expects correct execution (HALT state) with a single stack item
// returned. An util.Uint160 is extracted from this item and returned.
func Uint160(r *result.Invoke, err error) (util.Uint160, error) {
	b, err := Bytes(r, err)
	if err != nil {
		return util.Uint160{}, err
	}
	return util.Uint160DecodeBytesBE(b)
}

// Uint256 expects correct execution (HALT state) with a single stack item
// returned. An util.Uint256 is extracted from this item and returned.
func Uint256(r *result.Invoke, err error) (util.Uint256, error) {
	b, err := Bytes(r, err)
	if err != nil {
		return util.Uint256{}, err
	}
	return util.Uint256DecodeBytesBE(b)
}

// SessionIterator expects correct execution (HALT state) with a single stack
// item returned. If this item is an iterator it's returned to the caller along
// with the session ID.
func SessionIterator(r *result.Invoke, err error) (uuid.UUID, result.Iterator, error) {
	itm, err := Item(r, err)
	if err != nil {
		return uuid.UUID{}, result.Iterator{}, err
	}
	if t := itm.Type(); t != stackitem.InteropT {
		return uuid.UUID{}, result.Iterator{}, fmt.Errorf("expected InteropInterface, got %s", t)
	}
	iter, ok := itm.Value().(result.Iterator)
	if !ok {
		return uuid.UUID{}, result.Iterator{}, errors.New("the item is InteropInterface, but not an Iterator")
	}
	if (r.Session == uuid.UUID{}) && iter.ID != nil {
		return uuid.UUID{}, result.Iterator{}, errors.New("server returned iterator ID, but no session ID")
	}
	return r.Session, iter, nil
}

// Array expects correct execution (HALT state) with a single array stack item
// returned. This item is returned to the caller. Notice that this function can
// be used for structures as well since they're also represented as slices of
// stack items (the number of them and their types are structure-specific).
func Array(r *result.Invoke, err error) ([]stackitem.Item, error) {
	itm, err := Item(r, err)
	if err != nil {
		return nil, err
	}
	arr, ok := itm.Value().([]stackitem.Item)
	if !ok {
		return nil, errors.New("not an array")
	}
	return arr, nil
}

// ArrayOfBytes checks the result for correct state (HALT) and then extracts a
// slice of byte slices from the returned stack item.
func ArrayOfBytes(r *result.Invoke, err error) ([][]byte, error) {
	a, err := Array(r, err)
	if err != nil {
		return nil, err
	}
	res := make([][]byte, len(a))
	for i := range a {
		b, err := a[i].TryBytes()
		if err != nil {
			return nil, fmt.Errorf("element %d is not a byte string: %w", i, err)
		}
		res[i] = b
	}
	return res, nil
}

// ArrayOfPublicKeys checks the result for correct state (HALT) and then
// extracts a slice of public keys from the returned stack item.
func ArrayOfPublicKeys(r *result.Invoke, err error) (keys.PublicKeys, error) {
	arr, err := Array(r, err)
	if err != nil {
		return nil, err
	}
	pks := make(keys.PublicKeys, len(arr))
	for i, item := range arr {
		val, err := item.TryBytes()
		if err != nil {
			return nil, fmt.Errorf("invalid array element #%d: %s", i, item.Type())
		}
		pks[i], err = keys.NewPublicKeyFromBytes(val, elliptic.P256())
		if err != nil {
			return nil, fmt.Errorf("array element #%d in not a key: %w", i, err)
		}
	}
	return pks, nil
}

// Map expects correct execution (HALT state) with a single stack item
// returned. A stackitem.Map is extracted from this item and returned.
func Map(r *result.Invoke, err error) (*stackitem.Map, error) {
	itm, err := Item(r, err)
	if err != nil {
		return nil, err
	}
	if t := itm.Type(); t != stackitem.MapT {
		return nil, fmt.Errorf("%s is not a map", t.String())
	}
	return itm.(*stackitem.Map), nil
}

func checkResOK(r *result.Invoke, err error) error {
	if err != nil {
		return err
	}
	if r.State != vmstate.Halt.String() {
		return fmt.Errorf("invocation failed: %s", r.FaultException)
	}
	return nil
}

// Item returns a stack item from the result if execution was successful (HALT
// state) and if it's the only element on the result stack.
func Item(r *result.Invoke, err error) (stackitem.Item, error) {
	err = checkResOK(r, err)
	if err != nil {
		return nil, err
	}
	if len(r.Stack) == 0 {
		return nil, errors.New("result stack is empty")
	}
	if len(r.Stack) > 1 {
		return nil, fmt.Errorf("too many (%d) result items", len(r.Stack))
	}
	return r.Stack[0], nil
}
