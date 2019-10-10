package vm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSignatureContractGood(t *testing.T) {
	prog := make([]byte, 35)
	prog[0] = byte(PUSHBYTES33)
	prog[34] = byte(CHECKSIG)
	assert.Equal(t, true, IsSignatureContract(prog))
	assert.Equal(t, true, IsStandardContract(prog))
}

func TestIsSignatureContractBadNoCheckSig(t *testing.T) {
	prog := make([]byte, 34)
	prog[0] = byte(PUSHBYTES33)
	assert.Equal(t, false, IsSignatureContract(prog))
	assert.Equal(t, false, IsStandardContract(prog))
}

func TestIsSignatureContractBadNoCheckSig2(t *testing.T) {
	prog := make([]byte, 35)
	prog[0] = byte(PUSHBYTES33)
	prog[34] = byte(CHECKMULTISIG)
	assert.Equal(t, false, IsSignatureContract(prog))
}

func TestIsSignatureContractBadWrongPush(t *testing.T) {
	prog := make([]byte, 35)
	prog[0] = byte(PUSHBYTES32)
	prog[33] = byte(NOP)
	prog[34] = byte(CHECKSIG)
	assert.Equal(t, false, IsSignatureContract(prog))
}

func TestIsSignatureContractBadWrongInstr(t *testing.T) {
	prog := make([]byte, 30)
	prog[0] = byte(PUSHBYTES33)
	assert.Equal(t, false, IsSignatureContract(prog))
}

func TestIsSignatureContractBadExcessiveInstr(t *testing.T) {
	prog := make([]byte, 36)
	prog[0] = byte(PUSHBYTES33)
	prog[34] = byte(CHECKSIG)
	prog[35] = byte(RET)
	assert.Equal(t, false, IsSignatureContract(prog))
}

func TestIsMultiSigContractGood(t *testing.T) {
	prog := make([]byte, 71)
	prog[0] = byte(PUSH2)
	prog[1] = byte(PUSHBYTES33)
	prog[35] = byte(PUSHBYTES33)
	prog[69] = byte(PUSH2)
	prog[70] = byte(CHECKMULTISIG)
	assert.Equal(t, true, IsMultiSigContract(prog))
	assert.Equal(t, true, IsStandardContract(prog))
}

func TestIsMultiSigContractGoodPushBytes1(t *testing.T) {
	prog := make([]byte, 73)
	prog[0] = byte(PUSHBYTES1)
	prog[1] = 2
	prog[2] = byte(PUSHBYTES33)
	prog[36] = byte(PUSHBYTES33)
	prog[70] = byte(PUSHBYTES1)
	prog[71] = 2
	prog[72] = byte(CHECKMULTISIG)
	assert.Equal(t, true, IsMultiSigContract(prog))
}

func TestIsMultiSigContractGoodPushBytes2(t *testing.T) {
	prog := make([]byte, 75)
	prog[0] = byte(PUSHBYTES2)
	prog[1] = 2
	prog[3] = byte(PUSHBYTES33)
	prog[37] = byte(PUSHBYTES33)
	prog[71] = byte(PUSHBYTES2)
	prog[72] = 2
	prog[74] = byte(CHECKMULTISIG)
	assert.Equal(t, true, IsMultiSigContract(prog))
}

func TestIsMultiSigContractBadNSigs1(t *testing.T) {
	prog := make([]byte, 71)
	prog[0] = byte(PUSH0)
	prog[1] = byte(PUSHBYTES33)
	prog[35] = byte(PUSHBYTES33)
	prog[69] = byte(PUSH2)
	prog[70] = byte(CHECKMULTISIG)
	assert.Equal(t, false, IsMultiSigContract(prog))
	assert.Equal(t, false, IsStandardContract(prog))
}

func TestIsMultiSigContractBadNSigs2(t *testing.T) {
	prog := make([]byte, 73)
	prog[0] = byte(PUSHBYTES2)
	prog[1] = 0xff
	prog[2] = 0xff
	prog[3] = byte(PUSHBYTES33)
	prog[37] = byte(PUSHBYTES33)
	prog[71] = byte(PUSH2)
	prog[72] = byte(CHECKMULTISIG)
	assert.Equal(t, false, IsMultiSigContract(prog))
}

func TestIsMultiSigContractBadNSigs3(t *testing.T) {
	prog := make([]byte, 71)
	prog[0] = byte(PUSH5)
	prog[1] = byte(PUSHBYTES33)
	prog[35] = byte(PUSHBYTES33)
	prog[69] = byte(PUSH2)
	prog[70] = byte(CHECKMULTISIG)
	assert.Equal(t, false, IsMultiSigContract(prog))
}

func TestIsMultiSigContractBadExcessiveNOP1(t *testing.T) {
	prog := make([]byte, 72)
	prog[0] = byte(PUSH2)
	prog[1] = byte(NOP)
	prog[2] = byte(PUSHBYTES33)
	prog[36] = byte(PUSHBYTES33)
	prog[70] = byte(PUSH2)
	prog[71] = byte(CHECKMULTISIG)
	assert.Equal(t, false, IsMultiSigContract(prog))
}

func TestIsMultiSigContractBadExcessiveNOP2(t *testing.T) {
	prog := make([]byte, 72)
	prog[0] = byte(PUSH2)
	prog[1] = byte(PUSHBYTES33)
	prog[35] = byte(NOP)
	prog[36] = byte(PUSHBYTES33)
	prog[70] = byte(PUSH2)
	prog[71] = byte(CHECKMULTISIG)
	assert.Equal(t, false, IsMultiSigContract(prog))
}

func TestIsMultiSigContractBadExcessiveNOP3(t *testing.T) {
	prog := make([]byte, 72)
	prog[0] = byte(PUSH2)
	prog[1] = byte(PUSHBYTES33)
	prog[35] = byte(PUSHBYTES33)
	prog[69] = byte(NOP)
	prog[70] = byte(PUSH2)
	prog[71] = byte(CHECKMULTISIG)
	assert.Equal(t, false, IsMultiSigContract(prog))
}

func TestIsMultiSigContractBadExcessiveNOP4(t *testing.T) {
	prog := make([]byte, 72)
	prog[0] = byte(PUSH2)
	prog[1] = byte(PUSHBYTES33)
	prog[35] = byte(PUSHBYTES33)
	prog[69] = byte(PUSH2)
	prog[70] = byte(NOP)
	prog[71] = byte(CHECKMULTISIG)
	assert.Equal(t, false, IsMultiSigContract(prog))
}

func TestIsMultiSigContractBadExcessiveNOP5(t *testing.T) {
	prog := make([]byte, 72)
	prog[0] = byte(PUSH2)
	prog[1] = byte(PUSHBYTES33)
	prog[35] = byte(PUSHBYTES33)
	prog[69] = byte(PUSH2)
	prog[70] = byte(CHECKMULTISIG)
	prog[71] = byte(NOP)
	assert.Equal(t, false, IsMultiSigContract(prog))
}

func TestIsMultiSigContractBadNKeys1(t *testing.T) {
	prog := make([]byte, 71)
	prog[0] = byte(PUSH2)
	prog[1] = byte(PUSHBYTES33)
	prog[35] = byte(PUSHBYTES33)
	prog[69] = byte(PUSH3)
	prog[70] = byte(CHECKMULTISIG)
	assert.Equal(t, false, IsMultiSigContract(prog))
}

func TestIsMultiSigContractBadNKeys2(t *testing.T) {
	prog := make([]byte, 1)
	prog[0] = byte(PUSH10)
	key := make([]byte, 33)
	var asize = uint16(MaxArraySize + 1)
	for i := 0; i < int(asize); i++ {
		prog = append(prog, byte(PUSHBYTES33))
		prog = append(prog, key...)
	}
	prog = append(prog, byte(PUSHBYTES2), byte(asize&0xff), byte((asize<<8)&0xff), byte(CHECKMULTISIG))
	assert.Equal(t, false, IsMultiSigContract(prog))
}

func TestIsMultiSigContractBadRead1(t *testing.T) {
	prog := make([]byte, 71)
	prog[0] = byte(PUSHBYTES75)
	prog[1] = byte(PUSHBYTES33)
	prog[35] = byte(PUSHBYTES33)
	prog[69] = byte(PUSH2)
	prog[70] = byte(CHECKMULTISIG)
	assert.Equal(t, false, IsMultiSigContract(prog))
}

func TestIsMultiSigContractBadRead2(t *testing.T) {
	prog := make([]byte, 71)
	prog[0] = byte(PUSH2)
	prog[1] = byte(PUSHBYTES33)
	prog[35] = byte(PUSHBYTES75)
	prog[69] = byte(PUSH2)
	prog[70] = byte(CHECKMULTISIG)
	assert.Equal(t, false, IsMultiSigContract(prog))
}

func TestIsMultiSigContractBadRead3(t *testing.T) {
	prog := make([]byte, 71)
	prog[0] = byte(PUSH2)
	prog[1] = byte(PUSHBYTES33)
	prog[35] = byte(PUSHBYTES33)
	prog[69] = byte(PUSH2)
	prog[70] = byte(PUSHBYTES1)
	assert.Equal(t, false, IsMultiSigContract(prog))
}
