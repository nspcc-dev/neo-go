package fee

import (
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/scparser"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

const (
	// ECDSAVerifyPrice is a gas price of a single verification before Huyao hardfork.
	ECDSAVerifyPrice = 1 << 15
	// ECDSAVerifyPriceSinceHuyao is a gas price of a single verification since
	// Huyao hardfork in 10^-11 GAS units.
	ECDSAVerifyPriceSinceHuyao = 2326967
	// ReadFromDiskPrice is a gas price of a single storage read since Huyao
	// hardfork in 10^-11 GAS units.
	ReadFromDiskPrice = 6799366
	// ContractCallPrice is a gas price of a single contract call since Huyao
	// hardfork in 10^-11 GAS units.
	ContractCallPrice = 2205000
)

// Calculate returns network fee for a transaction in Datoshi units.
// isHardforkEnabled reports whether a hardfork is enabled at the height
// the script will be executed at.
func Calculate(base int64, script []byte, isHardforkEnabled func(*config.Hardfork) bool) (int64, int) {
	var (
		netFee int64
		size   int
	)
	priceTable, ecdsaVerifyPrice := getPriceTable(isHardforkEnabled)
	if scparser.IsSignatureContract(script) {
		size += 67 + io.GetVarSize(script)
		netFee += 2*priceTable(base, opcode.PUSHDATA1) + base*ecdsaVerifyPrice
	} else if m, mOpcode, pubs, nOpcode, ok := scparser.ParseMultiSigContractExtended(script); ok {
		n := len(pubs)
		sizeInv := 66 * m
		size += io.GetVarSize(sizeInv) + sizeInv + io.GetVarSize(script)
		netFee += int64(m+n)*priceTable(base, opcode.PUSHDATA1) +
			priceTable(base, mOpcode) + priceTable(base, nOpcode)
		netFee += base * ecdsaVerifyPrice * int64(n)
	} /*else {
		// We can support more contract types in the future.
	}*/
	return vm.FemtoGasToDatoshiInt64(netFee), size
}

// getPriceTable returns an opcode fee calculator in femtoGAS and a signature
// verification coefficient in 10^-11 GAS units.
// The returned opcode fee calculator supports only statically priced opcodes.
func getPriceTable(isHardforkEnabled func(*config.Hardfork) bool) (
	opcodePrice func(int64, opcode.Opcode) int64,
	verifyPrice int64,
) {
	if isHardforkEnabled(new(config.HFHuyao)) {
		return func(base int64, op opcode.Opcode) int64 {
			if dynamicCoefficients[op] != nil {
				// Dynamic prices require execution parameters unavailable to this calculator.
				panic("bug: cannot calculate dynamic opcode price")
			}
			return OpcodeV1(base, op, nil)
		}, ECDSAVerifyPriceSinceHuyao
	}

	return func(base int64, op opcode.Opcode) int64 {
		return Opcode(base, op) * vm.OpcodePriceMultiplier
	}, ECDSAVerifyPrice * vm.OpcodePriceMultiplier
}
