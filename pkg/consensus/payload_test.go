package consensus

import (
	"encoding/hex"
	gio "io"
	"math/rand"
	"testing"
	"time"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/nspcc-dev/dbft/payload"
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

func TestConsensusPayload_Hash(t *testing.T) {
	dataHex := "00000000d8fb8d3b143b5f98468ef701909c976505a110a01e26c5e99be9a90cff979199b6fc33000400000000008d2000184dc95de24018f9ad71f4448a2b438eaca8b4b2ab6b4524b5a69a45d920c35103f3901444320656c390ff39c0062f5e8e138ce446a40c7e4ba1af1f8247ebbdf49295933715d3a67949714ff924f8a28cec5b954c71eca3bfaf0e9d4b1f87b4e21e9ba4ae18f97de71501b5c5d07edc200bd66a46b9b28b1a371f2195c10b0af90000e24018f900000000014140c9faaee59942f58da0e5268bc199632f2a3ad0fcbee68681a4437f140b49512e8d9efc6880eb44d3490782895a5794f35eeccee2923ce0c76fa7a1890f934eac232103c089d7122b840a4935234e82e26ae5efd0c2acb627239dc9f207311337b6f2c1ac"
	data, err := hex.DecodeString(dataHex)
	require.NoError(t, err)

	r := io.NewBinReaderFromBuf(data)

	var p Payload
	p.DecodeBinary(r)

	require.NoError(t, err)
	require.Equal(t, p.Hash().String(), "45859759c8491597804f1922773947e0d37bf54484af82f80cd642f7b063aa56")
}

func TestConsensusPayload_Serializable(t *testing.T) {
	for _, mt := range messageTypes {
		p := randomPayload(t, mt)
		testSerializable(t, p, new(Payload))

		data := p.MarshalUnsigned()
		pu := new(Payload)
		require.NoError(t, pu.UnmarshalUnsigned(data))

		p.Witness = transaction.Witness{}
		require.Equal(t, p, pu)
	}
}

func TestConsensusPayload_DecodeBinaryInvalid(t *testing.T) {
	// PrepareResponse ConsensusPayload consists of:
	// 46-byte common prefix
	// 1-byte varint length of the payload (34),
	// - 1-byte view number
	// - 1-byte message type (PrepareResponse)
	// - 32-byte preparation hash
	// 1-byte delimiter (1)
	// 2-byte for empty invocation and verification scripts
	const (
		lenIndex       = 46
		typeIndex      = 47
		delimeterIndex = 81
	)

	buf := make([]byte, 46+1+34+1+2)

	expected := &Payload{
		message: message{
			Type:    prepareResponseType,
			payload: &prepareResponse{},
		},
		Witness: transaction.Witness{
			InvocationScript:   []byte{},
			VerificationScript: []byte{},
		},
	}

	// valid payload
	buf[delimeterIndex] = 1
	buf[lenIndex] = 34
	buf[typeIndex] = byte(prepareResponseType)
	r := io.NewBinReaderFromBuf(buf)
	p := new(Payload)
	p.DecodeBinary(r)
	require.NoError(t, r.Err)
	require.Equal(t, expected, p)

	// invalid type
	buf[typeIndex] = 0xFF
	r = io.NewBinReaderFromBuf(buf)
	new(Payload).DecodeBinary(r)
	require.Error(t, r.Err)

	// invalid format
	buf[delimeterIndex] = 0
	buf[typeIndex] = byte(prepareResponseType)
	r = io.NewBinReaderFromBuf(buf)
	new(Payload).DecodeBinary(r)
	require.Error(t, r.Err)

	// invalid message length
	buf[delimeterIndex] = 1
	buf[lenIndex] = 0xFF
	buf[typeIndex] = byte(prepareResponseType)
	r = io.NewBinReaderFromBuf(buf)
	new(Payload).DecodeBinary(r)
	require.Error(t, r.Err)
}

func TestCommit_Serializable(t *testing.T) {
	c := randomMessage(t, commitType)
	testSerializable(t, c, new(commit))
}

func TestPrepareResponse_Serializable(t *testing.T) {
	resp := randomMessage(t, prepareResponseType)
	testSerializable(t, resp, new(prepareResponse))
}

func TestPrepareRequest_Serializable(t *testing.T) {
	req := randomMessage(t, prepareRequestType)
	testSerializable(t, req, new(prepareRequest))
}

func TestRecoveryRequest_Serializable(t *testing.T) {
	req := randomMessage(t, recoveryRequestType)
	testSerializable(t, req, new(recoveryRequest))
}

func TestRecoveryMessage_Serializable(t *testing.T) {
	msg := randomMessage(t, recoveryMessageType)
	testSerializable(t, msg, new(recoveryMessage))
}

func randomPayload(t *testing.T, mt messageType) *Payload {
	p := &Payload{
		message: message{
			Type:       mt,
			ViewNumber: byte(rand.Uint32()),
			payload:    randomMessage(t, mt),
		},
		version:        1,
		validatorIndex: 13,
		height:         rand.Uint32(),
		timestamp:      rand.Uint32(),
		Witness: transaction.Witness{
			InvocationScript:   fillRandom(t, make([]byte, 3)),
			VerificationScript: fillRandom(t, make([]byte, 4)),
		},
	}
	fillRandom(t, p.prevHash[:])

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
		resp := &prepareResponse{}
		fillRandom(t, resp.preparationHash[:])
		return resp
	case commitType:
		var c commit
		fillRandom(t, c.signature[:])
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
		timestamp:         rand.Uint32(),
		nonce:             rand.Uint64(),
		transactionHashes: make([]util.Uint256, txCount),
		minerTx:           *newMinerTx(rand.Uint32()),
	}

	req.transactionHashes[0] = req.minerTx.Hash()
	for i := 1; i < txCount; i++ {
		fillRandom(t, req.transactionHashes[i][:])
	}
	fillRandom(t, req.nextConsensus[:])

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
				InvocationScript: fillRandom(t, make([]byte, 10)),
			},
		},
		commitPayloads: []*commitCompact{
			{
				ViewNumber:       0,
				ValidatorIndex:   1,
				Signature:        [64]byte{1, 2, 3},
				InvocationScript: fillRandom(t, make([]byte, 20)),
			},
			{
				ViewNumber:       0,
				ValidatorIndex:   2,
				Signature:        [64]byte{11, 3, 4, 98},
				InvocationScript: fillRandom(t, make([]byte, 10)),
			},
		},
		changeViewPayloads: []*changeViewCompact{
			{
				Timestamp:          rand.Uint32(),
				ValidatorIndex:     3,
				OriginalViewNumber: 3,
				InvocationScript:   fillRandom(t, make([]byte, 4)),
			},
		},
		prepareRequest: prepReq,
	}
}

func TestPayload_Sign(t *testing.T) {
	key, err := keys.NewPrivateKey()
	require.NoError(t, err)

	priv := &privateKey{key}
	p := randomPayload(t, prepareRequestType)
	require.False(t, p.Verify())
	require.NoError(t, p.Sign(priv))
	require.True(t, p.Verify())
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

func testSerializable(t *testing.T, expected, actual io.Serializable) {
	w := io.NewBufBinWriter()
	expected.EncodeBinary(w.BinWriter)

	r := io.NewBinReaderFromBuf(w.Bytes())
	actual.DecodeBinary(r)

	require.Equal(t, expected, actual)
}

func fillRandom(t *testing.T, buf []byte) []byte {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	_, err := gio.ReadFull(r, buf)
	require.NoError(t, err)

	return buf
}

func newMinerTx(nonce uint32) *transaction.Transaction {
	return &transaction.Transaction{
		Type:    transaction.MinerType,
		Version: 0,
		Data: &transaction.MinerTX{
			Nonce: rand.Uint32(),
		},
		Attributes: []transaction.Attribute{},
		Inputs:     []transaction.Input{},
		Outputs:    []transaction.Output{},
		Scripts:    []transaction.Witness{},
		Trimmed:    false,
	}
}
