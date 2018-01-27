package payload

import "io"

// Payloader is anything that can be binary encoded and decoded.
// Every payload used in messages need to satisfy the Payloader interface.
type Payloader interface {
	Encode(io.Writer) error
	Decode(io.Reader) error
	Size() uint32
}
