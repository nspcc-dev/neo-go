package binary

import (
	"encoding/base64"

	"github.com/mr-tron/base58"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// Serialize serializes top stack item into a ByteArray.
func Serialize(ic *interop.Context) error {
	return vm.RuntimeSerialize(ic.VM)
}

// Deserialize deserializes ByteArray from a stack into an item.
func Deserialize(ic *interop.Context) error {
	return vm.RuntimeDeserialize(ic.VM)
}

// EncodeBase64 encodes top stack item into a base64 string.
func EncodeBase64(ic *interop.Context) error {
	src := ic.VM.Estack().Pop().Bytes()
	result := base64.StdEncoding.EncodeToString(src)
	ic.VM.Estack().PushVal([]byte(result))
	return nil
}

// DecodeBase64 decodes top stack item from base64 string to byte array.
func DecodeBase64(ic *interop.Context) error {
	src := ic.VM.Estack().Pop().String()
	result, err := base64.StdEncoding.DecodeString(src)
	if err != nil {
		return err
	}
	ic.VM.Estack().PushVal(result)
	return nil
}

// EncodeBase58 encodes top stack item into a base58 string.
func EncodeBase58(ic *interop.Context) error {
	src := ic.VM.Estack().Pop().Bytes()
	result := base58.Encode(src)
	ic.VM.Estack().PushVal([]byte(result))
	return nil
}

// DecodeBase58 decodes top stack item from base58 string to byte array.
func DecodeBase58(ic *interop.Context) error {
	src := ic.VM.Estack().Pop().String()
	result, err := base58.Decode(src)
	if err != nil {
		return err
	}
	ic.VM.Estack().PushVal(result)
	return nil
}
