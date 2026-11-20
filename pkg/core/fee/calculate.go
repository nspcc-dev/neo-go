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
// the witness will be verified at.
func Calculate(base int64, script []byte, isHardforkEnabled func(config.Hardfork) bool) (int64, int) {
	var (
		netFee int64
		size   int
	)
	opcodePrice, verifyPrice := getWitnessPrices(isHardforkEnabled)
	if scparser.IsSignatureContract(script) {
		size += 67 + io.GetVarSize(script)
		netFee += 2*opcodePrice(base, opcode.PUSHDATA1) + base*verifyPrice
	} else if contract, ok := scparser.ParseMultiSigContractWithOpcodes(script); ok {
		m, n := contract.NSigs, len(contract.PublicKeys)
		sizeInv := 66 * m
		size += io.GetVarSize(sizeInv) + sizeInv + io.GetVarSize(script)
		netFee += int64(m+n)*opcodePrice(base, opcode.PUSHDATA1) +
			opcodePrice(base, contract.MOpcode) + opcodePrice(base, contract.NOpcode)
		netFee += base * verifyPrice * int64(n)
	} /*else {
		// We can support more contract types in the future.
	}*/
	return vm.FemtoGasToDatoshiInt64(netFee), size
}

// getWitnessPrices returns an opcode fee calculator in femtoGAS and a signature
// verification coefficient in 10^-11 GAS units.
func getWitnessPrices(isHardforkEnabled func(config.Hardfork) bool) (
	opcodePrice func(int64, opcode.Opcode) int64,
	verifyPrice int64,
) {
	if isHardforkEnabled(config.HFHuyao) {
		return func(base int64, op opcode.Opcode) int64 {
			return OpcodeV1(base, op, nil)
		}, ECDSAVerifyPriceSinceHuyao
	}

	return func(base int64, op opcode.Opcode) int64 {
		return Opcode(base, op) * vm.OpcodePriceMultiplier
	}, ECDSAVerifyPrice * vm.OpcodePriceMultiplier
}
