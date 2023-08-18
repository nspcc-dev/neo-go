package runtime

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/vm"
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
	return ic.VM.PushContextScriptHash(len(ic.VM.Istack()) - 1)
}

// GetScriptContainer returns transaction or block that contains the script
// being run.
func GetScriptContainer(ic *interop.Context) error {
	c, ok := ic.Container.(itemable)
	if !ok {
		return errors.New("unknown script container")
	}
	ic.VM.Estack().PushItem(c.ToStackItem())
	return nil
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
	curHash := ic.VM.GetCurrentScriptHash()
	ctr, err := ic.GetContract(curHash)
	if err != nil {
		return errors.New("notifications are not allowed in dynamic scripts")
	}
	var (
		ev       = ctr.Manifest.ABI.GetEvent(name)
		checkErr error
	)
	if ev == nil {
		checkErr = fmt.Errorf("notification %s does not exist", name)
	} else {
		err = ev.CheckCompliance(args)
		if err != nil {
			checkErr = fmt.Errorf("notification %s is invalid: %w", name, err)
		}
	}
	if checkErr != nil {
		if ic.IsHardforkEnabled(config.HFBasilisk) {
			return checkErr
		}
		ic.Log.Info("bad notification", zap.String("contract", curHash.StringLE()), zap.String("event", name), zap.Error(checkErr))
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
	ic.AddNotification(curHash, name, stackitem.DeepCopy(stackitem.NewArray(args), true).(*stackitem.Array))
	return nil
}

// LoadScript takes a script and arguments from the stack and loads it into the VM.
func LoadScript(ic *interop.Context) error {
	script := ic.VM.Estack().Pop().Bytes()
	fs := callflag.CallFlag(int32(ic.VM.Estack().Pop().BigInt().Int64()))
	if fs&^callflag.All != 0 {
		return errors.New("call flags out of range")
	}
	args := ic.VM.Estack().Pop().Array()
	err := vm.IsScriptCorrect(script, nil)
	if err != nil {
		return fmt.Errorf("invalid script: %w", err)
	}
	fs = ic.VM.Context().GetCallFlags() & callflag.ReadOnly & fs
	ic.VM.LoadDynamicScript(script, fs)

	for e, i := ic.VM.Estack(), len(args)-1; i >= 0; i-- {
		e.PushItem(args[i])
	}
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
	ic.Log.Info(SystemRuntimeLogMessage,
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

// BurnGas burns GAS to benefit Neo ecosystem.
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
