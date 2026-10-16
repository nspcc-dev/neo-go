package fee

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/scparser"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

const (
	// ECDSAVerifyPrice is a gas price of a single verification before Huyao hardfork.
	ECDSAVerifyPrice = 1 << 15
	// ECDSAVerifyPriceAfterHuyao is a gas price of a single verification after Huyao hardfork
	// in 10^-11 GAS units.
	ECDSAVerifyPriceAfterHuyao = 2326967
	// ReadFromDiskPrice is a gas price of a single storage read after Huyao hardfork
	// in 10^-11 GAS units.
	ReadFromDiskPrice = 6799366
)

// Calculate returns network fee for a transaction in Datoshi units.
func Calculate(base int64, script []byte) (int64, int) {
	var (
		netFee int64
		size   int
	)
	if scparser.IsSignatureContract(script) {
		size += 67 + io.GetVarSize(script)
		netFee += Opcode(base, opcode.PUSHDATA1, opcode.PUSHDATA1) + base*ECDSAVerifyPrice
	} else if m, pubs, ok := scparser.ParseMultiSigContract(script); ok {
		n := len(pubs)
		sizeInv := 66 * m
		size += io.GetVarSize(sizeInv) + sizeInv + io.GetVarSize(script)
		netFee += calculateMultisig(base, m) + calculateMultisig(base, n)
		netFee += base * ECDSAVerifyPrice * int64(n)
	} /*else {
		// We can support more contract types in the future.
	}*/
	return vm.PicoGasToDatoshiInt64(netFee), size
}

func calculateMultisig(base int64, n int) int64 {
	result := Opcode(base, opcode.PUSHDATA1) * int64(n)
	bw := io.NewBufBinWriter()
	emit.Int(bw.BinWriter, int64(n))
	// it's a hack because coefficients of small PUSH* opcodes are equal
	result += Opcode(base, opcode.Opcode(bw.Bytes()[0]))
	return result
}
