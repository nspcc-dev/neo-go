package core

import (
	"log"
	"os"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
)

func TestNewBlockchain(t *testing.T) {
	startHash, _ := util.Uint256DecodeString("996e37358dc369912041f966f8c5d8d3a8255ba5dcbd3447f8a82b55db869099")
	bc := NewBlockchain(nil, nil, startHash)

	want := uint32(0)
	if have := bc.BlockHeight(); want != have {
		t.Fatalf("expected %d got %d", want, have)
	}
	if have := bc.HeaderHeight(); want != have {
		t.Fatalf("expected %d got %d", want, have)
	}
	if have := bc.storedHeaderCount; want != have {
		t.Fatalf("expected %d got %d", want, have)
	}
	if !bc.CurrentBlockHash().Equals(startHash) {
		t.Fatalf("expected current block hash to be %d got %s", startHash, bc.CurrentBlockHash())
	}
}

func TestAddHeaders(t *testing.T) {
	startHash, _ := util.Uint256DecodeString("996e37358dc369912041f966f8c5d8d3a8255ba5dcbd3447f8a82b55db869099")
	bc := NewBlockchain(NewMemoryStore(), log.New(os.Stdout, "", 0), startHash)

	h1 := &Header{BlockBase: BlockBase{Version: 0, Index: 1, Script: &transaction.Witness{}}}
	h2 := &Header{BlockBase: BlockBase{Version: 0, Index: 2, Script: &transaction.Witness{}}}
	h3 := &Header{BlockBase: BlockBase{Version: 0, Index: 3, Script: &transaction.Witness{}}}

	if err := bc.AddHeaders(h1, h2, h3); err != nil {
		t.Fatal(err)
	}
	if want, have := h3.Index, bc.HeaderHeight(); want != have {
		t.Fatalf("expected header height of %d got %d", want, have)
	}
	if want, have := uint32(0), bc.storedHeaderCount; want != have {
		t.Fatalf("expected stored header count to be %d got %d", want, have)
	}
}
