package core

import (
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/CityOfZion/neo-go/pkg/vm/opcode"
)

// interopGasRatio is a multiplier by which a number returned from price getter
// and Fixed8 amount of GAS differ. Numbers defined in syscall tables are a multiple
// of 0.001 GAS = Fixed8(10^5).
const interopGasRatio = 100000

// getPrice returns a price for executing op with the provided parameter.
// Some SYSCALLs have variable price depending on their arguments.
func getPrice(v *vm.VM, op opcode.Opcode, parameter []byte) util.Fixed8 {
	if op <= opcode.NOP {
		return 0
	}

	switch op {
	case opcode.APPCALL, opcode.TAILCALL:
		return toFixed8(10)
	case opcode.SYSCALL:
		interopID := vm.GetInteropID(parameter)
		return getSyscallPrice(v, interopID)
	case opcode.SHA1, opcode.SHA256:
		return toFixed8(10)
	case opcode.HASH160, opcode.HASH256:
		return toFixed8(20)
	case opcode.CHECKSIG, opcode.VERIFY:
		return toFixed8(100)
	case opcode.CHECKMULTISIG:
		estack := v.Estack()
		if estack.Len() == 0 {
			return toFixed8(1)
		}

		var cost int

		item := estack.Peek(0)
		switch item.Item().(type) {
		case *vm.ArrayItem, *vm.StructItem:
			cost = len(item.Array())
		default:
			cost = int(item.BigInt().Int64())
		}

		if cost < 1 {
			return toFixed8(1)
		}

		return toFixed8(int64(100 * cost))
	default:
		return toFixed8(1)
	}
}

func toFixed8(n int64) util.Fixed8 {
	return util.Fixed8(n * interopGasRatio)
}

// getSyscallPrice returns cost of executing syscall with provided id.
// Is SYSCALL is not found, cost is 1.
func getSyscallPrice(v *vm.VM, id uint32) util.Fixed8 {
	ifunc := v.GetInteropByID(id)
	if ifunc != nil && ifunc.Price > 0 {
		return toFixed8(int64(ifunc.Price))
	}

	const (
		neoAssetCreate           = 0x1fc6c583 // Neo.Asset.Create
		antSharesAssetCreate     = 0x99025068 // AntShares.Asset.Create
		neoAssetRenew            = 0x71908478 // Neo.Asset.Renew
		antSharesAssetRenew      = 0xaf22447b // AntShares.Asset.Renew
		neoContractCreate        = 0x6ea56cf6 // Neo.Contract.Create
		neoContractMigrate       = 0x90621b47 // Neo.Contract.Migrate
		antSharesContractCreate  = 0x2a28d29b // AntShares.Contract.Create
		antSharesContractMigrate = 0xa934c8bb // AntShares.Contract.Migrate
		systemStoragePut         = 0x84183fe6 // System.Storage.Put
		systemStoragePutEx       = 0x3a9be173 // System.Storage.PutEx
		neoStoragePut            = 0xf541a152 // Neo.Storage.Put
		antSharesStoragePut      = 0x5f300a9e // AntShares.Storage.Put
	)

	estack := v.Estack()

	switch id {
	case neoAssetCreate, antSharesAssetCreate:
		return util.Fixed8FromInt64(5000)
	case neoAssetRenew, antSharesAssetRenew:
		arg := estack.Peek(1).BigInt().Int64()
		return util.Fixed8FromInt64(arg * 5000)
	case neoContractCreate, neoContractMigrate, antSharesContractCreate, antSharesContractMigrate:
		fee := int64(100)
		props := smartcontract.PropertyState(estack.Peek(3).BigInt().Int64())

		if props&smartcontract.HasStorage != 0 {
			fee += 400
		}

		if props&smartcontract.HasDynamicInvoke != 0 {
			fee += 500
		}

		return util.Fixed8FromInt64(fee)
	case systemStoragePut, systemStoragePutEx, neoStoragePut, antSharesStoragePut:
		// price for storage PUT is 1 GAS per 1 KiB
		keySize := len(estack.Peek(1).Bytes())
		valSize := len(estack.Peek(2).Bytes())
		return util.Fixed8FromInt64(int64((keySize+valSize-1)/1024 + 1))
	default:
		return util.Fixed8FromInt64(1)
	}
}
