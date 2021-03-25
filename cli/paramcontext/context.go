package paramcontext

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/context"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

// validUntilBlockIncrement is the number of extra blocks to add to an exported transaction
const validUntilBlockIncrement = 50

// InitAndSave creates incompletely signed transaction which can used
// as input to `multisig sign`.
func InitAndSave(net netmode.Magic, tx *transaction.Transaction, acc *wallet.Account, filename string) error {
	// avoid fast transaction expiration
	tx.ValidUntilBlock += validUntilBlockIncrement
	priv := acc.PrivateKey()
	pub := priv.PublicKey()
	sign := priv.SignHashable(uint32(net), tx)
	scCtx := context.NewParameterContext("Neo.Core.ContractTransaction", net, tx)
	h, err := address.StringToUint160(acc.Address)
	if err != nil {
		return fmt.Errorf("invalid address: %s", acc.Address)
	}
	if err := scCtx.AddSignature(h, acc.Contract, pub, sign); err != nil {
		return fmt.Errorf("can't add signature: %w", err)
	}
	return Save(scCtx, filename)
}

// Read reads parameter context from file.
func Read(filename string) (*context.ParameterContext, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("can't read input file: %w", err)
	}

	c := new(context.ParameterContext)
	if err := json.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("can't parse transaction: %w", err)
	}
	return c, nil
}

// Save writes parameter context to file.
func Save(c *context.ParameterContext, filename string) error {
	if data, err := json.Marshal(c); err != nil {
		return fmt.Errorf("can't marshal transaction: %w", err)
	} else if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("can't write transaction to file: %w", err)
	}
	return nil
}
