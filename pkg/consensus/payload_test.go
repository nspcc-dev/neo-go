package consensus

import (
	"encoding/hex"
	"math/rand"
	"testing"

	"github.com/nspcc-dev/dbft/payload"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/io"
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
	p.message = &message{}

	p.SetVersion(1)
	assert.EqualValues(t, 1, p.Version())

	p.SetPrevHash(util.Uint256{1, 2, 3})
	assert.Equal(t, util.Uint256{1, 2, 3}, p.PrevHash())

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

func TestConsensusPayload_Verify(t *testing.T) {
	// signed payload from testnet
	dataHex := "00000000a70b769e4af60878f6daa72be41770c62592c694bf9ead6b16b30ad90f28c4098cc704000400423000d5b4baae11191ac370a4d7860df01824fcea7f934d6461db6d4b7966ca3c135c8c262b7f23bbac13e73885223604141e062234d999068d9a74b77caeeb5271cf01420c4055ae8c7694c296e92da393f944b0dc1cd70d12de3ee944e9afc872d1db427fe87fcbe913709a8ec73e2f5acdfc0b7f0a96e9d63bad0a20e3226c882237f5c771290c2102a7834be9b32e2981d157cb5bbd3acb42cfd11ea5c3b10224d7a44e98c5910f1b0b410a906ad4"
	data, err := hex.DecodeString(dataHex)
	require.NoError(t, err)

	h, err := util.Uint160DecodeStringBE("31b7e7aea5131f74721e002c6a56b6885813f79e")
	require.NoError(t, err)

	p := NewPayload(netmode.TestNet)
	require.NoError(t, testserdes.DecodeBinary(data, p))
	require.NoError(t, p.decodeData())
	require.True(t, p.Verify(h))
}

func TestConsensusPayload_Serializable(t *testing.T) {
	for _, mt := range messageTypes {
		p := randomPayload(t, mt)
		actual := new(Payload)
		data, err := testserdes.EncodeBinary(p)
		require.NoError(t, err)
		require.NoError(t, testserdes.DecodeBinary(data, actual))
		// message is nil after decoding as we didn't yet call decodeData
		require.Nil(t, actual.message)
		// message should now be decoded from actual.data byte array
		assert.NoError(t, actual.decodeData())
		require.Equal(t, p, actual)

		data = p.MarshalUnsigned()
		pu := NewPayload(netmode.Magic(rand.Uint32()))
		require.NoError(t, pu.UnmarshalUnsigned(data))
		assert.NoError(t, pu.decodeData())

		p.Witness = transaction.Witness{}
		require.Equal(t, p, pu)
	}
}

func TestConsensusPayload_DecodeBinaryInvalid(t *testing.T) {
	// PrepareResponse ConsensusPayload consists of:
	// 42-byte common prefix
	// 1-byte varint length of the payload (34),
	// - 1-byte view number
	// - 1-byte message type (PrepareResponse)
	// - 32-byte preparation hash
	// 1-byte delimiter (1)
	// 2-byte for empty invocation and verification scripts
	const (
		lenIndex       = 42
		typeIndex      = lenIndex + 1
		delimeterIndex = typeIndex + 34
	)

	buf := make([]byte, delimeterIndex+1+2)

	expected := &Payload{
		message: &message{
			Type:    prepareResponseType,
			payload: &prepareResponse{},
		},
		Witness: transaction.Witness{
			InvocationScript:   []byte{},
			VerificationScript: []byte{},
		},
	}
	// fill `data` for next check
	_ = expected.Hash()

	// valid payload
	buf[delimeterIndex] = 1
	buf[lenIndex] = 34
	buf[typeIndex] = byte(prepareResponseType)
	p := new(Payload)
	require.NoError(t, testserdes.DecodeBinary(buf, p))
	// decode `data` into `message`
	assert.NoError(t, p.decodeData())
	require.Equal(t, expected, p)

	// invalid type
	buf[typeIndex] = 0xFF
	actual := new(Payload)
	require.NoError(t, testserdes.DecodeBinary(buf, actual))
	require.Error(t, actual.decodeData())

	// invalid format
	buf[delimeterIndex] = 0
	buf[typeIndex] = byte(prepareResponseType)
	require.Error(t, testserdes.DecodeBinary(buf, new(Payload)))

	// invalid message length
	buf[delimeterIndex] = 1
	buf[lenIndex] = 0xFF
	buf[typeIndex] = byte(prepareResponseType)
	require.Error(t, testserdes.DecodeBinary(buf, new(Payload)))
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
		message: &message{
			Type:       mt,
			ViewNumber: byte(rand.Uint32()),
			payload:    randomMessage(t, mt),
		},
		version:        1,
		validatorIndex: 13,
		height:         rand.Uint32(),
		prevHash:       random.Uint256(),
		Witness: transaction.Witness{
			InvocationScript:   random.Bytes(3),
			VerificationScript: []byte{byte(opcode.PUSH0)},
		},
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
		nonce:             rand.Uint64(),
		transactionHashes: make([]util.Uint256, txCount),
	}

	for i := 0; i < txCount; i++ {
		req.transactionHashes[i] = random.Uint256()
	}
	req.nextConsensus = random.Uint160()

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
	require.False(t, p.Verify(util.Uint160{}))
	require.NoError(t, p.Sign(priv))
	require.True(t, p.Verify(p.Witness.ScriptHash()))
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
