package core

import (
	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// References returns a map with input prevHash as key (util.Uint256)
// and transaction output as value from a transaction t.
// @TODO: unfortunately we couldn't attach this method to the Transaction struct in the
// transaction package because of a import cycle problem. Perhaps we should think to re-design
// the code base to avoid this situation.
func References(t *transaction.Transaction) map[util.Uint256]*transaction.Output {
	references := make(map[util.Uint256]*transaction.Output)

	for prevHash, inputs := range t.GroupInputsByPrevHash() {
		if BlockchainDefault == nil {
			panic("no default blockchain available! please register one.")
		} else if tx, _, err := BlockchainDefault.GetTransaction(prevHash); err != nil {
			tx = nil
		} else if tx != nil {
			for _, in := range inputs {
				references[in.PrevHash] = tx.Outputs[in.PrevIndex]
			}
		} else {
			references = nil
		}
	}
	return references
}

// FeePerByte returns network fee divided by the size of the transaction
func FeePerByte(t *transaction.Transaction) util.Fixed8 {
	return NetworkFee(t).Div(t.Size())
}

// NetworkFee returns network fee
func NetworkFee(t *transaction.Transaction) util.Fixed8 {
	inputAmount := util.NewFixed8(0)
	for _, txOutput := range References(t) {
		if txOutput.AssetID == utilityTokenTX().Hash() {
			inputAmount.Add(txOutput.Amount)
		}
	}

	outputAmount := util.NewFixed8(0)
	for _, txOutput := range t.Outputs {
		if txOutput.AssetID == utilityTokenTX().Hash() {
			outputAmount.Add(txOutput.Amount)
		}
	}

	return inputAmount.Sub(outputAmount).Sub(SystemFee(t))
}

// SystemFee returns system fee
func SystemFee(t *transaction.Transaction) util.Fixed8 {
	return config.ConfigDefault.ProtocolConfiguration.SystemFee.TryGetValue(t.Type)
}
