package smartcontract

import (
	"bytes"
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/vm"
)

// CreateMultiSigRedeemScript will output a script that will run on the VM.
func CreateMultiSigRedeemScript(m int, publicKeys []*crypto.PublicKey) ([]byte, error) {
	if m <= 1 {
		return nil, fmt.Errorf("param m cannot be smaller or equal to 1 got %d", m)
	}

	buf := new(bytes.Buffer)
	vm.EmitInt(buf, int64(m))
	for _, pubKey := range publicKeys {
		vm.EmitBytes(buf, pubKey.Bytes())
	}
	vm.EmitInt(buf, int64(len(publicKeys)))
	vm.EmitOpcode(buf, vm.Ocheckmultisig)

	return buf.Bytes(), nil
}
