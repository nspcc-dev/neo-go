package consensus

import (
	"testing"

	"github.com/nspcc-dev/dbft/crypto"
	"github.com/nspcc-dev/dbft/payload"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestRecoveryMessage_Setters(t *testing.T) {
	const size = 5

	privs := getKeys(t, size)
	pubs := make([]crypto.PublicKey, 5)
	for i := range pubs {
		pubs[i] = &publicKey{privs[i].PublicKey()}
	}

	r := &recoveryMessage{}
	p := new(Payload)
	p.message = &message{}
	p.SetType(payload.RecoveryMessageType)
	p.SetPayload(r)
	// sign payload to have verification script
	require.NoError(t, p.Sign(privs[0]))

	req := &prepareRequest{
		timestamp:         87,
		nonce:             321,
		transactionHashes: []util.Uint256{{1}},
		nextConsensus:     util.Uint160{1, 2},
	}
	p1 := new(Payload)
	p1.message = &message{}
	p1.SetType(payload.PrepareRequestType)
	p1.SetPayload(req)
	p1.SetValidatorIndex(0)
	require.NoError(t, p1.Sign(privs[0]))

	t.Run("prepare response is added", func(t *testing.T) {
		p2 := new(Payload)
		p2.message = &message{}
		p2.SetType(payload.PrepareResponseType)
		p2.SetPayload(&prepareResponse{
			preparationHash: p1.Hash(),
		})
		p2.SetValidatorIndex(1)
		require.NoError(t, p2.Sign(privs[1]))

		r.AddPayload(p2)
		require.NotNil(t, r.PreparationHash())
		require.Equal(t, p1.Hash(), *r.PreparationHash())

		ps := r.GetPrepareResponses(p, pubs)
		require.Len(t, ps, 1)
		require.Equal(t, p2, ps[0])
		ps0 := ps[0].(*Payload)
		require.True(t, ps0.Verify(ps0.Witness.ScriptHash()))
	})

	t.Run("prepare request is added", func(t *testing.T) {
		pr := r.GetPrepareRequest(p, pubs, p1.ValidatorIndex())
		require.Nil(t, pr)

		r.AddPayload(p1)
		pr = r.GetPrepareRequest(p, pubs, p1.ValidatorIndex())
		require.NotNil(t, pr)
		require.Equal(t, p1.Hash(), pr.Hash())
		require.Equal(t, p1, pr)

		pl := pr.(*Payload)
		require.True(t, pl.Verify(pl.Witness.ScriptHash()))
	})

	t.Run("change view is added", func(t *testing.T) {
		p3 := new(Payload)
		p3.message = &message{}
		p3.SetType(payload.ChangeViewType)
		p3.SetPayload(&changeView{
			newViewNumber: 1,
			timestamp:     12345,
		})
		p3.SetValidatorIndex(3)
		require.NoError(t, p3.Sign(privs[3]))

		r.AddPayload(p3)

		ps := r.GetChangeViews(p, pubs)
		require.Len(t, ps, 1)
		require.Equal(t, p3, ps[0])

		ps0 := ps[0].(*Payload)
		require.True(t, ps0.Verify(ps0.Witness.ScriptHash()))
	})

	t.Run("commit is added", func(t *testing.T) {
		p4 := new(Payload)
		p4.message = &message{}
		p4.SetType(payload.CommitType)
		p4.SetPayload(randomMessage(t, commitType))
		p4.SetValidatorIndex(4)
		require.NoError(t, p4.Sign(privs[4]))

		r.AddPayload(p4)

		ps := r.GetCommits(p, pubs)
		require.Len(t, ps, 1)
		require.Equal(t, p4, ps[0])

		ps0 := ps[0].(*Payload)
		require.True(t, ps0.Verify(ps0.Witness.ScriptHash()))
	})
}

//TODO NEO3.0: Update binary
/*
func TestRecoveryMessage_DecodeFromTestnet(t *testing.T) {
	hexDump := "00000000924b2fa6728782b6afb94873a377c49f31573005e7f2945beb27158ec2e887300d180000010000000000" +
		"fd29024100000120003db64b5e8e4ab7138abe65a3be48d3a3f5d10013ab9ffee489706078714f1ea20161e7ba952fdfd5f543891b1fe053af401bc34e9e3f63c90e3c0d6675d156344b00008e4ab71300000000030000414079946c76007e4297b06b074a20dc1d1d6871c74976f244df81bd03f4158a11dd485ed50fc0cc7c6ad352addd8440c5a55d7b7449650bb200e5e58b1fb8a0390c010041403631a490b17ca4fcfe52ed2e7a4ca4c0d3fcca67e73a1ef071f385db1d37cefa7a2de6e56654788647e9142425c29449b0bbfee5c46a96c4bdc79b23c1f862fc02004140147914878c23a9624a62598cebe2c75fdce80c1e19b5c73aa511630f67d4e5a660c63daad7fcfa9bd944f258f51427cb80730b8beb3015a3c2766325bf291a8e02000000989f8fa676ed07885a46ee08af10e1fa1893ef20fbd557dc3c1a9dc498189d5fceff694dcb2085e4969d90c56433b88fd7ba1caef9363829c70419a5314ac36541404f3ee34e11c521f2e31fee439206474d36951443014354ce81b32bd1787e6a92212737f7f72bee59c403ff74292ebf78c4091081174b5921c148cedcbe7bd585000100acfc8399bda6429c64b5c09885a3e4f1a0629f59125df03be956c00f5bb77616c43e43250e96700f80c42ef3e169e9ff9f906518acf0da17c53563ba41d91ebc41409957436afd1736970d4b5e52b8d845663d6b0335a34cf78ece733c71be876cf30125e9bfea197a607ea6945cef7ef28a74676ec23d14378f7ec23964544b6710014140b634941ecab3a5dd7251f9213bfbcff2021b1e3d966e6800007ea6f0d72ec46d2c04c042800e103091d2f5d184d997a10b890b13bf06b1078a4f1822d722891a232102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62ac"
	data, err := hex.DecodeString(hexDump)
	require.NoError(t, err)

	buf := io.NewBinReaderFromBuf(data)
	var p Payload
	p.DecodeBinary(buf)
	require.NoError(t, buf.Err)

	buf.ReadB()
	require.Equal(t, gio.EOF, buf.Err)
}
*/
func getKeys(t *testing.T, n int) []*privateKey {
	privs := make([]*privateKey, 0, n)
	for i := 0; i < n; i++ {
		priv, err := keys.NewPrivateKey()
		require.NoError(t, err)
		require.NotNil(t, priv)

		privs = append(privs, &privateKey{PrivateKey: priv})
	}

	return privs
}
