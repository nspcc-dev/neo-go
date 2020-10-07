package fee

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop/crypto"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// Calculate returns network fee for transaction
func Calculate(script []byte) (int64, int) {
	var (
		netFee int64
		size   int
	)
	if vm.IsSignatureContract(script) {
		size += 67 + io.GetVarSize(script)
		netFee += Opcode(opcode.PUSHDATA1, opcode.PUSHNULL, opcode.PUSHDATA1) + crypto.ECDSAVerifyPrice
	} else if m, pubs, ok := vm.ParseMultiSigContract(script); ok {
		n := len(pubs)
		sizeInv := 66 * m
		size += io.GetVarSize(sizeInv) + sizeInv + io.GetVarSize(script)
		netFee += calculateMultisig(m) + calculateMultisig(n)
		netFee += Opcode(opcode.PUSHNULL) + crypto.ECDSAVerifyPrice*int64(n)
	} else {
		// We can support more contract types in the future.
	}
	return netFee, size
}

func calculateMultisig(n int) int64 {
	result := Opcode(opcode.PUSHDATA1) * int64(n)
	bw := io.NewBufBinWriter()
	emit.Int(bw.BinWriter, int64(n))
	// it's a hack because prices of small PUSH* opcodes are equal
	result += Opcode(opcode.Opcode(bw.Bytes()[0]))
	return result
}
