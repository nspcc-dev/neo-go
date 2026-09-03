package fee

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
)

type InteropRunStats struct {
	// Length is the byte/character length of a string-like operand, e.g. a
	// called method's name (System.Contract.Call), a loaded script
	// (System.Runtime.LoadScript), or a value read from an iterator
	// (System.Iterator.Value).
	Length int
	// ArgsCount is the number of caller-supplied arguments/parameters, e.g.
	// System.Contract.Call's or System.Runtime.LoadScript's args, or
	// System.Runtime.Notify's event parameters.
	ArgsCount int
	// MethodsCount is the number of methods declared in a contract's
	// manifest ABI. Used by System.Contract.Call only, for the cost of its
	// GetMethod scan.
	MethodsCount int
	// EntriesCount is the number of entries in some other collection the
	// interop has to scan through or serialize as a whole, e.g. manifest
	// permissions (System.Contract.Call), accumulated notifications
	// (System.Runtime.GetNotifications), transaction signers
	// (System.Runtime.CurrentSigners), manifest events (System.Runtime.Notify),
	// or public keys to verify (System.Contract.CreateMultisigAccount,
	// System.Crypto.CheckMultisig).
	EntriesCount int
	// RefsDelta is the total change of refCounter value performed by the
	// interop, same meaning as [vm.OpcodePriceParams.RefsDelta].
	RefsDelta int
}

var (
	// {Length, ArgsCount, MethodsCount, EntriesCount}
	// as {name_len, args_count, methods_count, permissions_count}.
	contractCallW = []int64{1, 5, 5, 20, 2500}

	// {RefsDelta} as {refs_delta}.
	contractCallNativeW = []int64{1, 1000}

	// {Length, RefsDelta} as {num_bytes, refs_delta}.
	iteratorValueW = []int64{1, 1, 500}

	// {EntriesCount, RefsDelta} as {notifications_count, refs_delta}.
	getNotificationsW = []int64{1, 1, 500}

	// {Length, ArgsCount, RefsDelta} as {script_len, args_count, refs_delta}.
	loadScriptW = []int64{1, 1, 1, 1500}

	// {EntriesCount} as {signers_count}.
	currentSignersW = []int64{5, 1000}

	// {EntriesCount, ArgsCount} as {events_count, params_count}.
	notifyW = []int64{1, 5, 2000}

	// {EntriesCount} as {keys_count}.
	createMultisigAccountW = []int64{ECDSAVerifyPrice}

	// {EntriesCount} as {keys_count}.
	checkMultisigW = []int64{ECDSAVerifyPrice}
)

func InteropV1(base int64, name string, stats *InteropRunStats) int64 {
	if f := dynamicInteropCoefficients[name]; f != nil {
		if stats == nil {
			stats = &InteropRunStats{}
		}
		return base * f(stats)
	}
	return base * staticInteropPrices[name]
}

var dynamicInteropCoefficients = map[string]func(*InteropRunStats) int64{
	interopnames.SystemContractCall: func(s *InteropRunStats) int64 {
		return contractCallW[0]*int64(s.Length) + contractCallW[1]*int64(s.ArgsCount) +
			contractCallW[2]*int64(s.MethodsCount) + contractCallW[3]*int64(s.EntriesCount) + contractCallW[4]
	},
	interopnames.SystemContractCallNative: func(s *InteropRunStats) int64 {
		return contractCallNativeW[0]*int64(s.RefsDelta) + contractCallNativeW[1]
	},
	interopnames.SystemIteratorValue: func(s *InteropRunStats) int64 {
		return iteratorValueW[0]*int64(s.Length) + iteratorValueW[1]*int64(s.RefsDelta) + iteratorValueW[2]
	},
	interopnames.SystemRuntimeGetNotifications: func(s *InteropRunStats) int64 {
		return getNotificationsW[0]*int64(s.EntriesCount) + getNotificationsW[1]*int64(s.RefsDelta) + getNotificationsW[2]
	},
	interopnames.SystemRuntimeLoadScript: func(s *InteropRunStats) int64 {
		return loadScriptW[0]*int64(s.Length) + loadScriptW[1]*int64(s.ArgsCount) + loadScriptW[2]*int64(s.RefsDelta) + loadScriptW[3]
	},
	interopnames.SystemRuntimeCurrentSigners: func(s *InteropRunStats) int64 {
		return currentSignersW[0]*int64(s.EntriesCount) + currentSignersW[1]
	},
	interopnames.SystemRuntimeNotify: func(s *InteropRunStats) int64 {
		return notifyW[0]*int64(s.EntriesCount) + notifyW[1]*int64(s.ArgsCount) + notifyW[2]
	},
	interopnames.SystemContractCreateMultisigAccount: func(s *InteropRunStats) int64 {
		return createMultisigAccountW[0] * int64(s.EntriesCount)
	},
	interopnames.SystemCryptoCheckMultisig: func(s *InteropRunStats) int64 {
		return checkMultisigW[0] * int64(s.EntriesCount)
	},
}

var staticInteropPrices = map[string]int64{
	interopnames.SystemContractCreateStandardAccount: ECDSAVerifyPrice,
	interopnames.SystemContractGetCallFlags:          1 << 10,
	interopnames.SystemContractNativeOnPersist:       0,
	interopnames.SystemContractNativePostPersist:     0,
	interopnames.SystemCryptoCheckSig:                ECDSAVerifyPrice,
	interopnames.SystemIteratorNext:                  1 << 15,
	interopnames.SystemRuntimeBurnGas:                1 << 4,
	interopnames.SystemRuntimeCheckWitness:           1 << 10,
	interopnames.SystemRuntimeGasLeft:                1 << 4,
	interopnames.SystemRuntimeGetAddressVersion:      1 << 3,
	interopnames.SystemRuntimeGetCallingScriptHash:   1 << 4,
	interopnames.SystemRuntimeGetEntryScriptHash:     1 << 4,
	interopnames.SystemRuntimeGetExecutingScriptHash: 1 << 4,
	interopnames.SystemRuntimeGetInvocationCounter:   1 << 4,
	interopnames.SystemRuntimeGetNetwork:             1 << 3,
	interopnames.SystemRuntimeGetRandom:              0,
	interopnames.SystemRuntimeGetScriptContainer:     1 << 3,
	interopnames.SystemRuntimeGetTime:                1 << 3,
	interopnames.SystemRuntimeGetTrigger:             1 << 3,
	interopnames.SystemRuntimeLog:                    1 << 15,
	interopnames.SystemRuntimePlatform:               1 << 3,
	interopnames.SystemStorageDelete:                 1 << 15,
	interopnames.SystemStorageFind:                   1 << 15,
	interopnames.SystemStorageGet:                    1 << 15,
	interopnames.SystemStorageGetContext:             1 << 4,
	interopnames.SystemStorageGetReadOnlyContext:     1 << 4,
	interopnames.SystemStoragePut:                    1 << 15,
	interopnames.SystemStorageAsReadOnly:             1 << 4,
	interopnames.SystemStorageLocalGet:               1 << 15,
	interopnames.SystemStorageLocalFind:              1 << 15,
	interopnames.SystemStorageLocalPut:               1 << 15,
	interopnames.SystemStorageLocalDelete:            1 << 15,
}
