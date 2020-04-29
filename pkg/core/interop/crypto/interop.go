package crypto

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

var (
	ecdsaVerifyID        = emit.InteropNameToID([]byte("Neo.Crypto.ECDsaVerify"))
	ecdsaCheckMultisigID = emit.InteropNameToID([]byte("Neo.Crypto.ECDsaCheckMultiSig"))
	sha256ID             = emit.InteropNameToID([]byte("Neo.Crypto.SHA256"))
)

// GetInterop returns interop getter for crypto-related stuff.
func GetInterop(ic *interop.Context) func(uint32) *vm.InteropFuncPrice {
	return func(id uint32) *vm.InteropFuncPrice {
		switch id {
		case ecdsaVerifyID:
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return ECDSAVerify(ic, v)
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
