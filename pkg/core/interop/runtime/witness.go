package runtime

import (
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/pkg/errors"
)

// CheckHashedWitness checks given hash against current list of script hashes
// for verifying in the interop context.
func CheckHashedWitness(ic *interop.Context, v vm.ScriptHashGetter, hash util.Uint160) (bool, error) {
	if tx, ok := ic.Container.(*transaction.Transaction); ok {
		return checkScope(ic.DAO, tx, v, hash)
	}

	// only for non-Transaction types (Block, etc.)
	hashes, err := ic.Chain.GetScriptHashesForVerifying(ic.Tx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get script hashes")
	}
	for _, v := range hashes {
		if hash.Equals(v) {
			return true, nil
		}
	}
	return false, nil
}

func checkScope(d dao.DAO, tx *transaction.Transaction, v vm.ScriptHashGetter, hash util.Uint160) (bool, error) {
	for _, c := range tx.Cosigners {
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
func CheckKeyedWitness(ic *interop.Context, v vm.ScriptHashGetter, key *keys.PublicKey) (bool, error) {
	return CheckHashedWitness(ic, v, key.GetScriptHash())
}

// CheckWitness checks witnesses.
func CheckWitness(ic *interop.Context, v *vm.VM) error {
	var res bool
	var err error

	hashOrKey := v.Estack().Pop().Bytes()
	hash, err := util.Uint160DecodeBytesBE(hashOrKey)
	if err != nil {
		var key *keys.PublicKey
		key, err = keys.NewPublicKeyFromBytes(hashOrKey)
		if err != nil {
			return errors.New("parameter given is neither a key nor a hash")
		}
		res, err = CheckKeyedWitness(ic, v, key)
	} else {
		res, err = CheckHashedWitness(ic, v, hash)
	}
	if err != nil {
		return errors.Wrap(err, "failed to check")
	}
	v.Estack().PushVal(res)
	return nil
}
