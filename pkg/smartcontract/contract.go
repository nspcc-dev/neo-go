package smartcontract

import (
	"fmt"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

// CreateMultiSigRedeemScript creates an "m out of n" type verification script
// where n is the length of publicKeys. It modifies passed publicKeys by
// sorting them.
func CreateMultiSigRedeemScript(m int, publicKeys keys.PublicKeys) ([]byte, error) {
	if m < 1 {
		return nil, fmt.Errorf("param m cannot be smaller than 1, got %d", m)
	}
	if m > len(publicKeys) {
		return nil, fmt.Errorf("length of the signatures (%d) is higher then the number of public keys", m)
	}
	if m > 1024 {
		return nil, fmt.Errorf("public key count %d exceeds maximum of length 1024", len(publicKeys))
	}

	buf := io.NewBufBinWriter()
	emit.Int(buf.BinWriter, int64(m))
	sort.Sort(publicKeys)
	for _, pubKey := range publicKeys {
		emit.Bytes(buf.BinWriter, pubKey.Bytes())
	}
	emit.Int(buf.BinWriter, int64(len(publicKeys)))
	emit.Syscall(buf.BinWriter, interopnames.SystemCryptoCheckMultisig)

	return buf.Bytes(), nil
}

// CreateDefaultMultiSigRedeemScript creates an "m out of n" type verification script
// using publicKeys length with the default BFT assumptions of (n - (n-1)/3) for m.
func CreateDefaultMultiSigRedeemScript(publicKeys keys.PublicKeys) ([]byte, error) {
	n := len(publicKeys)
	m := GetDefaultHonestNodeCount(n)
	return CreateMultiSigRedeemScript(m, publicKeys)
}

// CreateMajorityMultiSigRedeemScript creates an "m out of n" type verification script
// using publicKeys length with m set to majority.
func CreateMajorityMultiSigRedeemScript(publicKeys keys.PublicKeys) ([]byte, error) {
	n := len(publicKeys)
	m := GetMajorityHonestNodeCount(n)
	return CreateMultiSigRedeemScript(m, publicKeys)
}

// GetDefaultHonestNodeCount returns minimum number of honest nodes
// required for network of size n.
func GetDefaultHonestNodeCount(n int) int {
	return n - (n-1)/3
}

// GetMajorityHonestNodeCount returns minimum number of honest nodes
// required for majority-style agreement.
func GetMajorityHonestNodeCount(n int) int {
	return n - (n-1)/2
}
