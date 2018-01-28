package payload

import (
	"encoding"
)

// Payloader is anything that can be binary marshaled and unmarshaled.
// Every payload embbedded in messages need to satisfy the Payloader interface.
type Payloader interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
	Size() uint32
}
