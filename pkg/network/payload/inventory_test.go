package payload

import (
	"bytes"
	"crypto/sha256"
	"reflect"
	"testing"

	. "github.com/CityOfZion/neo-go/pkg/util"
)

func TestInventoryEncodeDecode(t *testing.T) {
	hashes := []Uint256{
		sha256.Sum256([]byte("a")),
		sha256.Sum256([]byte("b")),
	}
	inv := NewInventory(BlockType, hashes)

	buf := new(bytes.Buffer)
	if err := inv.EncodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	invDecode := &Inventory{}
	if err := invDecode.DecodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(inv, invDecode) {
		t.Fatalf("expected both inventories to be equal %v and %v", inv, invDecode)
	}
}
