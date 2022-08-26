package notary

import (
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

// FakeSimpleAccount creates a fake account belonging to the given public key.
// It uses a simple signature contract and this account has SignTx that
// returns no error, but at the same time adds no signature (it obviously can't
// do that, so CanSign() returns false for it). Use this account for Actor when
// simple signatures are needed to be collected.
func FakeSimpleAccount(k *keys.PublicKey) *wallet.Account {
	return &wallet.Account{
		Address: k.Address(),
		Contract: &wallet.Contract{
			Script: k.GetVerificationScript(),
		},
	}
}

// FakeMultisigAccount creates a fake account belonging to the given "m out of
// len(pkeys)" account for the given set of keys. The account returned has SignTx
// that returns no error, but at the same time adds no signatures (it can't
// do that, so CanSign() returns false for it). Use this account for Actor when
// multisignature account needs to be added into a notary transaction, but you
// have no keys at all for it (if you have at least one (which usually is the
// case) ordinary multisig account works fine already).
func FakeMultisigAccount(m int, pkeys keys.PublicKeys) (*wallet.Account, error) {
	script, err := smartcontract.CreateMultiSigRedeemScript(m, pkeys)
	if err != nil {
		return nil, err
	}
	return &wallet.Account{
		Address: address.Uint160ToString(hash.Hash160(script)),
		Contract: &wallet.Contract{
			Script: script,
		},
	}, nil
}

// FakeContractAccount creates a fake account belonging to some deployed contract.
// SignTx can be called on this account with no error, but at the same time it
// adds no signature or other data into the invocation script (it obviously can't
// do that, so CanSign() returns false for it). Use this account for Actor when
// one of the signers is a contract and it doesn't need a signature or you can
// provide it externally.
func FakeContractAccount(hash util.Uint160) *wallet.Account {
	return &wallet.Account{
		Address: address.Uint160ToString(hash),
		Contract: &wallet.Contract{
			Deployed: true,
		},
	}
}
