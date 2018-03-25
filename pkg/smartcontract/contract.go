package smartcontract

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/vm"
)

// CreateMultiSigRedeemScript will create a script runnable by the VM.
func CreateMultiSigRedeemScript(m int, publicKeys crypto.PublicKeys) ([]byte, error) {
	if m <= 1 {
		return nil, fmt.Errorf("param m cannot be smaller or equal to 1 got %d", m)
	}
	if m > len(publicKeys) {
		return nil, fmt.Errorf("length of the signatures (%d) is higher then the number of public keys", m)
	}
	if m > 1024 {
		return nil, fmt.Errorf("public key count %d exceeds maximum of length 1024", len(publicKeys))
	}

	buf := new(bytes.Buffer)
	vm.EmitInt(buf, int64(m))
	sort.Sort(publicKeys)
	for _, pubKey := range publicKeys {
		vm.EmitBytes(buf, pubKey.Bytes())
	}
	vm.EmitInt(buf, int64(len(publicKeys)))
	vm.EmitOpcode(buf, vm.Ocheckmultisig)

	return buf.Bytes(), nil
}
