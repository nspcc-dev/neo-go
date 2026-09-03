package runtime

import (
	"encoding/binary"
	"errors"
	"math/big"
	"slices"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/twmb/murmur3"
)

// GasLeft returns the remaining amount of GAS.
func GasLeft(ic *interop.Context) (*fee.InteropRunStats, error) {
	ic.VM.Estack().PushItem(stackitem.NewBigInteger(ic.VM.GasLeft()))
	return nil, nil
}

// GetNotifications returns notifications emitted in the current execution context.
func GetNotifications(ic *interop.Context) (*fee.InteropRunStats, error) {
	item := ic.VM.Estack().Pop().Item()
	notifications := ic.Notifications
	if _, ok := item.(stackitem.Null); !ok {
		b, err := item.TryBytes()
		if err != nil {
			return nil, err
		}
		u, err := util.Uint160DecodeBytesBE(b)
		if err != nil {
			return nil, err
		}
		notifications = []state.NotificationEvent{}
		for i := range ic.Notifications {
			if ic.Notifications[i].ScriptHash.Equals(u) {
				notifications = append(notifications, ic.Notifications[i])
			}
		}
	}
	if len(notifications) > vm.MaxStackSize {
		return nil, errors.New("too many notifications")
	}
	// EntriesCount=notifications_count, RefsDelta=refs_delta.
	arr := stackitem.NewArray(make([]stackitem.Item, 0, len(notifications)))
	for i := range notifications {
		ev := stackitem.NewArray([]stackitem.Item{
			stackitem.NewByteArray(notifications[i].ScriptHash.BytesBE()),
			stackitem.Make(notifications[i].Name),
			notifications[i].Item,
		})
		arr.Append(ev)
	}
	r1 := ic.VM.RefCount()
	ic.VM.Estack().PushItem(arr)
	return &fee.InteropRunStats{EntriesCount: len(notifications), RefsDelta: ic.VM.RefCount() - r1}, nil
}

// GetInvocationCounter returns how many times the current contract has been invoked during the current tx execution.
func GetInvocationCounter(ic *interop.Context) (*fee.InteropRunStats, error) {
	currentScriptHash := ic.VM.GetCurrentScriptHash()
	count, ok := ic.Invocations[currentScriptHash]
	if !ok {
		count = 1
		ic.Invocations[currentScriptHash] = count
	}
	ic.VM.Estack().PushItem(stackitem.NewBigInteger(big.NewInt(int64(count))))
	return nil, nil
}

// GetAddressVersion returns the address version of the current protocol.
func GetAddressVersion(ic *interop.Context) (*fee.InteropRunStats, error) {
	ic.VM.Estack().PushItem(stackitem.NewBigInteger(big.NewInt(int64(address.NEO3Prefix))))
	return nil, nil
}

// GetNetwork returns chain network number.
func GetNetwork(ic *interop.Context) (*fee.InteropRunStats, error) {
	m := ic.Chain.GetConfig().Magic
	ic.VM.Estack().PushItem(stackitem.NewBigInteger(big.NewInt(int64(m))))
	return nil, nil
}

// GetRandom returns pseudo-random number which depends on block nonce and transaction hash.
func GetRandom(ic *interop.Context) (*fee.InteropRunStats, error) {
	var (
		price int64
		seed  = ic.Network
	)
	isHF := ic.IsHardforkEnabled(config.HFAspidochelone)
	if isHF {
		price = 1 << 13
		seed += ic.GetRandomCounter
		ic.GetRandomCounter++
	} else {
		price = 1 << 4
	}
	res := murmur128(ic.NonceData[:], seed)
	if !isHF {
		ic.NonceData = [interop.ContextNonceDataLen]byte(res)
	}
	if err := ic.VM.AddPicoGas(ic.BaseExecFee() * price); err != nil {
		return nil, err
	}
	// Resulting data is interpreted as an unsigned LE integer.
	slices.Reverse(res)
	ic.VM.Estack().PushItem(stackitem.NewBigInteger(new(big.Int).SetBytes(res)))
	return nil, nil
}

func murmur128(data []byte, seed uint32) []byte {
	h1, h2 := murmur3.SeedSum128(uint64(seed), uint64(seed), data)
	result := make([]byte, 16)
	binary.LittleEndian.PutUint64(result, h1)
	binary.LittleEndian.PutUint64(result[8:], h2)
	return result
}
