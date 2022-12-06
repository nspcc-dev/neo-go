package structs

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/ledger"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/management"
)

type Internal struct {
	Bool       bool
	Int        int
	Bytes      []byte
	String     string
	H160       interop.Hash160
	H256       interop.Hash256
	PK         interop.PublicKey
	PubKey     interop.PublicKey
	Sign       interop.Signature
	ArrOfBytes [][]byte
	ArrOfH160  []interop.Hash160
	Map        map[int][]interop.PublicKey
	Struct     *Internal
}

func Contract(mc management.Contract) management.Contract {
	return mc
}

func Block(b *ledger.Block) *ledger.Block {
	return b
}

func Transaction(t *ledger.Transaction) *ledger.Transaction {
	return t
}

func Struct(s *Internal) *Internal {
	return s
}
