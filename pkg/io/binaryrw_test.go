package io

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mocks io.Reader and io.Writer, always fails to Write() or Read().
type badRW struct{}

func (w *badRW) Write(p []byte) (int, error) {
	return 0, errors.New("it always fails")
}

func (w *badRW) Read(p []byte) (int, error) {
	return w.Write(p)
}

func TestWriteU64LE(t *testing.T) {
	var (
		val     uint64 = 0xbadc0de15a11dead
		readval uint64
		bin     = []byte{0xad, 0xde, 0x11, 0x5a, 0xe1, 0x0d, 0xdc, 0xba}
	)
	bw := NewBufBinWriter()
	bw.WriteU64LE(val)
	assert.Nil(t, bw.Error())
	wrotebin := bw.Bytes()
	assert.Equal(t, wrotebin, bin)
	br := NewBinReaderFromBuf(bin)
	readval = br.ReadU64LE()
	assert.Nil(t, br.Err)
	assert.Equal(t, val, readval)
}

func TestWriteU32LE(t *testing.T) {
	var (
		val     uint32 = 0xdeadbeef
		readval uint32
		bin     = []byte{0xef, 0xbe, 0xad, 0xde}
	)
	bw := NewBufBinWriter()
	bw.WriteU32LE(val)
	assert.Nil(t, bw.Error())
	wrotebin := bw.Bytes()
	assert.Equal(t, wrotebin, bin)
	br := NewBinReaderFromBuf(bin)
	readval = br.ReadU32LE()
	assert.Nil(t, br.Err)
	assert.Equal(t, val, readval)
}

func TestWriteU16LE(t *testing.T) {
	var (
		val     uint16 = 0xbabe
		readval uint16
		bin     = []byte{0xbe, 0xba}
	)
	bw := NewBufBinWriter()
	bw.WriteU16LE(val)
	assert.Nil(t, bw.Error())
	wrotebin := bw.Bytes()
	assert.Equal(t, wrotebin, bin)
	br := NewBinReaderFromBuf(bin)
	readval = br.ReadU16LE()
	assert.Nil(t, br.Err)
	assert.Equal(t, val, readval)
}

func TestWriteU16BE(t *testing.T) {
	var (
		val     uint16 = 0xbabe
		readval uint16
		bin     = []byte{0xba, 0xbe}
	)
	bw := NewBufBinWriter()
	bw.WriteU16BE(val)
	assert.Nil(t, bw.Error())
	wrotebin := bw.Bytes()
	assert.Equal(t, wrotebin, bin)
	br := NewBinReaderFromBuf(bin)
	readval = br.ReadU16BE()
	assert.Nil(t, br.Err)
	assert.Equal(t, val, readval)
}

func TestWriteByte(t *testing.T) {
	var (
		val     byte = 0xa5
		readval byte
		bin     = []byte{0xa5}
	)
	bw := NewBufBinWriter()
	bw.WriteB(val)
	assert.Nil(t, bw.Error())
	wrotebin := bw.Bytes()
	assert.Equal(t, wrotebin, bin)
	br := NewBinReaderFromBuf(bin)
	readval = br.ReadB()
	assert.Nil(t, br.Err)
	assert.Equal(t, val, readval)
}

func TestWriteBool(t *testing.T) {
	var (
		bin = []byte{0x01, 0x00}
	)
	bw := NewBufBinWriter()
	bw.WriteBool(true)
	bw.WriteBool(false)
	assert.Nil(t, bw.Error())
	wrotebin := bw.Bytes()
	assert.Equal(t, wrotebin, bin)
	br := NewBinReaderFromBuf(bin)
	assert.Equal(t, true, br.ReadBool())
	assert.Equal(t, false, br.ReadBool())
	assert.Nil(t, br.Err)
}

func TestReadLEErrors(t *testing.T) {
	bin := []byte{0xad, 0xde, 0x11, 0x5a, 0xe1, 0x0d, 0xdc, 0xba}
	br := NewBinReaderFromBuf(bin)
	// Prime the buffers with something.
	_ = br.ReadU64LE()
	assert.Nil(t, br.Err)

	assert.Equal(t, uint64(0), br.ReadU64LE())
	assert.Equal(t, uint32(0), br.ReadU32LE())
	assert.Equal(t, uint16(0), br.ReadU16LE())
	assert.Equal(t, uint16(0), br.ReadU16BE())
	assert.Equal(t, byte(0), br.ReadB())
	assert.Equal(t, false, br.ReadBool())
	assert.NotNil(t, br.Err)
}

func TestBufBinWriter_Len(t *testing.T) {
	val := []byte{0xde}
	bw := NewBufBinWriter()
	bw.WriteBytes(val)
	require.Equal(t, 1, bw.Len())
}

func TestBinReader_ReadVarBytes(t *testing.T) {
	buf := make([]byte, 11)
	for i := range buf {
		buf[i] = byte(i)
	}
	w := NewBufBinWriter()
	w.WriteVarBytes(buf)
	require.NoError(t, w.Error())
	data := w.Bytes()

	t.Run("NoArguments", func(t *testing.T) {
		r := NewBinReaderFromBuf(data)
		actual := r.ReadVarBytes()
		require.NoError(t, r.Err)
		require.Equal(t, buf, actual)
	})
	t.Run("Good", func(t *testing.T) {
		r := NewBinReaderFromBuf(data)
		actual := r.ReadVarBytes(11)
		require.NoError(t, r.Err)
		require.Equal(t, buf, actual)
	})
	t.Run("Bad", func(t *testing.T) {
		r := NewBinReaderFromBuf(data)
		r.ReadVarBytes(10)
		require.Error(t, r.Err)
	})
}

func TestWriterErrHandling(t *testing.T) {
	var badio = &badRW{}
	bw := NewBinWriterFromIO(badio)
	bw.WriteU32LE(uint32(0))
	assert.NotNil(t, bw.Error())
	// these should work (without panic), preserving the Err
	bw.WriteU32LE(uint32(0))
	bw.WriteU16BE(uint16(0))
	bw.WriteVarUint(0)
	bw.WriteVarBytes([]byte{0x55, 0xaa})
	bw.WriteString("neo")
	assert.NotNil(t, bw.Error())
}

func TestReaderErrHandling(t *testing.T) {
	var (
		badio = &badRW{}
	)
	br := NewBinReaderFromIO(badio)
	br.ReadU32LE()
	assert.NotNil(t, br.Err)
	// these should work (without panic), preserving the Err
	br.ReadU32LE()
	br.ReadU16BE()
	val := br.ReadVarUint()
	assert.Equal(t, val, uint64(0))
	b := br.ReadVarBytes()
	assert.Equal(t, b, []byte{})
	s := br.ReadString()
	assert.Equal(t, s, "")
	assert.NotNil(t, br.Err)
}

func TestBufBinWriterErr(t *testing.T) {
	bw := NewBufBinWriter()
	bw.WriteU32LE(uint32(0))
	assert.Nil(t, bw.Error())
	// inject error
	bw.SetError(errors.New("oopsie"))
	res := bw.Bytes()
	assert.NotNil(t, bw.Error())
	assert.Nil(t, res)
}

func TestBufBinWriterReset(t *testing.T) {
	bw := NewBufBinWriter()
	for i := 0; i < 3; i++ {
		bw.WriteU32LE(uint32(i))
		assert.Nil(t, bw.Error())
		_ = bw.Bytes()
		assert.NotNil(t, bw.Error())
		bw.Reset()
		assert.Nil(t, bw.Error())
	}
}

func TestWriteString(t *testing.T) {
	var (
		str = "teststring"
	)
	bw := NewBufBinWriter()
	bw.WriteString(str)
	assert.Nil(t, bw.Error())
	wrotebin := bw.Bytes()
	// +1 byte for length
	assert.Equal(t, len(wrotebin), len(str)+1)
	br := NewBinReaderFromBuf(wrotebin)
	readstr := br.ReadString()
	assert.Nil(t, br.Err)
	assert.Equal(t, str, readstr)
}

func TestWriteVarUint1(t *testing.T) {
	var (
		val = uint64(1)
	)
	bw := NewBufBinWriter()
	bw.WriteVarUint(val)
	assert.Nil(t, bw.Error())
	buf := bw.Bytes()
	assert.Equal(t, 1, len(buf))
	br := NewBinReaderFromBuf(buf)
	res := br.ReadVarUint()
	assert.Nil(t, br.Err)
	assert.Equal(t, val, res)
}

func TestWriteVarUint1000(t *testing.T) {
	var (
		val = uint64(1000)
	)
	bw := NewBufBinWriter()
	bw.WriteVarUint(val)
	assert.Nil(t, bw.Error())
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
	assert.Nil(t, bw.Error())
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
	assert.Nil(t, bw.Error())
	buf := bw.Bytes()
	assert.Equal(t, 9, len(buf))
	assert.Equal(t, byte(0xff), buf[0])
	br := NewBinReaderFromBuf(buf)
	res := br.ReadVarUint()
	assert.Nil(t, br.Err)
	assert.Equal(t, val, res)
}

func TestWriteBytes(t *testing.T) {
	var (
		bin = []byte{0xde, 0xad, 0xbe, 0xef}
	)
	bw := NewBufBinWriter()
	bw.WriteBytes(bin)
	assert.Nil(t, bw.Error())
	buf := bw.Bytes()
	assert.Equal(t, 4, len(buf))
	assert.Equal(t, byte(0xde), buf[0])

	bw = NewBufBinWriter()
	bw.SetError(errors.New("smth bad"))
	bw.WriteBytes(bin)
	assert.Equal(t, 0, bw.Len())
}

type testSerializable uint16

// EncodeBinary implements io.Serializable interface.
func (t testSerializable) EncodeBinary(w BinaryWriter) {
	w.WriteU16LE(uint16(t))
}

// DecodeBinary implements io.Serializable interface.
func (t *testSerializable) DecodeBinary(r BinaryReader) {
	*t = testSerializable(r.ReadU16LE())
}

type testPtrSerializable uint16

// EncodeBinary implements io.Serializable interface.
func (t *testPtrSerializable) EncodeBinary(w BinaryWriter) {
	w.WriteU16LE(uint16(*t))
}

// DecodeBinary implements io.Serializable interface.
func (t *testPtrSerializable) DecodeBinary(r BinaryReader) {
	*t = testPtrSerializable(r.ReadU16LE())
}

func TestBinWriter_WriteArray(t *testing.T) {
	var arr [3]testSerializable
	for i := range arr {
		arr[i] = testSerializable(i)
	}

	expected := []byte{3, 0, 0, 1, 0, 2, 0}

	w := NewBufBinWriter()
	w.WriteArray(arr)
	require.NoError(t, w.Error())
	require.Equal(t, expected, w.Bytes())

	w.Reset()
	w.WriteArray(arr[:])
	require.NoError(t, w.Error())
	require.Equal(t, expected, w.Bytes())

	arrS := make([]Serializable, len(arr))
	for i := range arrS {
		arrS[i] = &arr[i]
	}

	w.Reset()
	w.WriteArray(arr)
	require.NoError(t, w.Error())
	require.Equal(t, expected, w.Bytes())

	w.Reset()
	require.Panics(t, func() { w.WriteArray(1) })

	w.Reset()
	w.SetError(errors.New("error"))
	w.WriteArray(arr[:])
	require.Error(t, w.Error())
	require.Equal(t, w.Bytes(), []byte(nil))

	w.Reset()
	require.Panics(t, func() { w.WriteArray([]int{1}) })

	w.Reset()
	w.SetError(errors.New("error"))
	require.Panics(t, func() { w.WriteArray(make(chan testSerializable)) })

	// Ptr receiver test
	var arrPtr [3]testPtrSerializable
	for i := range arrPtr {
		arrPtr[i] = testPtrSerializable(i)
	}
	w.Reset()
	w.WriteArray(arr[:])
	require.NoError(t, w.Error())
	require.Equal(t, expected, w.Bytes())
}

func TestBinReader_ReadArray(t *testing.T) {
	data := []byte{3, 0, 0, 1, 0, 2, 0}
	elems := []testSerializable{0, 1, 2}

	r := NewBinReaderFromBuf(data)
	arrPtr := []*testSerializable{}
	r.ReadArray(&arrPtr)
	require.Equal(t, []*testSerializable{&elems[0], &elems[1], &elems[2]}, arrPtr)

	r = NewBinReaderFromBuf(data)
	arrVal := []testSerializable{}
	r.ReadArray(&arrVal)
	require.NoError(t, r.Err)
	require.Equal(t, elems, arrVal)

	r = NewBinReaderFromBuf(data)
	arrVal = []testSerializable{}
	r.ReadArray(&arrVal, 3)
	require.NoError(t, r.Err)
	require.Equal(t, elems, arrVal)

	r = NewBinReaderFromBuf(data)
	arrVal = []testSerializable{}
	r.ReadArray(&arrVal, 2)
	require.Error(t, r.Err)

	r = NewBinReaderFromBuf([]byte{0})
	r.ReadArray(&arrVal)
	require.NoError(t, r.Err)
	require.Equal(t, []testSerializable{}, arrVal)

	r = NewBinReaderFromBuf([]byte{0})
	r.Err = errors.New("error")
	arrVal = ([]testSerializable)(nil)
	r.ReadArray(&arrVal)
	require.Error(t, r.Err)
	require.Equal(t, ([]testSerializable)(nil), arrVal)

	r = NewBinReaderFromBuf([]byte{0})
	r.Err = errors.New("error")
	arrPtr = ([]*testSerializable)(nil)
	r.ReadArray(&arrVal)
	require.Error(t, r.Err)
	require.Equal(t, ([]*testSerializable)(nil), arrPtr)

	r = NewBinReaderFromBuf([]byte{0})
	arrVal = []testSerializable{1, 2}
	r.ReadArray(&arrVal)
	require.NoError(t, r.Err)
	require.Equal(t, []testSerializable{}, arrVal)

	r = NewBinReaderFromBuf([]byte{1})
	require.Panics(t, func() { r.ReadArray(&[]int{1}) })

	r = NewBinReaderFromBuf([]byte{0})
	r.Err = errors.New("error")
	require.Panics(t, func() { r.ReadArray(1) })
}

func TestBinReader_ReadBytes(t *testing.T) {
	data := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	r := NewBinReaderFromBuf(data)

	buf := make([]byte, 4)
	r.ReadBytes(buf)
	require.NoError(t, r.Err)
	require.Equal(t, data[:4], buf)

	r.ReadBytes([]byte{})
	require.NoError(t, r.Err)

	buf = make([]byte, 3)
	r.ReadBytes(buf)
	require.NoError(t, r.Err)
	require.Equal(t, data[4:7], buf)

	buf = make([]byte, 2)
	r.ReadBytes(buf)
	require.Error(t, r.Err)

	r.ReadBytes([]byte{})
	require.Error(t, r.Err)
}
