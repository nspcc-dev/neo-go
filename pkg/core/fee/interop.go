package fee

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
)

type InteropRunStats struct {
	Stat int
}

var (
	createMultisigAccountW = []int64{1471, 11876}
	checkMultisigW         = []int64{176990, 164642}
	checkWitnessW          = []int64{40025, 220000}
	getNotificationsW      = []int64{975, 3692}
	loadScriptW            = []int64{29, 130000}
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
	interopnames.SystemContractCreateMultisigAccount: func(s *InteropRunStats) int64 {
		return createMultisigAccountW[0]*int64(s.Stat) + createMultisigAccountW[1]
	},
	interopnames.SystemCryptoCheckMultisig: func(s *InteropRunStats) int64 {
		return checkMultisigW[0]*int64(s.Stat) + checkMultisigW[1]
	},
	interopnames.SystemRuntimeCheckWitness: func(s *InteropRunStats) int64 {
		return checkWitnessW[0]*int64(s.Stat) + checkWitnessW[1]
	},
	interopnames.SystemRuntimeGetNotifications: func(s *InteropRunStats) int64 {
		return getNotificationsW[0]*int64(s.Stat) + getNotificationsW[1]
	},
	interopnames.SystemRuntimeLoadScript: func(s *InteropRunStats) int64 {
		return loadScriptW[0]*int64(s.Stat) + loadScriptW[1]
	},
}

var staticInteropPrices = map[string]int64{
	interopnames.SystemContractCall:                  270000,
	interopnames.SystemContractCallNative:            225000,
	interopnames.SystemContractCreateStandardAccount: 8909,
	interopnames.SystemContractGetCallFlags:          1283,
	interopnames.SystemContractNativeOnPersist:       0,
	interopnames.SystemContractNativePostPersist:     0,
	interopnames.SystemCryptoCheckSig:                3465900,
	interopnames.SystemIteratorNext:                  1000,
	interopnames.SystemIteratorValue:                 375000,
	interopnames.SystemRuntimeBurnGas:                1522,
	interopnames.SystemRuntimeCurrentSigners:         31792,
	interopnames.SystemRuntimeGasLeft:                1725,
	interopnames.SystemRuntimeGetAddressVersion:      1319,
	interopnames.SystemRuntimeGetCallingScriptHash:   4540,
	interopnames.SystemRuntimeGetEntryScriptHash:     1554,
	interopnames.SystemRuntimeGetExecutingScriptHash: 1513,
	interopnames.SystemRuntimeGetInvocationCounter:   3130,
	interopnames.SystemRuntimeGetNetwork:             1518,
	interopnames.SystemRuntimeGetRandom:              3148,
	interopnames.SystemRuntimeGetScriptContainer:     5608,
	interopnames.SystemRuntimeGetTime:                1396,
	interopnames.SystemRuntimeGetTrigger:             1369,
	interopnames.SystemRuntimeLog:                    4896,
	interopnames.SystemRuntimeNotify:                 330000,
	interopnames.SystemRuntimePlatform:               1680,
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
