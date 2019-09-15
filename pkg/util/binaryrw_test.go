package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteVarUint1(t *testing.T) {
	var (
		val = uint64(1)
	)
	bw := NewBufBinWriter()
	bw.WriteVarUint(val)
	assert.Nil(t, bw.Err)
	buf := bw.Bytes()
	assert.Equal(t, 1, len(buf))
}

func TestWriteVarUint1000(t *testing.T) {
	var (
		val = uint64(1000)
	)
	bw := NewBufBinWriter()
	bw.WriteVarUint(val)
	assert.Nil(t, bw.Err)
	buf := bw.Bytes()
	assert.Equal(t, 3, len(buf))
	assert.Equal(t, byte(0xfd), buf[0])
	br := NewBinReaderFromBuf(buf)
	res := br.ReadVarUint()
	assert.Nil(t, br.Err)
	assert.Equal(t, val, res)
}

func TestWriteVarUint100000(t *testing.T) {
	var (
		val = uint64(100000)
	)
	bw := NewBufBinWriter()
	bw.WriteVarUint(val)
	assert.Nil(t, bw.Err)
	buf := bw.Bytes()
	assert.Equal(t, 5, len(buf))
	assert.Equal(t, byte(0xfe), buf[0])
	br := NewBinReaderFromBuf(buf)
	res := br.ReadVarUint()
	assert.Nil(t, br.Err)
	assert.Equal(t, val, res)
}

func TestWriteVarUint100000000000(t *testing.T) {
	var (
		val = uint64(1000000000000)
	)
	bw := NewBufBinWriter()
	bw.WriteVarUint(val)
	assert.Nil(t, bw.Err)
	buf := bw.Bytes()
	assert.Equal(t, 9, len(buf))
	assert.Equal(t, byte(0xff), buf[0])
	br := NewBinReaderFromBuf(buf)
	res := br.ReadVarUint()
	assert.Nil(t, br.Err)
	assert.Equal(t, val, res)
}
