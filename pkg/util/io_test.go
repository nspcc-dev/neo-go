package util

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteVarUint1(t *testing.T) {
	var (
		val = uint64(1)
		buf = new(bytes.Buffer)
	)
	if err := WriteVarUint(buf, val); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, buf.Len())
}

func TestWriteVarUint1000(t *testing.T) {
	var (
		val = uint64(1000)
		buf = new(bytes.Buffer)
	)

	if err := WriteVarUint(buf, val); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 3, buf.Len())
	assert.Equal(t, byte(0xfd), buf.Bytes()[0])
	res := ReadVarUint(buf)
	assert.Equal(t, val, res)
}

func TestWriteVarUint100000(t *testing.T) {
	var (
		val = uint64(100000)
		buf = new(bytes.Buffer)
	)

	if err := WriteVarUint(buf, val); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 5, buf.Len())
	assert.Equal(t, byte(0xfe), buf.Bytes()[0])
	res := ReadVarUint(buf)
	assert.Equal(t, val, res)
}

func TestWriteVarUint100000000000(t *testing.T) {
	var (
		val = uint64(1000000000000)
		buf = new(bytes.Buffer)
	)

	if err := WriteVarUint(buf, val); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 9, buf.Len())
	assert.Equal(t, byte(0xff), buf.Bytes()[0])
	res := ReadVarUint(buf)
	assert.Equal(t, val, res)
}
