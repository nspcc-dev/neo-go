package consensus

import (
	"math/rand"
	"testing"

	"github.com/nspcc-dev/dbft/payload"
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

//TODO NEO3.0: Update binary
/*
func TestConsensusPayload_Hash(t *testing.T) {
	dataHex := "00000000d8fb8d3b143b5f98468ef701909c976505a110a01e26c5e99be9a90cff979199b6fc33000400000000008d2000184dc95de24018f9ad71f4448a2b438eaca8b4b2ab6b4524b5a69a45d920c35103f3901444320656c390ff39c0062f5e8e138ce446a40c7e4ba1af1f8247ebbdf49295933715d3a67949714ff924f8a28cec5b954c71eca3bfaf0e9d4b1f87b4e21e9ba4ae18f97de71501b5c5d07edc200bd66a46b9b28b1a371f2195c10b0af90000e24018f900000000014140c9faaee59942f58da0e5268bc199632f2a3ad0fcbee68681a4437f140b49512e8d9efc6880eb44d3490782895a5794f35eeccee2923ce0c76fa7a1890f934eac232103c089d7122b840a4935234e82e26ae5efd0c2acb627239dc9f207311337b6f2c1ac"
	data, err := hex.DecodeString(dataHex)
	require.NoError(t, err)

	var p Payload
	require.NoError(t, testserdes.DecodeBinary(data, &p))
	require.Equal(t, p.Hash().String(), "45859759c8491597804f1922773947e0d37bf54484af82f80cd642f7b063aa56")
}
*/
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
		pu := new(Payload)
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
			timestamp: rand.Uint32(),
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
		return &recoveryRequest{timestamp: rand.Uint32()}
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
		minerTx:           *transaction.NewMinerTX(),
	}

	req.transactionHashes[0] = req.minerTx.Hash()
	for i := 1; i < txCount; i++ {
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
				Timestamp:          rand.Uint32(),
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
