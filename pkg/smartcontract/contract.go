package smartcontract

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/vm/emit"
	"github.com/CityOfZion/neo-go/pkg/vm/opcode"
)

// CreateMultiSigRedeemScript creates a script runnable by the VM.
func CreateMultiSigRedeemScript(m int, publicKeys keys.PublicKeys) ([]byte, error) {
	if m < 1 {
		return nil, fmt.Errorf("param m cannot be smaller or equal to 1 got %d", m)
	}
	if m > len(publicKeys) {
		return nil, fmt.Errorf("length of the signatures (%d) is higher then the number of public keys", m)
	}
	if m > 1024 {
		return nil, fmt.Errorf("public key count %d exceeds maximum of length 1024", len(publicKeys))
	}

	buf := new(bytes.Buffer)
	if err := emit.Int(buf, int64(m)); err != nil {
		return nil, err
	}
	sort.Sort(publicKeys)
	for _, pubKey := range publicKeys {
		if err := emit.Bytes(buf, pubKey.Bytes()); err != nil {
			return nil, err
		}
	}
	if err := emit.Int(buf, int64(len(publicKeys))); err != nil {
		return nil, err
	}
	if err := emit.Opcode(buf, opcode.CHECKMULTISIG); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
