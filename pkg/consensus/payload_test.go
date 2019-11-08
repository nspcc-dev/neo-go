package consensus

import (
	"encoding/hex"
	gio "io"
	"math/rand"
	"testing"
	"time"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
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
	}
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
		Version:        1,
		ValidatorIndex: 13,
		Height:         rand.Uint32(),
		Timestamp:      rand.Uint32(),
		Witness: transaction.Witness{
			InvocationScript:   fillRandom(t, make([]byte, 3)),
			VerificationScript: fillRandom(t, make([]byte, 4)),
		},
	}
	fillRandom(t, p.PrevHash[:])

	if mt == changeViewType {
		p.payload.(*changeView).NewViewNumber = p.ViewNumber + 1
	}

	return p
}

func randomMessage(t *testing.T, mt messageType) io.Serializable {
	switch mt {
	case changeViewType:
		return &changeView{
			Timestamp: rand.Uint32(),
		}
	case prepareRequestType:
		return randomPrepareRequest(t)
	case prepareResponseType:
		resp := &prepareResponse{}
		fillRandom(t, resp.PreparationHash[:])
		return resp
	case commitType:
		var c commit
		fillRandom(t, c.Signature[:])
		return &c
	case recoveryRequestType:
		return &recoveryRequest{Timestamp: rand.Uint32()}
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
		Timestamp:         rand.Uint32(),
		Nonce:             rand.Uint64(),
		TransactionHashes: make([]util.Uint256, txCount),
		MinerTransaction:  *newMinerTx(rand.Uint32()),
	}

	req.TransactionHashes[0] = req.MinerTransaction.Hash()
	for i := 1; i < txCount; i++ {
		fillRandom(t, req.TransactionHashes[i][:])
	}
	fillRandom(t, req.NextConsensus[:])

	return req
}

func randomRecoveryMessage(t *testing.T) *recoveryMessage {
	result := randomMessage(t, prepareRequestType)
	require.IsType(t, (*prepareRequest)(nil), result)
	prepReq := result.(*prepareRequest)

	return &recoveryMessage{
		PreparationPayloads: []*preparationCompact{
			{
				ValidatorIndex:   1,
				InvocationScript: fillRandom(t, make([]byte, 10)),
			},
		},
		CommitPayloads: []*commitCompact{
			{
				ViewNumber:       0,
				ValidatorIndex:   1,
				Signature:        fillRandom(t, make([]byte, signatureSize)),
				InvocationScript: fillRandom(t, make([]byte, 20)),
			},
			{
				ViewNumber:       0,
				ValidatorIndex:   2,
				Signature:        fillRandom(t, make([]byte, signatureSize)),
				InvocationScript: fillRandom(t, make([]byte, 10)),
			},
		},
		ChangeViewPayloads: []*changeViewCompact{
			{
				Timestamp:          rand.Uint32(),
				ValidatorIndex:     3,
				OriginalViewNumber: 3,
				InvocationScript:   fillRandom(t, make([]byte, 4)),
			},
		},
		PrepareRequest: prepReq,
	}
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
		Attributes: []*transaction.Attribute{},
		Inputs:     []*transaction.Input{},
		Outputs:    []*transaction.Output{},
		Scripts:    []*transaction.Witness{},
		Trimmed:    false,
	}
}
