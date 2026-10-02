package runtime

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/scparser"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"go.uber.org/zap"
)

type itemable interface {
	ToStackItem() stackitem.Item
}

const (
	// MaxEventNameLen is the maximum length of a name for event.
	MaxEventNameLen = 32
	// MaxNotificationSize is the maximum length of a runtime log message.
	MaxNotificationSize = 1024
	// SystemRuntimeLogMessage represents log entry message used for output
	// of the System.Runtime.Log syscall.
	SystemRuntimeLogMessage = "runtime log"
)

// GetExecutingScriptHash returns executing script hash.
func GetExecutingScriptHash(ic *interop.Context) (*fee.InteropRunStats, error) {
	return nil, ic.VM.PushContextScriptHash(0)
}

// GetCallingScriptHash returns calling script hash.
// While Executing and Entry script hashes are always valid for non-native contracts,
// Calling hash is set explicitly when native contracts are used, because when switching from
// one native to another, no operations are performed on invocation stack.
func GetCallingScriptHash(ic *interop.Context) (*fee.InteropRunStats, error) {
	h := ic.VM.GetCallingScriptHash()
	ic.VM.Estack().PushItem(stackitem.NewByteArray(h.BytesBE()))
	return nil, nil
}

// GetEntryScriptHash returns entry script hash.
func GetEntryScriptHash(ic *interop.Context) (*fee.InteropRunStats, error) {
	return nil, ic.VM.PushContextScriptHash(len(ic.VM.Istack()) - 1)
}

// GetScriptContainer returns transaction or block that contains the script
// being run.
func GetScriptContainer(ic *interop.Context) (*fee.InteropRunStats, error) {
	c, ok := ic.Container.(itemable)
	if !ok {
		return nil, errors.New("unknown script container")
	}
	ic.VM.Estack().PushItem(c.ToStackItem())
	return nil, nil
}

// Platform returns the name of the platform.
func Platform(ic *interop.Context) (*fee.InteropRunStats, error) {
	ic.VM.Estack().PushItem(stackitem.NewByteArray([]byte("NEO")))
	return nil, nil
}

// GetTrigger returns the script trigger.
func GetTrigger(ic *interop.Context) (*fee.InteropRunStats, error) {
	ic.VM.Estack().PushItem(stackitem.NewBigInteger(big.NewInt(int64(ic.Trigger))))
	return nil, nil
}

// Notify should pass stack item to the notify plugin to handle it, but
// in neo-go the only meaningful thing to do here is to log.
func Notify(ic *interop.Context) (*fee.InteropRunStats, error) {
	name := ic.VM.Estack().Pop().String()
	elem := ic.VM.Estack().Pop()
	args := elem.Array()
	if len(name) > MaxEventNameLen {
		return nil, fmt.Errorf("event name must be less than %d", MaxEventNameLen)
	}
	curr := ic.VM.Context().GetManifest()
	if curr == nil {
		return nil, errors.New("notifications are not allowed in dynamic scripts")
	}
	var (
		ev       = curr.ABI.GetEvent(name)
		checkErr error
		curHash  = ic.VM.GetCurrentScriptHash()
	)
	if ev == nil {
		checkErr = fmt.Errorf("notification %s does not exist", name)
	} else {
		err := ev.CheckCompliance(args)
		if err != nil {
			checkErr = fmt.Errorf("notification %s is invalid: %w", name, err)
		}
	}
	if checkErr != nil {
		if ic.IsHardforkEnabled(config.HFBasilisk) {
			return nil, checkErr
		}
		ic.Log.Info("bad notification", zap.String("contract", curHash.StringLE()), zap.String("event", name), zap.Error(checkErr))
	}

	// But it has to be serializable, otherwise we either have some broken
	// (recursive) structure inside or an interop item that can't be used
	// outside of the interop subsystem anyway.
	bytes, err := ic.DAO.GetItemCtx().Serialize(elem.Item(), false)
	if err != nil {
		return nil, fmt.Errorf("bad notification: %w", err)
	}
	if len(bytes) > MaxNotificationSize {
		return nil, fmt.Errorf("notification size shouldn't exceed %d", MaxNotificationSize)
	}
	cpItem, count := deepCopy(stackitem.NewArray(args))
	return &fee.InteropRunStats{Stat: count}, ic.AddNotification(curHash, name, cpItem.(*stackitem.Array))
}

// LoadScript takes a script and arguments from the stack and loads it into the VM.
func LoadScript(ic *interop.Context) (*fee.InteropRunStats, error) {
	script := ic.VM.Estack().Pop().Bytes()
	fs := callflag.CallFlag(int32(ic.VM.Estack().Pop().BigInt().Int64()))
	if fs&^callflag.All != 0 {
		return nil, errors.New("call flags out of range")
	}
	args := ic.VM.Estack().Pop().Array()
	err := scparser.IsScriptCorrect(script, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid script: %w", err)
	}
	fs = ic.VM.Context().GetCallFlags() & callflag.ReadOnly & fs
	ic.VM.LoadDynamicScript(script, fs)

	for e, i := ic.VM.Estack(), len(args)-1; i >= 0; i-- {
		e.PushItem(args[i])
	}
	return &fee.InteropRunStats{Stat: len(script)}, nil
}

// Log logs the message passed.
func Log(ic *interop.Context) (*fee.InteropRunStats, error) {
	state := ic.VM.Estack().Pop().String()
	if len(state) > MaxNotificationSize {
		return nil, fmt.Errorf("message length shouldn't exceed %v", MaxNotificationSize)
	}
	var txHash string
	if ic.Tx != nil {
		txHash = ic.Tx.Hash().StringLE()
	}
	ic.Log.Info(SystemRuntimeLogMessage,
		zap.String("tx", txHash),
		zap.String("script", ic.VM.GetCurrentScriptHash().StringLE()),
		zap.String("msg", state))
	return nil, nil
}

// GetTime returns timestamp of the block being verified, or the latest
// one in the blockchain if no block is given to Context.
func GetTime(ic *interop.Context) (*fee.InteropRunStats, error) {
	ic.VM.Estack().PushItem(stackitem.NewBigInteger(new(big.Int).SetUint64(ic.GetTime())))
	return nil, nil
}

// BurnGas burns GAS to benefit Neo ecosystem.
func BurnGas(ic *interop.Context) (*fee.InteropRunStats, error) {
	gas := ic.VM.Estack().Pop().BigInt()
	if !gas.IsInt64() {
		return nil, errors.New("invalid GAS value")
	}

	g := gas.Int64()
	if g <= 0 {
		return nil, errors.New("GAS must be positive")
	}

	if err := ic.VM.AddDatoshi(g); err != nil {
		return nil, err
	}
	return nil, nil
}

// CurrentSigners returns signers of the currently loaded transaction or stackitem.Null
// if script container is not a transaction.
func CurrentSigners(ic *interop.Context) (*fee.InteropRunStats, error) {
	tx, ok := ic.Container.(*transaction.Transaction)
	if ok {
		ic.VM.Estack().PushItem(transaction.SignersToStackItem(tx.Signers))
		return &fee.InteropRunStats{Stat: len(tx.Signers)}, nil
	}
	ic.VM.Estack().PushItem(stackitem.Null{})
	return nil, nil
}

// deepCopy returns a new deep copy of the provided item. Array, Struct and
// Map are copied recursively along with their elements and marked as read-only.
// Bool, ByteArray, BigInteger, Pointer and Interop are returned as is since
// they're immutable. Buffer is copied into a new immutable ByteArray, and Null
// is returned as a new instance. Unsupported item types result in a nil return.
// It also returns the number of visited items including repeated references.
func deepCopy(item stackitem.Item) (stackitem.Item, int) {
	seen := make(map[stackitem.Item]stackitem.Item, 4)
	return deepCopyAux(item, seen)
}

func deepCopyAux(item stackitem.Item, seen map[stackitem.Item]stackitem.Item) (stackitem.Item, int) {
	count := 1
	if it := seen[item]; it != nil {
		return it, count
	}
	switch it := item.(type) {
	case stackitem.Null:
		return stackitem.Null{}, count
	case *stackitem.Array:
		src := it.Value().([]stackitem.Item)
		arr := stackitem.NewArray(make([]stackitem.Item, len(src)))
		seen[item] = arr
		dst := arr.Value().([]stackitem.Item)
		for i := range src {
			cpItem, n := deepCopyAux(src[i], seen)
			count += n
			dst[i] = cpItem
		}
		arr.MarkAsReadOnly()
		return arr, count
	case *stackitem.Struct:
		src := it.Value().([]stackitem.Item)
		st := stackitem.NewStruct(make([]stackitem.Item, len(src)))
		seen[item] = st
		dst := st.Value().([]stackitem.Item)
		for i := range src {
			cpItem, n := deepCopyAux(src[i], seen)
			count += n
			dst[i] = cpItem
		}
		st.MarkAsReadOnly()
		return st, count
	case *stackitem.Map:
		m := stackitem.NewMap()
		seen[item] = m
		for _, e := range it.Value().([]stackitem.MapElement) {
			key, nKey := deepCopyAux(e.Key, seen)
			value, nValue := deepCopyAux(e.Value, seen)
			count += nKey + nValue
			m.Add(key, value)
		}
		m.MarkAsReadOnly()
		return m, count
	case *stackitem.Buffer:
		return stackitem.NewByteArray(bytes.Clone(it.Value().([]byte))), count
	case *stackitem.Pointer, *stackitem.Interop, stackitem.Bool,
		*stackitem.ByteArray, *stackitem.BigInteger:
		return item, count
	default:
		return nil, count
	}
}
