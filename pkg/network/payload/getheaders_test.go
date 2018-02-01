package payload

import (
	"bytes"
	"crypto/sha256"
	"reflect"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/util"
)

func TestGetHeadersEncodeDecode(t *testing.T) {
	start := []util.Uint256{
		sha256.Sum256([]byte("a")),
		sha256.Sum256([]byte("b")),
	}
	stop := sha256.Sum256([]byte("c"))

	p := NewGetHeaders(start, stop)
	buf := new(bytes.Buffer)
	if err := p.EncodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	if have, want := buf.Len(), 1+64+32; have != want {
		t.Fatalf("expecting a length of %d got %d", want, have)
	}

	pDecode := &GetHeaders{}
	if err := pDecode.DecodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(p, pDecode) {
		t.Fatalf("expecting both getheaders payloads to be equal %v and %v", p, pDecode)
	}
}
