package core

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
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
	case opcode.SYSCALL:
		interopID := vm.GetInteropID(parameter)
		return getSyscallPrice(v, interopID)
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
		return smartcontract.GetDeploymentPrice(smartcontract.PropertyState(estack.Peek(3).BigInt().Int64()))
	case systemStoragePut, systemStoragePutEx, neoStoragePut, antSharesStoragePut:
		// price for storage PUT is 1 GAS per 1 KiB
		keySize := len(estack.Peek(1).Bytes())
		valSize := len(estack.Peek(2).Bytes())
		return util.Fixed8FromInt64(int64((keySize+valSize-1)/1024 + 1))
	default:
		return util.Fixed8FromInt64(1)
	}
}
