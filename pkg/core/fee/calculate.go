package fee

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// ECDSAVerifyPrice is a gas price of a single verification.
const ECDSAVerifyPrice = 1 << 15

// Calculate returns network fee for transaction.
func Calculate(base int64, script []byte) (int64, int) {
	var size int
	if vm.IsSignatureContract(script) {
		size += 67 + io.GetVarSize(script)
	} else if m, _, ok := vm.ParseMultiSigContract(script); ok {
		sizeInv := 66 * m
		size += io.GetVarSize(sizeInv) + sizeInv + io.GetVarSize(script)
	} /*else {
		// We can support more contract types in the future.
	}*/
	return 0, size
}

func calculateMultisig(base int64, n int) int64 {
	result := Opcode(base, opcode.PUSHDATA1) * int64(n)
	bw := io.NewBufBinWriter()
	emit.Int(bw.BinWriter, int64(n))
	// it's a hack because coefficients of small PUSH* opcodes are equal
	result += Opcode(base, opcode.Opcode(bw.Bytes()[0]))
	return result
}
