package entities

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/testutil"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeNotificationEvent(t *testing.T) {
	event := &NotificationEvent{
		ScriptHash: testutil.RandomUint160(),
		Item:       nil,
	}

	buf := io.NewBufBinWriter()
	event.EncodeBinary(buf.BinWriter)
	assert.Nil(t, buf.Err)

	eventDecoded := &NotificationEvent{}
	reader := io.NewBinReaderFromBuf(buf.Bytes())
	eventDecoded.DecodeBinary(reader)
	assert.Equal(t, event, eventDecoded)
}

func TestEncodeDecodeAppExecResult(t *testing.T) {
	appExecResult := &AppExecResult{
		TxHash:      testutil.RandomUint256(),
		Trigger:     1,
		VMState:     "Hault",
		GasConsumed: 10,
		Stack:       "",
		Events:      []NotificationEvent{},
	}
	buf := io.NewBufBinWriter()
	appExecResult.EncodeBinary(buf.BinWriter)
	assert.Nil(t, buf.Err)

	appExecResultDecoded := &AppExecResult{}
	reader := io.NewBinReaderFromBuf(buf.Bytes())
	appExecResultDecoded.DecodeBinary(reader)
	assert.Equal(t, appExecResult, appExecResultDecoded)
}
