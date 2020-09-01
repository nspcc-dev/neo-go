package runtime

import (
	"crypto/elliptic"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// CheckHashedWitness checks given hash against current list of script hashes
// for verifying in the interop context.
func CheckHashedWitness(ic *interop.Context, hash util.Uint160) (bool, error) {
	if tx, ok := ic.Container.(*transaction.Transaction); ok {
		return checkScope(ic.DAO, tx, ic.VM, hash)
	}

	if !ic.VM.Context().GetCallFlags().Has(smartcontract.AllowStates) {
		return false, errors.New("missing AllowStates call flag")
	}
	return false, errors.New("script container is not a transaction")
}

func checkScope(d dao.DAO, tx *transaction.Transaction, v *vm.VM, hash util.Uint160) (bool, error) {
	for _, c := range tx.Signers {
		if c.Account == hash {
			if c.Scopes == transaction.Global {
				return true, nil
			}
			if c.Scopes&transaction.CalledByEntry != 0 {
				callingScriptHash := v.GetCallingScriptHash()
				entryScriptHash := v.GetEntryScriptHash()
				if callingScriptHash == entryScriptHash {
					return true, nil
				}
			}
			if c.Scopes&transaction.CustomContracts != 0 {
				currentScriptHash := v.GetCurrentScriptHash()
				for _, allowedContract := range c.AllowedContracts {
					if allowedContract == currentScriptHash {
						return true, nil
					}
				}
			}
			if c.Scopes&transaction.CustomGroups != 0 {
				callingScriptHash := v.GetCallingScriptHash()
				if callingScriptHash.Equals(util.Uint160{}) {
					return false, nil
				}
				if !v.Context().GetCallFlags().Has(smartcontract.AllowStates) {
					return false, errors.New("missing AllowStates call flag")
				}
				cs, err := d.GetContractState(callingScriptHash)
				if err != nil {
					return false, err
				}
				// check if the current group is the required one
				for _, allowedGroup := range c.AllowedGroups {
					for _, group := range cs.Manifest.Groups {
						if group.PublicKey.Equal(allowedGroup) {
							return true, nil
						}
					}
				}
			}
			return false, nil
		}
	}
	return false, nil
}

// CheckKeyedWitness checks hash of signature check contract with a given public
// key against current list of script hashes for verifying in the interop context.
func CheckKeyedWitness(ic *interop.Context, key *keys.PublicKey) (bool, error) {
	return CheckHashedWitness(ic, key.GetScriptHash())
}

// CheckWitness checks witnesses.
func CheckWitness(ic *interop.Context) error {
	var res bool
	var err error

	hashOrKey := ic.VM.Estack().Pop().Bytes()
	hash, err := util.Uint160DecodeBytesBE(hashOrKey)
	if err != nil {
		var key *keys.PublicKey
		key, err = keys.NewPublicKeyFromBytes(hashOrKey, elliptic.P256())
		if err != nil {
			return errors.New("parameter given is neither a key nor a hash")
		}
		res, err = CheckKeyedWitness(ic, key)
	} else {
		res, err = CheckHashedWitness(ic, hash)
	}
	if err != nil {
		return fmt.Errorf("failed to check witness: %w", err)
	}
	ic.VM.Estack().PushVal(res)
	return nil
}
