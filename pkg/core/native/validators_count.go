package native

import (
	"errors"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// MaxValidatorsVoted limits the number of validators that one can vote for.
const MaxValidatorsVoted = 1024

// ValidatorsCount represents votes with particular number of consensus nodes
// for this number to be changeable by the voting system.
type ValidatorsCount [MaxValidatorsVoted]big.Int

// ValidatorsCountFromBytes converts serialized ValidatorsCount to structure.
func ValidatorsCountFromBytes(b []byte) (*ValidatorsCount, error) {
	vc := new(ValidatorsCount)
	if len(b) == 0 {
		return vc, nil
	}
	r := io.NewBinReaderFromBuf(b)
	vc.DecodeBinary(r)

	if r.Err != nil {
		return nil, r.Err
	}
	return vc, nil
}

// Bytes returns serialized ValidatorsCount.
func (vc *ValidatorsCount) Bytes() []byte {
	w := io.NewBufBinWriter()
	vc.EncodeBinary(w.BinWriter)
	if w.Err != nil {
		panic(w.Err)
	}
	return w.Bytes()
}

// EncodeBinary implements io.Serializable interface.
func (vc *ValidatorsCount) EncodeBinary(w *io.BinWriter) {
	w.WriteVarUint(uint64(MaxValidatorsVoted))
	for i := range vc {
		w.WriteVarBytes(bigint.ToBytes(&vc[i]))
	}
}

// DecodeBinary implements io.Serializable interface.
func (vc *ValidatorsCount) DecodeBinary(r *io.BinReader) {
	count := r.ReadVarUint()
	if count < 0 || count > MaxValidatorsVoted {
		r.Err = errors.New("invalid validators count")
		return
	}
	for i := 0; i < int(count); i++ {
		buf := r.ReadVarBytes()
		if r.Err != nil {
			return
		}
		vc[i] = *bigint.FromBytes(buf)
	}
}
