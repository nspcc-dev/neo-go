package network

import (
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/network/capability"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeVersion(t *testing.T) {
	// message with tiny payload, shouldn't be compressed
	expected := NewMessage(CMDVersion, &payload.Version{
		Magic:     1,
		Version:   2,
		Timestamp: uint32(time.Now().UnixNano()),
		Nonce:     987,
		UserAgent: []byte{1, 2, 3},
		Capabilities: capability.Capabilities{
			{
				Type: capability.FullNode,
				Data: &capability.Node{
					StartHeight: 123,
				},
			},
		},
	})
	testserdes.EncodeDecode(t, expected, &Message{})
	uncompressed, err := testserdes.EncodeBinary(expected.Payload)
	require.NoError(t, err)
	require.Equal(t, len(expected.compressedPayload), len(uncompressed))

	// large payload should be compressed
	largeArray := make([]byte, CompressionMinSize)
	for i := range largeArray {
		largeArray[i] = byte(i)
	}
	expected.Payload.(*payload.Version).UserAgent = largeArray
	testserdes.EncodeDecode(t, expected, &Message{})
	uncompressed, err = testserdes.EncodeBinary(expected.Payload)
	require.NoError(t, err)
	require.NotEqual(t, len(expected.compressedPayload), len(uncompressed))
}

func TestEncodeDecodeHeaders(t *testing.T) {
	// shouldn't try to compress headers payload
	headers := &payload.Headers{Hdrs: make([]*block.Header, CompressionMinSize)}
	for i := range headers.Hdrs {
		h := &block.Header{
			Base: block.Base{
				Index: uint32(i + 1),
				Script: transaction.Witness{
					InvocationScript:   []byte{0x0},
					VerificationScript: []byte{0x1},
				},
			},
		}
		h.Hash()
		headers.Hdrs[i] = h
	}
	expected := NewMessage(CMDHeaders, headers)
	testserdes.EncodeDecode(t, expected, &Message{})
	uncompressed, err := testserdes.EncodeBinary(expected.Payload)
	require.NoError(t, err)
	require.Equal(t, len(expected.compressedPayload), len(uncompressed))
}

func TestEncodeDecodeGetAddr(t *testing.T) {
	// NullPayload should be handled properly
	expected := NewMessage(CMDGetAddr, payload.NewNullPayload())
	testserdes.EncodeDecode(t, expected, &Message{})
}

func TestEncodeDecodeNil(t *testing.T) {
	// nil payload should be decoded into NullPayload
	expected := NewMessage(CMDGetAddr, nil)
	encoded, err := testserdes.Encode(expected)
	require.NoError(t, err)
	decoded := &Message{}
	err = testserdes.Decode(encoded, decoded)
	require.NoError(t, err)
	require.Equal(t, NewMessage(CMDGetAddr, payload.NewNullPayload()), decoded)
}

func TestEncodeDecodePing(t *testing.T) {
	expected := NewMessage(CMDPing, payload.NewPing(123, 456))
	testserdes.EncodeDecode(t, expected, &Message{})
}

func TestEncodeDecodeInventory(t *testing.T) {
	expected := NewMessage(CMDInv, payload.NewInventory(payload.ConsensusType, []util.Uint256{{1, 2, 3}}))
	testserdes.EncodeDecode(t, expected, &Message{})
}
