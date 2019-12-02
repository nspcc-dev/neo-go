package consensus

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/nspcc-dev/dbft/payload"
	"github.com/stretchr/testify/require"
)

func TestRecoveryMessage_Setters(t *testing.T) {
	r := &recoveryMessage{}
	p := new(Payload)
	p.SetType(payload.RecoveryMessageType)
	p.SetPayload(r)

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

	t.Run("prepare response is added", func(t *testing.T) {
		p2 := new(Payload)
		p2.SetType(payload.PrepareResponseType)
		p2.SetPayload(&prepareResponse{
			preparationHash: p1.Hash(),
		})

		r.AddPayload(p2)
		require.NotNil(t, r.PreparationHash())
		require.Equal(t, p1.Hash(), *r.PreparationHash())

		ps := r.GetPrepareResponses(p)
		require.Len(t, ps, 1)
		require.Equal(t, p2, ps[0])
	})

	t.Run("prepare request is added", func(t *testing.T) {
		pr := r.GetPrepareRequest(p)
		require.Nil(t, pr)

		r.AddPayload(p1)
		pr = r.GetPrepareRequest(p)
		require.NotNil(t, pr)
		require.Equal(t, p1, pr)
	})

	t.Run("change view is added", func(t *testing.T) {
		p3 := new(Payload)
		p3.SetType(payload.ChangeViewType)
		p3.SetPayload(&changeView{
			newViewNumber: 1,
			timestamp:     12345,
		})

		r.AddPayload(p3)

		ps := r.GetChangeViews(p)
		require.Len(t, ps, 1)
		require.Equal(t, p3, ps[0])
	})

	t.Run("commit is added", func(t *testing.T) {
		p4 := new(Payload)
		p4.SetType(payload.CommitType)
		p4.SetPayload(randomMessage(t, commitType))

		r.AddPayload(p4)

		ps := r.GetCommits(p)
		require.Len(t, ps, 1)
		require.Equal(t, p4, ps[0])
	})
}
