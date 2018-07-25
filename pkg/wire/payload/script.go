package payload

import (
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type Script struct {
	InvocationScript   []byte
	VerificationScript []byte
}

func (s *Script) EncodeScript(bw *util.BinWriter) error {

	bw.VarUint(uint64(len(s.InvocationScript)))
	bw.Write(s.InvocationScript)

	bw.VarUint(uint64(len(s.VerificationScript)))
	bw.Write(s.VerificationScript)

	return bw.Err
}

func (s *Script) DecodeScript(br *util.BinReader) error {

	lenb := br.VarUint()
	s.InvocationScript = make([]byte, lenb)
	br.Read(s.InvocationScript)

	lenb = br.VarUint()
	s.VerificationScript = make([]byte, lenb)
	br.Read(s.VerificationScript)

	return br.Err
}
