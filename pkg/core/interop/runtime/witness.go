package runtime

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/pkg/errors"
)

// CheckHashedWitness checks given hash against current list of script hashes
// for verifying in the interop context.
func CheckHashedWitness(ic *interop.Context, hash util.Uint160) (bool, error) {
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

// CheckKeyedWitness checks hash of signature check contract with a given public
// key against current list of script hashes for verifying in the interop context.
func CheckKeyedWitness(ic *interop.Context, key *keys.PublicKey) (bool, error) {
	return CheckHashedWitness(ic, key.GetScriptHash())
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
		res, err = CheckKeyedWitness(ic, key)
	} else {
		res, err = CheckHashedWitness(ic, hash)
	}
	if err != nil {
		return errors.Wrap(err, "failed to check")
	}
	v.Estack().PushVal(res)
	return nil
}
