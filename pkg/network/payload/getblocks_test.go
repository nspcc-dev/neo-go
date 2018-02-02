package payload

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"reflect"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// 202 255 230 220 70 9 186 252 176 194 242 74 13 19 131 202 0 79 90 239 159 49 61 79 19 44 76 27 251 10 249 225
// caffe6dc4609bafcb0c2f24a0d1383ca004f5aef9f313d4f132c4c1bfb0af9e1

func TestSomethingHere(t *testing.T) {
	hash, _ := util.Uint256FromHexString("caffe6dc4609bafcb0c2f24a0d1383ca004f5aef9f313d4f132c4c1bfb0af9e1")
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, hash)
	fmt.Println(buf.Bytes())

}

func TestGetBlocksEncodeDecode(t *testing.T) {
	hash, _ := util.Uint256FromHexString("d42561e3d30e15be6400b6df2f328e02d2bf6354c41dce433bc57687c82144bf")
	fmt.Println(hash)

	start := []util.Uint256{
		hash,
		sha256.Sum256([]byte("a")),
		sha256.Sum256([]byte("b")),
		sha256.Sum256([]byte("c")),
	}
	stop := sha256.Sum256([]byte("d"))

	p := NewGetBlocks(start, stop)
	buf := new(bytes.Buffer)
	if err := p.EncodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	fmt.Println(buf.Bytes()[1:33])

	if have, want := buf.Len(), 1+32*len(p.HashStart)+32; have != want {
		t.Fatalf("expecting a length %d got %d", want, have)
	}

	pDecode := &GetBlocks{}
	if err := pDecode.DecodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(p, pDecode) {
		t.Fatalf("expecting both getblocks payloads to be equal %v and %v", p, pDecode)
	}
}

func TestGetBlocksWithEmptyHashStop(t *testing.T) {
	start := []util.Uint256{
		sha256.Sum256([]byte("a")),
	}
	stop := util.Uint256{}

	buf := new(bytes.Buffer)
	p := NewGetBlocks(start, stop)
	if err := p.EncodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	pDecode := &GetBlocks{}
	if err := pDecode.DecodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(p, pDecode) {
		t.Fatalf("expecting both getblocks payloads to be equal %v and %v", p, pDecode)
	}
}
