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
	// signed payload from mixed privnet (Go + 3C# nodes)
	dataHex := "000000004c02d52305a6a8981bd1598f0c3076d6de15a44a60ca692e189cd8a7249f175c0800000003222100f24b9147a21e09562c68abdec56d3c5fc09936592933aea5692800b75edbab2301420c40b2b8080ab02b703bc4e64407a6f31bb7ae4c9b1b1c8477668afa752eba6148e03b3ffc7e06285c09bdce4582188466209f876c38f9921a88b545393543ab201a290c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee6990b4195440d78"
	data, err := hex.DecodeString(dataHex)
	require.NoError(t, err)

	h, err := util.Uint160DecodeStringLE("a8826043c40abacfac1d9acc6b92a4458308ca18")
	require.NoError(t, err)

	p := NewPayload(netmode.PrivNet, false)
	require.NoError(t, testserdes.DecodeBinary(data, p))
	require.NoError(t, p.decodeData())
	bc := newTestChain(t, false)
	defer bc.Close()
	require.NoError(t, bc.VerifyWitness(h, p, &p.Witness, payloadGasLimit))
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
		actual.message = new(message)
		// message should now be decoded from actual.data byte array
		actual.message = new(message)
		assert.NoError(t, actual.decodeData())
		assert.NotNil(t, actual.MarshalUnsigned())
		require.Equal(t, p, actual)

		data = p.MarshalUnsigned()
		pu := NewPayload(netmode.Magic(rand.Uint32()), false)
		require.NoError(t, pu.UnmarshalUnsigned(data))
		assert.NoError(t, pu.decodeData())
		_ = pu.MarshalUnsigned()

		p.Witness = transaction.Witness{}
		require.Equal(t, p, pu)
	}
}

func TestConsensusPayload_DecodeBinaryInvalid(t *testing.T) {
	// PrepareResponse ConsensusPayload consists of:
	// 41-byte common prefix
	// 1-byte varint length of the payload (34),
	// - 1-byte view number
	// - 1-byte message type (PrepareResponse)
	// - 32-byte preparation hash
	// 1-byte delimiter (1)
	// 2-byte for empty invocation and verification scripts
	const (
		lenIndex       = 41
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
	p := &Payload{message: new(message)}
	require.NoError(t, testserdes.DecodeBinary(buf, p))
	// decode `data` into `message`
	_ = p.Hash()
	assert.NoError(t, p.decodeData())
	require.Equal(t, expected, p)

	// invalid type
	buf[typeIndex] = 0xFF
	actual := &Payload{message: new(message)}
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
	defer bc.Close()
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
	hexDump := "000000004c02d52305a6a8981bd1598f0c3076d6de15a44a60ca692e189cd8a7249f175c08000000004230000368c5c5401d40eef6b8a9899d2041d29fd2e6300980fdcaa6660c10b85965f57852193cdb6f0d1e9f91dc510dff6df3a004b569fe2ad456d07007f6ccd55b1d01420c40e760250b821a4dcfc4b8727ecc409a758ab4bd3b288557fd3c3d76e083fe7c625b4ed25e763ad96c4eb0abc322600d82651fd32f8866fca1403fa04d3acc4675290c2102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e0b4195440d78"
	data, err := hex.DecodeString(hexDump)
	require.NoError(t, err)

	buf := io.NewBinReaderFromBuf(data)
	p := NewPayload(netmode.PrivNet, false)
	p.DecodeBinary(buf)
	require.NoError(t, buf.Err)
	require.NoError(t, p.decodeData())
	require.Equal(t, payload.CommitType, p.Type())
	require.Equal(t, uint32(8), p.Height())

	buf.ReadB()
	require.Equal(t, gio.EOF, buf.Err)
}
