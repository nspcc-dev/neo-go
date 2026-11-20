package fee_test

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func TestCalculate(t *testing.T) {
	const execFeeFactor = 30
	const base = execFeeFactor * vm.ExecFeeFactorMultiplier
	const signaturePushSize = 1 + 1 + keys.SignatureLen // PUSHDATA1, length and signature.

	checkFee := func(t *testing.T, script []byte, beforeHuyao, sinceHuyao int64, wantSize int) {
		for _, fork := range []struct {
			name  string
			huyao bool
			want  int64
		}{
			{name: "before Huyao", want: vm.PicoGasToDatoshiInt64(beforeHuyao)},
			{name: "since Huyao", huyao: true, want: vm.FemtoGasToDatoshiInt64(sinceHuyao)},
		} {
			t.Run(fork.name, func(t *testing.T) {
				isHardforkEnabled := func(hf config.Hardfork) bool {
					return hf == config.HFHuyao && fork.huyao
				}
				cost, size := fee.Calculate(base, script, isHardforkEnabled)
				require.Equal(t, fork.want, cost)
				require.Equal(t, wantSize, size)
			})
		}
	}

	t.Run("signature", func(t *testing.T) {
		priv, err := keys.NewPrivateKey()
		require.NoError(t, err)
		verification := io.NewBufBinWriter()
		emit.CheckSig(verification.BinWriter, priv.PublicKey().Bytes())
		require.NoError(t, verification.Err)
		script := verification.Bytes()

		beforeHuyao := 2*fee.Opcode(base, opcode.PUSHDATA1) + base*fee.ECDSAVerifyPrice
		sinceHuyao := 2*fee.OpcodeV1(base, opcode.PUSHDATA1, nil) + base*fee.ECDSAVerifyPriceSinceHuyao
		wantSize := io.GetVarSize(signaturePushSize) + signaturePushSize + io.GetVarSize(script)
		checkFee(t, script, beforeHuyao, sinceHuyao, wantSize)
	})

	t.Run("multi signature", func(t *testing.T) {
		type testCase struct {
			name             string
			m, n             int
			mOpcode, nOpcode opcode.Opcode
		}
		cases := []testCase{
			{name: "2 of 3", m: 2, n: 3, mOpcode: opcode.PUSH2, nOpcode: opcode.PUSH3},
			{name: "16 of 17", m: 16, n: 17, mOpcode: opcode.PUSH16, nOpcode: opcode.PUSHINT8},
			{name: "17 of 17", m: 17, n: 17, mOpcode: opcode.PUSHINT8, nOpcode: opcode.PUSHINT8},
			{name: "1 of 128", m: 1, n: 128, mOpcode: opcode.PUSH1, nOpcode: opcode.PUSHINT16},
			{name: "mixed counters", m: 1, n: 1, mOpcode: opcode.PUSHINT32, nOpcode: opcode.PUSHINT64},
		}

		var maxKeys int
		for _, tc := range cases {
			maxKeys = max(maxKeys, tc.n)
		}
		pubs := make([][]byte, maxKeys)
		for i := range pubs {
			priv, err := keys.NewPrivateKey()
			require.NoError(t, err)
			pubs[i] = priv.PublicKey().Bytes()
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				verification := io.NewBufBinWriter()
				emitWitnessCount(verification.BinWriter, tc.m, tc.mOpcode)
				for _, pub := range pubs[:tc.n] {
					emit.Bytes(verification.BinWriter, pub)
				}
				emitWitnessCount(verification.BinWriter, tc.n, tc.nOpcode)
				emit.Syscall(verification.BinWriter, interopnames.SystemCryptoCheckMultisig)
				require.NoError(t, verification.Err)
				script := verification.Bytes()

				dataPushes := int64(tc.m + tc.n)
				beforeHuyao := dataPushes*fee.Opcode(base, opcode.PUSHDATA1) +
					fee.Opcode(base, tc.mOpcode, tc.nOpcode) + base*fee.ECDSAVerifyPrice*int64(tc.n)
				sinceHuyao := dataPushes*fee.OpcodeV1(base, opcode.PUSHDATA1, nil) +
					fee.OpcodeV1(base, tc.mOpcode, nil) + fee.OpcodeV1(base, tc.nOpcode, nil) +
					base*fee.ECDSAVerifyPriceSinceHuyao*int64(tc.n)

				signaturesSize := tc.m * signaturePushSize
				wantSize := io.GetVarSize(signaturesSize) + signaturesSize + io.GetVarSize(script)

				checkFee(t, script, beforeHuyao, sinceHuyao, wantSize)
			})
		}
	})
}

func emitWitnessCount(w *io.BinWriter, n int, op opcode.Opcode) {
	emit.Opcodes(w, op)
	if op <= opcode.PUSHINT256 {
		data := make([]byte, 1<<op)
		for i := range data {
			data[i] = byte(n)
			n >>= 8
		}
		w.WriteBytes(data)
	}
}
