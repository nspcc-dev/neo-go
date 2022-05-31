package runtime

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"go.uber.org/zap"
)

const (
	// MaxEventNameLen is the maximum length of a name for event.
	MaxEventNameLen = 32
	// MaxNotificationSize is the maximum length of a runtime log message.
	MaxNotificationSize = 1024
)

// GetExecutingScriptHash returns executing script hash.
func GetExecutingScriptHash(ic *interop.Context) error {
	return ic.VM.PushContextScriptHash(0)
}

// GetCallingScriptHash returns calling script hash.
// While Executing and Entry script hashes are always valid for non-native contracts,
// Calling hash is set explicitly when native contracts are used, because when switching from
// one native to another, no operations are performed on invocation stack.
func GetCallingScriptHash(ic *interop.Context) error {
	h := ic.VM.GetCallingScriptHash()
	ic.VM.Estack().PushItem(stackitem.NewByteArray(h.BytesBE()))
	return nil
}

// GetEntryScriptHash returns entry script hash.
func GetEntryScriptHash(ic *interop.Context) error {
	return ic.VM.PushContextScriptHash(ic.VM.Istack().Len() - 1)
}

// Platform returns the name of the platform.
func Platform(ic *interop.Context) error {
	ic.VM.Estack().PushItem(stackitem.NewByteArray([]byte("NEO")))
	return nil
}

// GetTrigger returns the script trigger.
func GetTrigger(ic *interop.Context) error {
	ic.VM.Estack().PushItem(stackitem.NewBigInteger(big.NewInt(int64(ic.Trigger))))
	return nil
}

// Notify should pass stack item to the notify plugin to handle it, but
// in neo-go the only meaningful thing to do here is to log.
func Notify(ic *interop.Context) error {
	name := ic.VM.Estack().Pop().String()
	elem := ic.VM.Estack().Pop()
	args := elem.Array()
	if len(name) > MaxEventNameLen {
		return fmt.Errorf("event name must be less than %d", MaxEventNameLen)
	}
	// But it has to be serializable, otherwise we either have some broken
	// (recursive) structure inside or an interop item that can't be used
	// outside of the interop subsystem anyway.
	bytes, err := ic.DAO.GetItemCtx().Serialize(elem.Item(), false)
	if err != nil {
		return fmt.Errorf("bad notification: %w", err)
	}
	if len(bytes) > MaxNotificationSize {
		return fmt.Errorf("notification size shouldn't exceed %d", MaxNotificationSize)
	}
	ic.AddNotification(ic.VM.GetCurrentScriptHash(), name, stackitem.DeepCopy(stackitem.NewArray(args)).(*stackitem.Array))
	return nil
}

// Log logs the message passed.
func Log(ic *interop.Context) error {
	state := ic.VM.Estack().Pop().String()
	if len(state) > MaxNotificationSize {
		return fmt.Errorf("message length shouldn't exceed %v", MaxNotificationSize)
	}
	var txHash string
	if ic.Tx != nil {
		txHash = ic.Tx.Hash().StringLE()
	}
	ic.Log.Info("runtime log",
		zap.String("tx", txHash),
		zap.String("script", ic.VM.GetCurrentScriptHash().StringLE()),
		zap.String("msg", state))
	return nil
}

// GetTime returns timestamp of the block being verified, or the latest
// one in the blockchain if no block is given to Context.
func GetTime(ic *interop.Context) error {
	ic.VM.Estack().PushItem(stackitem.NewBigInteger(new(big.Int).SetUint64(ic.Block.Timestamp)))
	return nil
}

// BurnGas burns GAS to benefit NEO ecosystem.
func BurnGas(ic *interop.Context) error {
	gas := ic.VM.Estack().Pop().BigInt()
	if !gas.IsInt64() {
		return errors.New("invalid GAS value")
	}

	g := gas.Int64()
	if g <= 0 {
		return errors.New("GAS must be positive")
	}

	if !ic.VM.AddGas(g) {
		return errors.New("GAS limit exceeded")
	}
	return nil
}
