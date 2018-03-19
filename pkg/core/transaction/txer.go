package transaction

import "io"

//TXer is interface that can act as the underlying data of
// a transaction.
type TXer interface {
	DecodeBinary(io.Reader) error
	EncodeBinary(io.Writer) error
}
