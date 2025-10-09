package network

import (
	"errors"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/capability"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestMessageDecodeFuzzCases(t *testing.T) {
	raw := []byte("10\x0200")
	m := new(Message)
	r := io.NewBinReaderFromBuf(raw)
	require.NotPanics(t, func() { _ = m.Decode(r) })
}

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

	// large payload should be compressed...
	largeArray := make([]byte, CompressionMinSize)
	for i := range largeArray {
		largeArray[i] = byte(i)
	}
	expected.Payload.(*payload.Version).UserAgent = largeArray
	testserdes.EncodeDecode(t, expected, &Message{})
	uncompressed, err = testserdes.EncodeBinary(expected.Payload)
	require.NoError(t, err)
	require.True(t, expected.Flags&Compressed != 0)
	require.NotEqual(t, len(expected.compressedPayload), len(uncompressed))

	// ... only if compression is not prohibited.
	expected.Flags ^= Compressed // reset compression state
	compressedBytes, err := testserdes.Encode(expected)
	require.NoError(t, err)
	expected.Flags = 0 // reset compression state
	buf := io.NewBufBinWriter()
	require.NoError(t, expected.EncodeCompressed(buf.BinWriter, false))
	require.True(t, expected.Flags&Compressed == 0)
	require.NotEqual(t, compressedBytes, buf.Bytes())

	// large UserAgent is not allowed
	expected.Payload.(*payload.Version).UserAgent = append(largeArray, 1)
	data, err := testserdes.Encode(expected)
	require.NoError(t, err)
	err = testserdes.Decode(data, &Message{})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "byte-slice is too big"), err)
}

func BenchmarkMessageBytes(b *testing.B) {
	// shouldn't try to compress headers payload
	ep := &payload.Extensible{
		Category:        "consensus",
		ValidBlockStart: rand.Uint32(),
		ValidBlockEnd:   rand.Uint32(),
		Sender:          util.Uint160{},
		Data:            make([]byte, 300),
		Witness: transaction.Witness{
			InvocationScript:   make([]byte, 33),
			VerificationScript: make([]byte, 40),
		},
	}
	random.Fill(ep.Data)
	random.Fill(ep.Witness.InvocationScript)
	random.Fill(ep.Witness.VerificationScript)
	msg := NewMessage(CMDExtensible, ep)

	b.ReportAllocs()
	for b.Loop() {
		_, err := msg.Bytes()
		if err != nil {
			b.FailNow()
		}
	}
}

func TestEncodeDecodeHeaders(t *testing.T) {
	// shouldn't try to compress headers payload
	headers := &payload.Headers{Hdrs: make([]*block.Header, CompressionMinSize)}
	for i := range headers.Hdrs {
		h := &block.Header{
			Index: uint32(i + 1),
			Script: transaction.Witness{
				InvocationScript:   []byte{0x0},
				VerificationScript: []byte{0x1},
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
	testEncodeDecode(t, CMDGetAddr, payload.NewNullPayload())
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
	testEncodeDecode(t, CMDPing, payload.NewPing(123, 456))
}

func TestEncodeDecodeInventory(t *testing.T) {
	testEncodeDecode(t, CMDInv, payload.NewInventory(payload.ExtensibleType, []util.Uint256{{1, 2, 3}}))
}

func TestEncodeDecodeAddr(t *testing.T) {
	const count = 4
	p := payload.NewAddressList(count)
	p.Addrs[0] = &payload.AddressAndTime{
		Timestamp: rand.Uint32(),
		Capabilities: capability.Capabilities{{
			Type: capability.FullNode,
			Data: &capability.Node{StartHeight: rand.Uint32()},
		}},
	}
	p.Addrs[1] = &payload.AddressAndTime{
		Timestamp: rand.Uint32(),
		Capabilities: capability.Capabilities{{
			Type: capability.TCPServer,
			Data: &capability.Server{Port: uint16(rand.Uint32())},
		}},
	}
	p.Addrs[2] = &payload.AddressAndTime{
		Timestamp: rand.Uint32(),
		Capabilities: capability.Capabilities{{
			Type: capability.WSServer,
			Data: &capability.Server{Port: uint16(rand.Uint32())},
		}},
	}
	p.Addrs[3] = &payload.AddressAndTime{
		Timestamp: rand.Uint32(),
		Capabilities: capability.Capabilities{{
			Type: capability.DisableCompressionNode,
			Data: &capability.DisableCompression{},
		}},
	}
	testEncodeDecode(t, CMDAddr, p)
}

func TestEncodeDecodeBlock(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		testEncodeDecode(t, CMDBlock, newDummyBlock(12, 1))
	})
}

func TestEncodeDecodeGetBlock(t *testing.T) {
	t.Run("good, Count>0", func(t *testing.T) {
		testEncodeDecode(t, CMDGetBlocks, &payload.GetBlocks{
			HashStart: random.Uint256(),
			Count:     int16(rand.Uint32() >> 17),
		})
	})
	t.Run("good, Count=-1", func(t *testing.T) {
		testEncodeDecode(t, CMDGetBlocks, &payload.GetBlocks{
			HashStart: random.Uint256(),
			Count:     -1,
		})
	})
	t.Run("bad, Count=-2", func(t *testing.T) {
		testEncodeDecodeFail(t, CMDGetBlocks, &payload.GetBlocks{
			HashStart: random.Uint256(),
			Count:     -2,
		})
	})
}

func TestEnodeDecodeGetHeaders(t *testing.T) {
	testEncodeDecode(t, CMDGetHeaders, &payload.GetBlockByIndex{
		IndexStart: rand.Uint32(),
		Count:      payload.MaxHeadersAllowed,
	})
}

func TestEncodeDecodeGetBlockByIndex(t *testing.T) {
	t.Run("good, Count>0", func(t *testing.T) {
		testEncodeDecode(t, CMDGetBlockByIndex, &payload.GetBlockByIndex{
			IndexStart: rand.Uint32(),
			Count:      payload.MaxHeadersAllowed,
		})
	})
	t.Run("bad, Count too big", func(t *testing.T) {
		testEncodeDecodeFail(t, CMDGetBlockByIndex, &payload.GetBlockByIndex{
			IndexStart: rand.Uint32(),
			Count:      payload.MaxHeadersAllowed + 1,
		})
	})
	t.Run("good, Count=-1", func(t *testing.T) {
		testEncodeDecode(t, CMDGetBlockByIndex, &payload.GetBlockByIndex{
			IndexStart: rand.Uint32(),
			Count:      -1,
		})
	})
	t.Run("bad, Count=-2", func(t *testing.T) {
		testEncodeDecodeFail(t, CMDGetBlockByIndex, &payload.GetBlockByIndex{
			IndexStart: rand.Uint32(),
			Count:      -2,
		})
	})
}

func TestEncodeDecodeTransaction(t *testing.T) {
	testEncodeDecode(t, CMDTX, newDummyTx())
}

func TestEncodeDecodeMerkleBlock(t *testing.T) {
	base := &block.Header{
		PrevHash:  random.Uint256(),
		Timestamp: rand.Uint64(),
		Script: transaction.Witness{
			InvocationScript:   random.Bytes(10),
			VerificationScript: random.Bytes(11),
		},
	}
	base.Hash()
	t.Run("good", func(t *testing.T) {
		testEncodeDecode(t, CMDMerkleBlock, &payload.MerkleBlock{
			Header:  base,
			TxCount: 1,
			Hashes:  []util.Uint256{random.Uint256()},
			Flags:   []byte{0},
		})
	})
	t.Run("bad, invalid TxCount", func(t *testing.T) {
		testEncodeDecodeFail(t, CMDMerkleBlock, &payload.MerkleBlock{
			Header:  base,
			TxCount: 2,
			Hashes:  []util.Uint256{random.Uint256()},
			Flags:   []byte{0},
		})
	})
}

func TestEncodeDecodeNotFound(t *testing.T) {
	testEncodeDecode(t, CMDNotFound, &payload.Inventory{
		Type:   payload.TXType,
		Hashes: []util.Uint256{random.Uint256()},
	})
}

func TestEncodeDecodeGetMPTData(t *testing.T) {
	testEncodeDecode(t, CMDGetMPTData, &payload.MPTInventory{
		Hashes: []util.Uint256{
			{1, 2, 3},
			{4, 5, 6},
		},
	})
}

func TestEncodeDecodeMPTData(t *testing.T) {
	testEncodeDecode(t, CMDMPTData, &payload.MPTData{
		Nodes: [][]byte{{1, 2, 3}, {4, 5, 6}},
	})
}

func TestInvalidMessages(t *testing.T) {
	t.Run("CMDBlock, empty payload", func(t *testing.T) {
		testEncodeDecodeFail(t, CMDBlock, payload.NullPayload{})
	})
	t.Run("send decompressed with flag", func(t *testing.T) {
		m := NewMessage(CMDTX, newDummyTx())
		data, err := testserdes.Encode(m)
		require.NoError(t, err)
		require.True(t, m.Flags&Compressed == 0)
		data[0] |= byte(Compressed)
		require.Error(t, testserdes.Decode(data, &Message{}))
	})
	t.Run("invalid command", func(t *testing.T) {
		testEncodeDecodeFail(t, CommandType(0xFF), &payload.Version{Magic: netmode.UnitTestNet})
	})
	t.Run("very big payload size", func(t *testing.T) {
		m := NewMessage(CMDBlock, nil)
		w := io.NewBufBinWriter()
		w.WriteB(byte(m.Flags))
		w.WriteB(byte(m.Command))
		w.WriteVarBytes(make([]byte, payload.MaxSize+1))
		require.NoError(t, w.Err)
		require.Error(t, testserdes.Decode(w.Bytes(), &Message{}))
	})
	t.Run("fail to encode message if payload can't be serialized", func(t *testing.T) {
		m := NewMessage(CMDBlock, failSer(true))
		_, err := m.Bytes()
		require.Error(t, err)

		// good otherwise
		m = NewMessage(CMDBlock, failSer(false))
		_, err = m.Bytes()
		require.NoError(t, err)
	})
	t.Run("trimmed payload", func(t *testing.T) {
		m := NewMessage(CMDBlock, newDummyBlock(1, 0))
		data, err := testserdes.Encode(m)
		require.NoError(t, err)
		data = data[:len(data)-1]
		require.Error(t, testserdes.Decode(data, &Message{}))
	})
}

type failSer bool

func (f failSer) EncodeBinary(r *io.BinWriter) {
	if f {
		r.Err = errors.New("unserializable payload")
	}
}

func (failSer) DecodeBinary(w *io.BinReader) {}

func newDummyBlock(height uint32, txCount int) *block.Block {
	b := &block.Block{}
	b.Index = height
	b.PrevHash = random.Uint256()
	b.Timestamp = rand.Uint64()
	b.Script.InvocationScript = random.Bytes(2)
	b.Script.VerificationScript = random.Bytes(3)
	b.Transactions = make([]*transaction.Transaction, txCount)
	for i := range b.Transactions {
		b.Transactions[i] = newDummyTx()
	}
	b.Hash()
	return b
}

func newDummyTx() *transaction.Transaction {
	tx := transaction.New(random.Bytes(100), 123)
	tx.Signers = []transaction.Signer{{Account: random.Uint160()}}
	tx.Scripts = []transaction.Witness{{InvocationScript: []byte{}, VerificationScript: []byte{}}}
	tx.Size()
	tx.Hash()
	return tx
}

func testEncodeDecode(t *testing.T, cmd CommandType, p payload.Payload) *Message {
	expected := NewMessage(cmd, p)
	actual := &Message{}
	testserdes.EncodeDecode(t, expected, actual)
	return actual
}

func testEncodeDecodeFail(t *testing.T, cmd CommandType, p payload.Payload) *Message {
	expected := NewMessage(cmd, p)
	data, err := testserdes.Encode(expected)
	require.NoError(t, err)

	actual := &Message{}
	require.Error(t, testserdes.Decode(data, actual))
	return actual
}
