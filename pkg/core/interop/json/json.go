package json

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Serialize handles System.JSON.Serialize syscall.
func Serialize(ic *interop.Context) error {
	item := ic.VM.Estack().Pop().Item()
	data, err := stackitem.ToJSON(item)
	if err != nil {
		return err
	}
	ic.VM.Estack().PushVal(data)
	return nil
}

// Deserialize handles System.JSON.Deserialize syscall.
func Deserialize(ic *interop.Context) error {
	data := ic.VM.Estack().Pop().Bytes()
	item, err := stackitem.FromJSON(data)
	if err != nil {
		return err
	}
	ic.VM.Estack().PushVal(item)
	return nil
}
