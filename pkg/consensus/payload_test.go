package consensus

import (
	"encoding/hex"
	gio "io"
	"math/rand"
	"testing"

	"github.com/nspcc-dev/dbft/payload"
	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	npayload "github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var messageTypes = []messageType{
	changeViewType,
	prepareRequestType,
	prepareResponseType,
	commitType,
	recoveryRequestType,
	recoveryMessageType,
}

func TestConsensusPayload_Setters(t *testing.T) {
	var p Payload

	//p.SetVersion(1)
	//assert.EqualValues(t, 1, p.Version())

	//p.SetPrevHash(util.Uint256{1, 2, 3})
	//assert.Equal(t, util.Uint256{1, 2, 3}, p.PrevHash())

	p.SetValidatorIndex(4)
	assert.EqualValues(t, 4, p.ValidatorIndex())

	p.SetHeight(11)
	assert.EqualValues(t, 11, p.Height())

	p.SetViewNumber(2)
	assert.EqualValues(t, 2, p.ViewNumber())

	p.SetType(payload.PrepareRequestType)
	assert.Equal(t, payload.PrepareRequestType, p.Type())

	pl := randomMessage(t, prepareRequestType)
	p.SetPayload(pl)
	require.Equal(t, pl, p.Payload())
	require.Equal(t, pl, p.GetPrepareRequest())

	pl = randomMessage(t, prepareResponseType)
	p.SetPayload(pl)
	require.Equal(t, pl, p.GetPrepareResponse())

	pl = randomMessage(t, commitType)
	p.SetPayload(pl)
	require.Equal(t, pl, p.GetCommit())

	pl = randomMessage(t, changeViewType)
	p.SetPayload(pl)
	require.Equal(t, pl, p.GetChangeView())

	pl = randomMessage(t, recoveryRequestType)
	p.SetPayload(pl)
	require.Equal(t, pl, p.GetRecoveryRequest())

	pl = randomMessage(t, recoveryMessageType)
	p.SetPayload(pl)
	require.Equal(t, pl, p.GetRecoveryMessage())
}

func TestConsensusPayload_Serializable(t *testing.T) {
	for _, mt := range messageTypes {
		p := randomPayload(t, mt)
		actual := &Payload{Extensible: npayload.Extensible{}, network: netmode.UnitTestNet}
		data, err := testserdes.EncodeBinary(p)
		require.NoError(t, err)
		require.NoError(t, testserdes.DecodeBinary(data, &actual.Extensible))
		assert.NoError(t, actual.decodeData())
		require.Equal(t, p, actual)
	}
}

func TestConsensusPayload_DecodeBinaryInvalid(t *testing.T) {
	// PrepareResponse payload consists of:
	// - 1-byte message type (PrepareResponse)
	// - 4-byte block index
	// - 1-byte validator index
	// - 1-byte view number
	// - 32-byte preparation hash
	const (
		typeIndex = 0
		size      = 39
	)

	buf := make([]byte, size)
	expected := message{
		Type:    prepareResponseType,
		payload: &prepareResponse{},
	}

	// valid payload
	buf[typeIndex] = byte(prepareResponseType)
	p := &Payload{Extensible: npayload.Extensible{Data: buf}}
	require.NoError(t, p.decodeData())
	require.Equal(t, expected, p.message)

	// invalid type
	buf[typeIndex] = 0xFF
	p = &Payload{Extensible: npayload.Extensible{Data: buf}}
	require.Error(t, p.decodeData())

	// invalid length
	buf[typeIndex] = byte(prepareResponseType)
	p = &Payload{Extensible: npayload.Extensible{Data: buf[:len(buf)-1]}}
	require.Error(t, p.decodeData())
}

func TestCommit_Serializable(t *testing.T) {
	c := randomMessage(t, commitType)
	testserdes.EncodeDecodeBinary(t, c, new(commit))
}

func TestPrepareResponse_Serializable(t *testing.T) {
	resp := randomMessage(t, prepareResponseType)
	testserdes.EncodeDecodeBinary(t, resp, new(prepareResponse))
}

func TestPrepareRequest_Serializable(t *testing.T) {
	req := randomMessage(t, prepareRequestType)
	testserdes.EncodeDecodeBinary(t, req, new(prepareRequest))
}

func TestRecoveryRequest_Serializable(t *testing.T) {
	req := randomMessage(t, recoveryRequestType)
	testserdes.EncodeDecodeBinary(t, req, new(recoveryRequest))
}

func TestRecoveryMessage_Serializable(t *testing.T) {
	msg := randomMessage(t, recoveryMessageType)
	testserdes.EncodeDecodeBinary(t, msg, new(recoveryMessage))
}

func randomPayload(t *testing.T, mt messageType) *Payload {
	p := &Payload{
		message: message{
			Type:           mt,
			ValidatorIndex: byte(rand.Uint32()),
			BlockIndex:     rand.Uint32(),
			ViewNumber:     byte(rand.Uint32()),
			payload:        randomMessage(t, mt),
		},
		Extensible: npayload.Extensible{
			Witness: transaction.Witness{
				InvocationScript:   random.Bytes(3),
				VerificationScript: []byte{byte(opcode.PUSH0)},
			},
		},
		network: netmode.UnitTestNet,
	}

	if mt == changeViewType {
		p.payload.(*changeView).newViewNumber = p.ViewNumber() + 1
	}

	return p
}

func randomMessage(t *testing.T, mt messageType) io.Serializable {
	switch mt {
	case changeViewType:
		return &changeView{
			timestamp: rand.Uint64(),
		}
	case prepareRequestType:
		return randomPrepareRequest(t)
	case prepareResponseType:
		return &prepareResponse{preparationHash: random.Uint256()}
	case commitType:
		var c commit
		random.Fill(c.signature[:])
		return &c
	case recoveryRequestType:
		return &recoveryRequest{timestamp: rand.Uint64()}
	case recoveryMessageType:
		return randomRecoveryMessage(t)
	default:
		require.Fail(t, "invalid type")
		return nil
	}
}

func randomPrepareRequest(t *testing.T) *prepareRequest {
	const txCount = 3

	req := &prepareRequest{
		timestamp:         rand.Uint64(),
		transactionHashes: make([]util.Uint256, txCount),
	}

	for i := 0; i < txCount; i++ {
		req.transactionHashes[i] = random.Uint256()
	}

	return req
}

func randomRecoveryMessage(t *testing.T) *recoveryMessage {
	result := randomMessage(t, prepareRequestType)
	require.IsType(t, (*prepareRequest)(nil), result)
	prepReq := result.(*prepareRequest)

	return &recoveryMessage{
		preparationPayloads: []*preparationCompact{
			{
				ValidatorIndex:   1,
				InvocationScript: random.Bytes(10),
			},
		},
		commitPayloads: []*commitCompact{
			{
				ViewNumber:       0,
				ValidatorIndex:   1,
				Signature:        [64]byte{1, 2, 3},
				InvocationScript: random.Bytes(20),
			},
			{
				ViewNumber:       0,
				ValidatorIndex:   2,
				Signature:        [64]byte{11, 3, 4, 98},
				InvocationScript: random.Bytes(10),
			},
		},
		changeViewPayloads: []*changeViewCompact{
			{
				Timestamp:          rand.Uint64(),
				ValidatorIndex:     3,
				OriginalViewNumber: 3,
				InvocationScript:   random.Bytes(4),
			},
		},
		prepareRequest: &message{
			Type:    prepareRequestType,
			payload: prepReq,
		},
	}
}

func TestPayload_Sign(t *testing.T) {
	key, err := keys.NewPrivateKey()
	require.NoError(t, err)

	priv := &privateKey{key}

	p := randomPayload(t, prepareRequestType)
	h := priv.PublicKey().GetScriptHash()
	bc := newTestChain(t, false)
	require.Error(t, bc.VerifyWitness(h, p, &p.Witness, payloadGasLimit))
	require.NoError(t, p.Sign(priv))
	require.NoError(t, bc.VerifyWitness(h, p, &p.Witness, payloadGasLimit))
}

func TestMessageType_String(t *testing.T) {
	require.Equal(t, "ChangeView", changeViewType.String())
	require.Equal(t, "PrepareRequest", prepareRequestType.String())
	require.Equal(t, "PrepareResponse", prepareResponseType.String())
	require.Equal(t, "Commit", commitType.String())
	require.Equal(t, "RecoveryMessage", recoveryMessageType.String())
	require.Equal(t, "RecoveryRequest", recoveryRequestType.String())
	require.Equal(t, "UNKNOWN(0xff)", messageType(0xff).String())
}

func TestPayload_DecodeFromPrivnet(t *testing.T) {
	hexDump := "0464424654000000000200000018ca088345a4926bcc9a1daccfba0ac4436082a847300200000003003c57d952539c5e0c39a83c0de5744a772c0dcb0e8ccd7c5bba27ef" +
		"498506cd860cdfd01ad215b251ab64dc64cd544a6f453f3b0128ddc98d95ac15915dbe6f6301420c40b39c9136af3b8186409dec2dbd31d0fd4f3e637b3eeb96d8556b41f8512dd25d91134f62a6c293db089b7e82b7a0fd23bf9a1" +
		"5ee26c42a5738b913beef74176d290c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee6990b4195440d78"
	data, err := hex.DecodeString(hexDump)
	require.NoError(t, err)

	buf := io.NewBinReaderFromBuf(data)
	p := NewPayload(netmode.PrivNet, false)
	p.DecodeBinary(buf)
	require.NoError(t, buf.Err)
	require.Equal(t, payload.CommitType, p.Type())
	require.Equal(t, uint32(2), p.Height())
	require.Equal(t, uint16(3), p.ValidatorIndex())
	require.Equal(t, byte(0), p.ViewNumber())
	require.NotNil(t, p.message.payload)

	buf.ReadB()
	require.Equal(t, gio.EOF, buf.Err)
}
