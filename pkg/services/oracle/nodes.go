package oracle

import (
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"go.uber.org/zap"
)

// UpdateOracleNodes updates oracle nodes list.
func (o *Oracle) UpdateOracleNodes(oracleNodes keys.PublicKeys) {
	o.accMtx.Lock()
	defer o.accMtx.Unlock()

	old := o.oracleNodes
	if isEqual := len(old) == len(oracleNodes); isEqual {
		for i := range old {
			if !old[i].Equal(oracleNodes[i]) {
				isEqual = false
				break
			}
		}
		if isEqual {
			return
		}
	}

	var acc *wallet.Account
	for i := range oracleNodes {
		acc = o.wallet.GetAccount(oracleNodes[i].GetScriptHash())
		if acc != nil {
			if acc.PrivateKey() != nil {
				break
			}
			err := acc.Decrypt(o.MainCfg.UnlockWallet.Password)
			if err != nil {
				o.Log.Error("can't unlock account",
					zap.String("address", address.Uint160ToString(acc.Contract.ScriptHash())),
					zap.Error(err))
				o.currAccount = nil
				return
			}
			break
		}
	}

	o.currAccount = acc
	o.oracleSignContract, _ = smartcontract.CreateDefaultMultiSigRedeemScript(oracleNodes)
	o.oracleNodes = oracleNodes
}

func (o *Oracle) getAccount() *wallet.Account {
	o.accMtx.RLock()
	defer o.accMtx.RUnlock()
	return o.currAccount
}

func (o *Oracle) getOracleNodes() keys.PublicKeys {
	o.accMtx.RLock()
	defer o.accMtx.RUnlock()
	return o.oracleNodes
}

func (o *Oracle) getOracleSignContract() []byte {
	o.accMtx.RLock()
	defer o.accMtx.RUnlock()
	return o.oracleSignContract
}
