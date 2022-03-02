package transaction

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// NotaryAssisted represents attribute for notary service transactions.
type NotaryAssisted struct {
	NKeys uint8 `json:"nkeys"`
}

// DecodeBinary implements io.Serializable interface.
func (n *NotaryAssisted) DecodeBinary(br *io.BinReader) {
	bytes := br.ReadVarBytes(1)
	if br.Err != nil {
		return
	}
	if len(bytes) != 1 {
		br.Err = fmt.Errorf("expected 1 byte, got %d", len(bytes))
		return
	}
	n.NKeys = bytes[0]
}

// EncodeBinary implements io.Serializable interface.
func (n *NotaryAssisted) EncodeBinary(w *io.BinWriter) {
	w.WriteVarBytes([]byte{n.NKeys})
}

func (n *NotaryAssisted) toJSONMap(m map[string]interface{}) {
	m["nkeys"] = n.NKeys
}
