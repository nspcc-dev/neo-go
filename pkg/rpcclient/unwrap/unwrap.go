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

// Exception is a type used for VM fault messages (aka exceptions). If any of
// unwrapper functions encounters a FAULT VM state it creates an instance of
// this type as an error using exception string. It can be used with [errors.As]
// to get the exact message from VM and compare with known contract-specific
// errors.
type Exception string

// ErrNoSessionID is returned from the SessionIterator when the server does not
// have sessions enabled and does not perform automatic iterator expansion. It
// means you have no way to get the data from returned iterators using this
// server, other than expanding it in the VM script.
var ErrNoSessionID = errors.New("server returned iterator ID, but no session ID")

// ErrNull is returned when Null item is present on the stack instead of the
// expected one (like integer, array, string or whatever else contract is
// supposed to return). The semantics of this error is entirely
// contract-specific, in most cases it's normal contract behavior which means
// that contract has no value to return. In some cases though returning Null is
// not expected and it's a violation of API from the contract side. Since many
// types returned from this package methods can't be nil this error lets package
// user to differentiate between Null and zero value (like an empty string)
// returned and handle both cases in a way appropriate for a particular
// contract/backend interaction.
var ErrNull = errors.New("Null result")

// Error implements the error interface.
func (e Exception) Error() string {
	return string(e)
}

// BigInt expects correct execution (HALT state) with a single stack item
// returned. A big.Int is extracted from this item and returned.
func BigInt(r *result.Invoke, err error) (*big.Int, error) {
	itm, err := nonNullItem(r, err)
	if err != nil {
		return nil, err
	}
	return itm.TryInteger()
}

// Bool expects correct execution (HALT state) with a single stack item
// returned. A bool is extracted from this item and returned.
func Bool(r *result.Invoke, err error) (bool, error) {
	itm, err := nonNullItem(r, err)
	if err != nil {
		return false, err
	}
	return itm.TryBool()
}

// Int64 expects correct execution (HALT state) with a single stack item
// returned. An int64 is extracted from this item and returned.
func Int64(r *result.Invoke, err error) (int64, error) {
	itm, err := nonNullItem(r, err)
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
func LimitedInt64(r *result.Invoke, err error, minI int64, maxI int64) (int64, error) {
	i, err := Int64(r, err)
	if err != nil {
		return 0, err
	}
	if i < minI {
		return 0, errors.New("too small value")
	}
	if i > maxI {
		return 0, errors.New("too big value")
	}
	return i, nil
}

// Bytes expects correct execution (HALT state) with a single stack item
// returned. A slice of bytes is extracted from this item and returned.
func Bytes(r *result.Invoke, err error) ([]byte, error) {
	itm, err := nonNullItem(r, err)
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

// PublicKey expects correct execution (HALT state) with a single stack item
// returned. A public key is extracted from this item and returned.
func PublicKey(r *result.Invoke, err error) (*keys.PublicKey, error) {
	b, err := Bytes(r, err)
	if err != nil {
		return nil, err
	}
	return keys.NewPublicKeyFromBytes(b, elliptic.P256())
}

// SessionIterator expects correct execution (HALT state) with a single stack
// item returned. If this item is an iterator it's returned to the caller along
// with the session ID. Notice that this function also returns successfully
// with zero session ID (but an appropriate Iterator holding all the data
// received) when RPC server performs (limited) iterator expansion which is the
// default behavior for NeoGo servers with SessionEnabled set to false.
func SessionIterator(r *result.Invoke, err error) (uuid.UUID, result.Iterator, error) {
	itm, err := nonNullItem(r, err)
	if err != nil {
		return uuid.UUID{}, result.Iterator{}, err
	}
	iter, err := itemToSessionIterator(itm)
	if err != nil {
		return uuid.UUID{}, result.Iterator{}, err
	}
	if (r.Session == uuid.UUID{}) && iter.ID != nil {
		return uuid.UUID{}, result.Iterator{}, ErrNoSessionID
	}
	return r.Session, iter, nil
}

// ArrayAndSessionIterator expects correct execution (HALT state) with one or two stack
// items returned. If there is 1 item, it must be an array. If there is a second item,
// it must be an iterator. This is exactly the result of smartcontract.CreateCallAndPrefetchIteratorScript.
// Sessions must be enabled on the RPC server for this to function correctly.
func ArrayAndSessionIterator(r *result.Invoke, err error) ([]stackitem.Item, uuid.UUID, result.Iterator, error) {
	if err := checkResOK(r, err); err != nil {
		return nil, uuid.UUID{}, result.Iterator{}, err
	}
	if len(r.Stack) == 0 {
		return nil, uuid.UUID{}, result.Iterator{}, errors.New("result stack is empty")
	}
	if len(r.Stack) != 1 && len(r.Stack) != 2 {
		return nil, uuid.UUID{}, result.Iterator{}, fmt.Errorf("expected 1 or 2 result items, got %d", len(r.Stack))
	}

	// Unwrap array.
	itm := r.Stack[0]
	arr, ok := itm.Value().([]stackitem.Item)
	if !ok {
		if itm.Type() == stackitem.AnyT {
			return nil, uuid.UUID{}, result.Iterator{}, ErrNull
		}
		return nil, uuid.UUID{}, result.Iterator{}, errors.New("not an array")
	}

	// Check whether iterator exists and unwrap it.
	if len(r.Stack) == 1 {
		return arr, uuid.UUID{}, result.Iterator{}, nil
	}

	iter, err := itemToSessionIterator(r.Stack[1])
	if err != nil {
		return nil, uuid.UUID{}, result.Iterator{}, err
	}
	if (r.Session == uuid.UUID{}) {
		return nil, uuid.UUID{}, result.Iterator{}, ErrNoSessionID
	}
	return arr, r.Session, iter, nil
}

func itemToSessionIterator(itm stackitem.Item) (result.Iterator, error) {
	if t := itm.Type(); t != stackitem.InteropT {
		return result.Iterator{}, fmt.Errorf("expected InteropInterface, got %s", t)
	}
	iter, ok := itm.Value().(result.Iterator)
	if !ok {
		return result.Iterator{}, errors.New("the item is InteropInterface, but not an Iterator")
	}
	return iter, nil
}

// Array expects correct execution (HALT state) with a single array stack item
// returned. This item is returned to the caller. Notice that this function can
// be used for structures as well since they're also represented as slices of
// stack items (the number of them and their types are structure-specific).
func Array(r *result.Invoke, err error) ([]stackitem.Item, error) {
	itm, err := nonNullItem(r, err)
	if err != nil {
		return nil, err
	}
	arr, ok := itm.Value().([]stackitem.Item)
	if !ok {
		return nil, errors.New("not an array")
	}
	return arr, nil
}

// ArrayOfBools checks the result for correct state (HALT) and then extracts a
// slice of boolean values from the returned stack item.
func ArrayOfBools(r *result.Invoke, err error) ([]bool, error) {
	a, err := Array(r, err)
	if err != nil {
		return nil, err
	}
	res := make([]bool, len(a))
	for i := range a {
		b, err := a[i].TryBool()
		if err != nil {
			return nil, fmt.Errorf("element %d is not a boolean: %w", i, err)
		}
		res[i] = b
	}
	return res, nil
}

// ArrayOfBigInts checks the result for correct state (HALT) and then extracts a
// slice of (big) integer values from the returned stack item.
func ArrayOfBigInts(r *result.Invoke, err error) ([]*big.Int, error) {
	a, err := Array(r, err)
	if err != nil {
		return nil, err
	}
	res := make([]*big.Int, len(a))
	for i := range a {
		v, err := a[i].TryInteger()
		if err != nil {
			return nil, fmt.Errorf("element %d is not an integer: %w", i, err)
		}
		res[i] = v
	}
	return res, nil
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

// ArrayOfUTB8Strings checks the result for correct state (HALT) and then extracts a
// slice of UTF-8 strings from the returned stack item.
func ArrayOfUTF8Strings(r *result.Invoke, err error) ([]string, error) {
	a, err := Array(r, err)
	if err != nil {
		return nil, err
	}
	res := make([]string, len(a))
	for i := range a {
		b, err := a[i].TryBytes()
		if err != nil {
			return nil, fmt.Errorf("element %d is not a byte string: %w", i, err)
		}
		if !utf8.Valid(b) {
			return nil, fmt.Errorf("element %d is not a UTF-8 string", i)
		}
		res[i] = string(b)
	}
	return res, nil
}

// ArrayOfUint160 checks the result for correct state (HALT) and then extracts a
// slice of util.Uint160 from the returned stack item.
func ArrayOfUint160(r *result.Invoke, err error) ([]util.Uint160, error) {
	a, err := Array(r, err)
	if err != nil {
		return nil, err
	}
	res := make([]util.Uint160, len(a))
	for i := range a {
		b, err := a[i].TryBytes()
		if err != nil {
			return nil, fmt.Errorf("element %d is not a byte string: %w", i, err)
		}
		u, err := util.Uint160DecodeBytesBE(b)
		if err != nil {
			return nil, fmt.Errorf("element %d is not a uint160: %w", i, err)
		}
		res[i] = u
	}
	return res, nil
}

// ArrayOfUint256 checks the result for correct state (HALT) and then extracts a
// slice of util.Uint256 from the returned stack item.
func ArrayOfUint256(r *result.Invoke, err error) ([]util.Uint256, error) {
	a, err := Array(r, err)
	if err != nil {
		return nil, err
	}
	res := make([]util.Uint256, len(a))
	for i := range a {
		b, err := a[i].TryBytes()
		if err != nil {
			return nil, fmt.Errorf("element %d is not a byte string: %w", i, err)
		}
		u, err := util.Uint256DecodeBytesBE(b)
		if err != nil {
			return nil, fmt.Errorf("element %d is not a uint256: %w", i, err)
		}
		res[i] = u
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
	itm, err := nonNullItem(r, err)
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
		return fmt.Errorf("invocation failed: %w", Exception(r.FaultException))
	}
	if r.FaultException != "" {
		return fmt.Errorf("inconsistent result, HALTed with exception: %w", Exception(r.FaultException))
	}
	return nil
}

// Item returns a stack item from the result if execution was successful (HALT
// state) and if it's the only element on the result stack. It returns items
// as is, so it never returns [ErrNull], a proper [stackitem.Null] is returned
// in this case.
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

// nonNullItem is similar to Item, but returns ErrNull if the item is Null.
func nonNullItem(r *result.Invoke, err error) (stackitem.Item, error) {
	itm, err := Item(r, err)
	if err != nil {
		return nil, err
	}
	if itm.Type() == stackitem.AnyT {
		return nil, ErrNull
	}
	return itm, err
}

// Nothing expects zero stack items and a successful invocation (HALT state).
func Nothing(r *result.Invoke, err error) error {
	err = checkResOK(r, err)
	if err != nil {
		return err
	}
	if len(r.Stack) != 0 {
		return errors.New("result stack is not empty")
	}
	return nil
}
