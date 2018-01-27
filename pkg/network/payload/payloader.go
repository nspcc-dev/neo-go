package payload

import "io"

// Nothing is a safe non payload.
var Nothing = nothing{}

// Payloader ..
type Payloader interface {
	Encode(io.Writer) error
	Decode(io.Reader) error
	Size() uint32
}

type nothing struct{}

func (p nothing) Encode(w io.Writer) error { return nil }
func (p nothing) Decode(R io.Reader) error { return nil }
func (p nothing) Size() uint32             { return 0 }
