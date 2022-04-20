package runtime

import (
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/twmb/murmur3"
)

// GasLeft returns the remaining amount of GAS.
func GasLeft(ic *interop.Context) error {
	if ic.VM.GasLimit == -1 {
		ic.VM.Estack().PushItem(stackitem.NewBigInteger(big.NewInt(ic.VM.GasLimit)))
	} else {
		ic.VM.Estack().PushItem(stackitem.NewBigInteger(big.NewInt(ic.VM.GasLimit - ic.VM.GasConsumed())))
	}
	return nil
}

// GetNotifications returns notifications emitted in the current execution context.
func GetNotifications(ic *interop.Context) error {
	item := ic.VM.Estack().Pop().Item()
	notifications := ic.Notifications
	if _, ok := item.(stackitem.Null); !ok {
		b, err := item.TryBytes()
		if err != nil {
			return err
		}
		u, err := util.Uint160DecodeBytesBE(b)
		if err != nil {
			return err
		}
		notifications = []state.NotificationEvent{}
		for i := range ic.Notifications {
			if ic.Notifications[i].ScriptHash.Equals(u) {
				notifications = append(notifications, ic.Notifications[i])
			}
		}
	}
	if len(notifications) > vm.MaxStackSize {
		return errors.New("too many notifications")
	}
	arr := stackitem.NewArray(make([]stackitem.Item, 0, len(notifications)))
	for i := range notifications {
		ev := stackitem.NewArray([]stackitem.Item{
			stackitem.NewByteArray(notifications[i].ScriptHash.BytesBE()),
			stackitem.Make(notifications[i].Name),
			stackitem.DeepCopy(notifications[i].Item).(*stackitem.Array),
		})
		arr.Append(ev)
	}
	ic.VM.Estack().PushItem(arr)
	return nil
}

// GetInvocationCounter returns how many times the current contract has been invoked during the current tx execution.
func GetInvocationCounter(ic *interop.Context) error {
	currentScriptHash := ic.VM.GetCurrentScriptHash()
	count, ok := ic.Invocations[currentScriptHash]
	if !ok {
		count = 1
		ic.Invocations[currentScriptHash] = count
	}
	ic.VM.Estack().PushItem(stackitem.NewBigInteger(big.NewInt(int64(count))))
	return nil
}

// GetAddressVersion returns the address version of the current protocol.
func GetAddressVersion(ic *interop.Context) error {
	ic.VM.Estack().PushItem(stackitem.NewBigInteger(big.NewInt(int64(address.NEO3Prefix))))
	return nil
}

// GetNetwork returns chain network number.
func GetNetwork(ic *interop.Context) error {
	m := ic.Chain.GetConfig().Magic
	ic.VM.Estack().PushItem(stackitem.NewBigInteger(big.NewInt(int64(m))))
	return nil
}

// GetRandom returns pseudo-random number which depends on block nonce and transaction hash.
func GetRandom(ic *interop.Context) error {
	res := murmur128(ic.NonceData[:], ic.Network)
	ic.VM.Estack().PushItem(stackitem.NewBigInteger(bigint.FromBytesUnsigned(res)))
	copy(ic.NonceData[:], res)
	return nil
}

func murmur128(data []byte, seed uint32) []byte {
	h1, h2 := murmur3.SeedSum128(uint64(seed), uint64(seed), data)
	result := make([]byte, 16)
	binary.LittleEndian.PutUint64(result, h1)
	binary.LittleEndian.PutUint64(result[8:], h2)
	return result
}
