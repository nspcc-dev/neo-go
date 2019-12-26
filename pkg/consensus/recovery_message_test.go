package consensus

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/nspcc-dev/dbft/crypto"
	"github.com/nspcc-dev/dbft/payload"
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
	p.SetType(payload.RecoveryMessageType)
	p.SetPayload(r)
	// sign payload to have verification script
	require.NoError(t, p.Sign(privs[0]))

	req := &prepareRequest{
		timestamp:         87,
		nonce:             321,
		transactionHashes: []util.Uint256{{1}},
		minerTx:           *newMinerTx(123),
		nextConsensus:     util.Uint160{1, 2},
	}
	p1 := new(Payload)
	p1.SetType(payload.PrepareRequestType)
	p1.SetPayload(req)
	p1.SetValidatorIndex(0)
	require.NoError(t, p1.Sign(privs[0]))

	t.Run("prepare response is added", func(t *testing.T) {
		p2 := new(Payload)
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
		require.Equal(t, p1, pr)

		pl := pr.(*Payload)
		require.True(t, pl.Verify(pl.Witness.ScriptHash()))
	})

	t.Run("change view is added", func(t *testing.T) {
		p3 := new(Payload)
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
