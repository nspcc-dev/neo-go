package payload

import "io"

// Payload is anything that can be binary encoded/decoded.
type Payload interface {
    EncodeBinary(io.Writer) error
    DecodeBinary(io.Reader) error
}
