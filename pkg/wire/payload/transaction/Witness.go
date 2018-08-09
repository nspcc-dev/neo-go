package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type Witness struct {
	InvocationScript   []byte
	VerificationScript []byte
}

func (s *Witness) Encode(bw *util.BinWriter) error {

	bw.VarUint(uint64(len(s.InvocationScript)))
	bw.Write(s.InvocationScript)

	bw.VarUint(uint64(len(s.VerificationScript)))
	bw.Write(s.VerificationScript)

	return bw.Err
}

func (s *Witness) Decode(br *util.BinReader) error {

	lenb := br.VarUint()
	s.InvocationScript = make([]byte, lenb)
	br.Read(s.InvocationScript)

	lenb = br.VarUint()
	s.VerificationScript = make([]byte, lenb)
	br.Read(s.VerificationScript)

	return br.Err
}
