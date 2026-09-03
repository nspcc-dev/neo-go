package runtime

import (
	"crypto/elliptic"
	"errors"
	"fmt"
	"slices"

	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// CheckHashedWitness checks the given hash against the current list of script hashes
// for verifying in the interop context.
func CheckHashedWitness(ic *interop.Context, hash util.Uint160) (bool, int, error) {
	callingSH := ic.VM.GetCallingScriptHash()
	if !callingSH.Equals(util.Uint160{}) && hash.Equals(callingSH) {
		return true, 0, nil
	}
	return checkScope(ic, hash)
}

type scopeContext struct {
	*vm.VM
	ic *interop.Context
}

func getContractGroups(v *vm.VM, ic *interop.Context, h util.Uint160) (manifest.Groups, error) {
	if !v.Context().GetCallFlags().Has(callflag.ReadStates) {
		return nil, errors.New("missing ReadStates call flag")
	}
	cs, err := ic.GetContract(h)
	if err != nil {
		return nil, nil // It's OK to not have the contract.
	}
	return manifest.Groups(cs.Manifest.Groups), nil
}

func (sc scopeContext) IsCalledByEntry() bool {
	return sc.VM.Context().IsCalledByEntry()
}

func (sc scopeContext) checkScriptGroups(h util.Uint160, k *keys.PublicKey) (bool, error) {
	groups, err := getContractGroups(sc.VM, sc.ic, h)
	if err != nil {
		return false, err
	}
	return groups.Contains(k), nil
}

func (sc scopeContext) CallingScriptHasGroup(k *keys.PublicKey) (bool, error) {
	return sc.checkScriptGroups(sc.GetCallingScriptHash(), k)
}

func (sc scopeContext) CurrentScriptHasGroup(k *keys.PublicKey) (bool, error) {
	return sc.checkScriptGroups(sc.GetCurrentScriptHash(), k)
}

func checkScope(ic *interop.Context, hash util.Uint160) (bool, int, error) {
	signers := ic.Signers()
	if len(signers) == 0 {
		return false, 0, errors.New("no valid signers")
	}
	for i := range signers {
		c := &signers[i]
		if c.Account == hash {
			if c.Scopes == transaction.Global {
				return true, 0, nil
			}
			if c.Scopes&transaction.CalledByEntry != 0 {
				if ic.VM.Context().IsCalledByEntry() {
					return true, 0, nil
				}
			}
			if c.Scopes&transaction.CustomContracts != 0 {
				currentScriptHash := ic.VM.GetCurrentScriptHash()
				if slices.Contains(c.AllowedContracts, currentScriptHash) {
					return true, 0, nil
				}
			}
			if c.Scopes&transaction.CustomGroups != 0 {
				groups, err := getContractGroups(ic.VM, ic, ic.VM.GetCurrentScriptHash())
				if err != nil {
					return false, 0, err
				}
				// check if the current group is the required one
				if slices.ContainsFunc(c.AllowedGroups, groups.Contains) {
					return true, 0, nil
				}
			}
			rulesCount := 0
			if c.Scopes&transaction.Rules != 0 {
				ctx := scopeContext{ic.VM, ic}
				for i, r := range c.Rules {
					res, err := r.Condition.Match(ctx)
					if err != nil {
						return false, 0, err
					}
					if res {
						return r.Action == transaction.WitnessAllow, i + 1, nil
					}
				}
				rulesCount = len(c.Rules)
			}
			return false, rulesCount, nil
		}
	}
	return false, 0, nil
}

// CheckKeyedWitness checks the hash of the signature check contract with the given public
// key against the current list of script hashes for verifying in the interop context.
func CheckKeyedWitness(ic *interop.Context, key *keys.PublicKey) (bool, int, error) {
	return CheckHashedWitness(ic, key.GetScriptHash())
}

// CheckWitness checks witnesses.
func CheckWitness(ic *interop.Context) (*fee.InteropRunStats, error) {
	var res bool
	var err error

	hashOrKey := ic.VM.Estack().Pop().Bytes()
	hash, err := util.Uint160DecodeBytesBE(hashOrKey)
	rulesCount := 0
	if err != nil {
		var key *keys.PublicKey
		key, err = keys.NewPublicKeyFromBytes(hashOrKey, elliptic.P256())
		if err != nil {
			return nil, errors.New("parameter given is neither a key nor a hash")
		}
		res, rulesCount, err = CheckKeyedWitness(ic, key)
	} else {
		res, rulesCount, err = CheckHashedWitness(ic, hash)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to check witness: %w", err)
	}
	ic.VM.Estack().PushItem(stackitem.Bool(res))
	return &fee.InteropRunStats{Stat: rulesCount}, nil
}
