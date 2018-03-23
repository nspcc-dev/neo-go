package rpc

import (
	"fmt"
)

type (
	// Param represent a param either passed to
	// the server or to send to a server using
	// the client.
	Param struct {
		StringVal string
		IntVal    int
		Type      string
		RawValue  interface{}
	}
)

func (p Param) String() string {
	return fmt.Sprintf("%v", p.RawValue)
}
