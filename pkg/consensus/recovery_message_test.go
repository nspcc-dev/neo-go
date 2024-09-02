package consensus

import (
	"testing"

	"github.com/nspcc-dev/dbft"
	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestRecoveryMessageSetters(t *testing.T) {
	t.Run("NoStateRoot", func(t *testing.T) {
		testRecoveryMessageSetters(t, false)
	})
	t.Run("WithStateRoot", func(t *testing.T) {
		testRecoveryMessageSetters(t, true)
	})
}

func testRecoveryMessageSetters(t *testing.T, enableStateRoot bool) {
	srv := newTestServiceWithState(t, enableStateRoot)
	privs := make([]*privateKey, testchain.Size())
	pubs := make([]dbft.PublicKey, testchain.Size())
	for i := range testchain.Size() {
		privs[i], pubs[i] = getTestValidator(i)
	}

	const msgHeight = 10

	r := &recoveryMessage{stateRootEnabled: enableStateRoot}
	p := NewPayload(netmode.UnitTestNet, enableStateRoot)
	p.message.Type = messageType(dbft.RecoveryMessageType)
	p.BlockIndex = msgHeight
	p.payload = r
	// sign payload to have verification script
	require.NoError(t, p.Sign(privs[0]))

	req := &prepareRequest{
		timestamp:         87,
		transactionHashes: []util.Uint256{{1}},
		stateRootEnabled:  enableStateRoot,
	}
	p1 := NewPayload(netmode.UnitTestNet, enableStateRoot)
	p1.message.Type = messageType(dbft.PrepareRequestType)
	p1.BlockIndex = msgHeight
	p1.payload = req
	p1.message.ValidatorIndex = 0
	p1.Sender = privs[0].GetScriptHash()
	require.NoError(t, p1.Sign(privs[0]))

	t.Run("prepare response is added", func(t *testing.T) {
		p2 := NewPayload(netmode.UnitTestNet, enableStateRoot)
		p2.message.Type = messageType(dbft.PrepareResponseType)
		p2.BlockIndex = msgHeight
		p2.payload = &prepareResponse{
			preparationHash: p1.Hash(),
		}
		p2.message.ValidatorIndex = 1
		p2.Sender = privs[1].GetScriptHash()
		require.NoError(t, p2.Sign(privs[1]))

		r.AddPayload(p2)
		require.NotNil(t, r.PreparationHash())
		require.Equal(t, p1.Hash(), *r.PreparationHash())

		ps := r.GetPrepareResponses(p, pubs)
		require.Len(t, ps, 1)
		// Update hashes and serialized data.
		_ = ps[0].Hash()
		require.Equal(t, p2, ps[0])
		ps0 := ps[0].(*Payload)
		require.True(t, srv.validatePayload(ps0))
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
		require.True(t, srv.validatePayload(pl))
	})

	t.Run("change view is added", func(t *testing.T) {
		p3 := NewPayload(netmode.UnitTestNet, enableStateRoot)
		p3.message.Type = messageType(dbft.ChangeViewType)
		p3.BlockIndex = msgHeight
		p3.payload = &changeView{
			newViewNumber: 1,
			timestamp:     12345,
		}
		p3.message.ValidatorIndex = 3
		p3.Sender = privs[3].GetScriptHash()
		require.NoError(t, p3.Sign(privs[3]))

		r.AddPayload(p3)

		ps := r.GetChangeViews(p, pubs)
		require.Len(t, ps, 1)
		// update hashes and serialized data.
		_ = ps[0].Hash()
		require.Equal(t, p3, ps[0])

		ps0 := ps[0].(*Payload)
		require.True(t, srv.validatePayload(ps0))
	})

	t.Run("commit is added", func(t *testing.T) {
		p4 := NewPayload(netmode.UnitTestNet, enableStateRoot)
		p4.message.Type = messageType(dbft.CommitType)
		p4.BlockIndex = msgHeight
		p4.payload = randomMessage(t, commitType)
		p4.message.ValidatorIndex = 3
		p4.Sender = privs[3].GetScriptHash()
		require.NoError(t, p4.Sign(privs[3]))

		r.AddPayload(p4)

		ps := r.GetCommits(p, pubs)
		require.Len(t, ps, 1)
		// update hashes and serialized data.
		_ = ps[0].Hash()
		require.Equal(t, p4, ps[0])

		ps0 := ps[0].(*Payload)
		require.True(t, srv.validatePayload(ps0))
	})
}

/*
func TestRecoveryMessage_Decode(t *testing.T) {
	hexDump := "000000007f5b6094e1281e6bac667f1f871aee755dbe62c012868c718d7709de62135d250d1800000100fd0f024100000120003db64b5e000000008e4ab7138abe65a30133175ebcf3c66ad59ed2c532ca19bbb84cb3802f7dc9b6decde10e117ff6fc3303000041e52280e60c46778876e4c7fdcd262170d906090256ff2ac11d14d45516dd465b5b8f241ff78096ee7280f226df677681bff091884dcd7c4f25cd9a61856ce0bc6a01004136b0b971ef320135f61c11475ff07c5cad04635fc1dad41d346d085646e29e6ff1c5181421a203e5d4b627c6bacdd78a78c9f4cb0a749877ea5a9ed2b02196f17f020041ac5e279927ded591c234391078db55cad2ada58bded974fa2d2751470d0b2f94dddc84ed312f31ee960c884066f778e000f4f05883c74defa75d2a2eb524359c7d020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000041546d2e34cbbfd0d09b7ce937eec07b402bd597f7bef24938f4a01041f443fb4dd31bebcabdaae3942bb9d549724a152e851bee43ebc5f482ddd9316f2690b48e7d00010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000415281a6579b875c480d9b3cc9144485d1f898e13405eaf1e4117d83844f1265f81a71998d53fa32d6c3b5249446ac036ecda73b1fe8c1341475fcc4b8d6ba8ec6e20141d775fd1a605173a8ed02084fef903ee043239ca4c76cb658809c6216031437e8f4d5a265550d5934fe386732364d9b49a14baef5a1236d02c557cb394a3a0873c82364f65259a991768a35ba18777f76901e1022f87d71910f4e3e46f161299401f2074d0c"
	data, err := hex.DecodeString(hexDump)
	require.NoError(t, err)

	buf := io.NewBinReaderFromBuf(data)
	p := NewPayload(netmode.TestNet)
	p.DecodeBinary(buf)
	require.NoError(t, buf.Err)
	require.NoError(t, p.decodeData())
	require.Equal(t, payload.RecoveryMessageType, p.Type())
	require.NotNil(t, p.message.payload)
	req := p.message.payload.(*recoveryMessage).prepareRequest
	require.NotNil(t, req)
	require.Equal(t, prepareRequestType, p.message.payload.(*recoveryMessage).prepareRequest.Type)

	buf.ReadB()
	require.Equal(t, gio.EOF, buf.Err)
}
*/
