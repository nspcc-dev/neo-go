package crypto

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

var (
	ecdsaSecp256r1VerifyID        = emit.InteropNameToID([]byte("Neo.Crypto.VerifyWithECDsaSecp256r1"))
	ecdsaCheckMultisigID = emit.InteropNameToID([]byte("Neo.Crypto.ECDsaCheckMultiSig"))
	sha256ID             = emit.InteropNameToID([]byte("Neo.Crypto.SHA256"))
)

// GetInterop returns interop getter for crypto-related stuff.
func GetInterop(ic *interop.Context) func(uint32) *vm.InteropFuncPrice {
	return func(id uint32) *vm.InteropFuncPrice {
		switch id {
		case ecdsaSecp256r1VerifyID:
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return ECDSASecp256r1Verify(ic, v)
				},
			}
		case ecdsaCheckMultisigID:
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return ECDSACheckMultisig(ic, v)
				},
			}
		case sha256ID:
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return Sha256(ic, v)
				},
			}
		default:
			return nil
		}
	}
}
