package timer

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/engine"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/interop/util"
)

const defaultTicks = 3

var (
	// ctx holds storage context for contract methods
	ctx storage.Context
	// Check if the invoker of the contract is the specified owner
	owner = util.FromAddress("NULwe3UAHckN2fzNdcVg31tDiaYtMDwANt")
	// ticksKey is a storage key for ticks counter
	ticksKey = []byte("ticks")
)

func init() {
	ctx = storage.GetContext()
}

func _deploy(isUpdate bool) {
	if isUpdate {
		ticksLeft := storage.Get(ctx, ticksKey).(int) + 1
		storage.Put(ctx, ticksKey, ticksLeft)
		runtime.Log("One more tick is added.")
		return
	}
	storage.Put(ctx, ticksKey, defaultTicks)
	runtime.Log("Timer set to " + itoa(defaultTicks) + " ticks.")
}

// Migrate migrates the contract.
func Migrate(script []byte, manifest []byte) bool {
	if !runtime.CheckWitness(owner) {
		runtime.Log("Only owner is allowed to update the contract.")
		return false
	}
	contract.Update(script, manifest)
	runtime.Log("Contract updated.")
	return true
}

// Tick decrement ticks count and checks whether the timer is fired.
func Tick() bool {
	runtime.Log("Tick-tock.")
	ticksLeft := storage.Get(ctx, ticksKey)
	ticksLeft = ticksLeft.(int) - 1
	if ticksLeft == 0 {
		runtime.Log("Fired!")
		return engine.AppCall(runtime.GetExecutingScriptHash(), "selfDestroy").(bool)
	}
	storage.Put(ctx, ticksKey, ticksLeft)
	runtime.Log(itoa(ticksLeft.(int)) + " ticks left.")
	return true
}

// SelfDestroy destroys the contract.
func SelfDestroy() bool {
	if !(runtime.CheckWitness(owner) || runtime.CheckWitness(runtime.GetExecutingScriptHash())) {
		runtime.Log("Only owner or the contract itself are allowed to destroy the contract.")
		return false
	}
	contract.Destroy()
	runtime.Log("Destroyed.")
	return true
}

// itoa converts int to string
func itoa(i int) string {
	digits := "0123456789"
	var (
		res        string
		isNegative bool
	)
	if i < 0 {
		i = -i
		isNegative = true
	}
	for {
		r := i % 10
		res = digits[r:r+1] + res
		i = i / 10
		if i == 0 {
			break
		}
	}
	if isNegative {
		res = "-" + res
	}
	return res
}
