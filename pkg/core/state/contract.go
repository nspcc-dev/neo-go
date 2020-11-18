package state

import (
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// Contract holds information about a smart contract in the NEO blockchain.
type Contract struct {
	ID            int32             `json:"id"`
	UpdateCounter uint16            `json:"updatecounter"`
	Hash          util.Uint160      `json:"hash"`
	Script        []byte            `json:"script"`
	Manifest      manifest.Manifest `json:"manifest"`
}

// DecodeBinary implements Serializable interface.
func (cs *Contract) DecodeBinary(br *io.BinReader) {
	cs.ID = int32(br.ReadU32LE())
	cs.UpdateCounter = br.ReadU16LE()
	cs.Hash.DecodeBinary(br)
	cs.Script = br.ReadVarBytes()
	cs.Manifest.DecodeBinary(br)
}

// EncodeBinary implements Serializable interface.
func (cs *Contract) EncodeBinary(bw *io.BinWriter) {
	bw.WriteU32LE(uint32(cs.ID))
	bw.WriteU16LE(cs.UpdateCounter)
	cs.Hash.EncodeBinary(bw)
	bw.WriteVarBytes(cs.Script)
	cs.Manifest.EncodeBinary(bw)
}

// CreateContractHash creates deployed contract hash from transaction sender
// and contract script.
func CreateContractHash(sender util.Uint160, script []byte) util.Uint160 {
	w := io.NewBufBinWriter()
	emit.Opcodes(w.BinWriter, opcode.ABORT)
	emit.Bytes(w.BinWriter, sender.BytesBE())
	emit.Bytes(w.BinWriter, script)
	if w.Err != nil {
		panic(w.Err)
	}
	return hash.Hash160(w.Bytes())
}
