package json

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Serialize handles System.JSON.Serialize syscall.
func Serialize(_ *interop.Context, v *vm.VM) error {
	item := v.Estack().Pop().Item()
	data, err := stackitem.ToJSON(item)
	if err != nil {
		return err
	}
	v.Estack().PushVal(data)
	return nil
}

// Deserialize handles System.JSON.Deserialize syscall.
func Deserialize(_ *interop.Context, v *vm.VM) error {
	data := v.Estack().Pop().Bytes()
	item, err := stackitem.FromJSON(data)
	if err != nil {
		return err
	}
	v.Estack().PushVal(item)
	return nil
}
