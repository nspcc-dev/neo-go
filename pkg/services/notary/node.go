package notary

import (
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"go.uber.org/zap"
)

// UpdateNotaryNodes implements Notary interface and updates current notary account.
func (n *Notary) UpdateNotaryNodes(notaryNodes keys.PublicKeys) {
	n.accMtx.Lock()
	defer n.accMtx.Unlock()

	if n.currAccount != nil {
		for _, node := range notaryNodes {
			if node.Equal(n.currAccount.PrivateKey().PublicKey()) {
				return
			}
		}
	}

	var acc *wallet.Account
	for _, node := range notaryNodes {
		acc = n.wallet.GetAccount(node.GetScriptHash())
		if acc != nil {
			if acc.PrivateKey() != nil {
				break
			}
			err := acc.Decrypt(n.Config.MainCfg.UnlockWallet.Password, n.wallet.Scrypt)
			if err != nil {
				n.Config.Log.Warn("can't unlock notary node account",
					zap.String("address", address.Uint160ToString(acc.Contract.ScriptHash())),
					zap.Error(err))
				acc = nil
			}
			break
		}
	}

	n.currAccount = acc
	if acc == nil {
		n.reqMtx.Lock()
		n.requests = make(map[util.Uint256]*request)
		n.reqMtx.Unlock()
	}
}

func (n *Notary) getAccount() *wallet.Account {
	n.accMtx.RLock()
	defer n.accMtx.RUnlock()
	return n.currAccount
}
