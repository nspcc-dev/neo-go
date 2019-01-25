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
	if err := vm.EmitInt(buf, int64(m)); err != nil {
		return nil, err
	}
	sort.Sort(publicKeys)
	for _, pubKey := range publicKeys {
		if err := vm.EmitBytes(buf, pubKey.Bytes()); err != nil {
			return nil, err
		}
	}
	if err := vm.EmitInt(buf, int64(len(publicKeys))); err != nil {
		return nil, err
	}
	if err := vm.EmitOpcode(buf, vm.Ocheckmultisig); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
