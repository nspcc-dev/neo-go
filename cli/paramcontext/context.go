package paramcontext

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/context"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

// InitAndSave creates an incompletely signed transaction which can be used
// as an input to `multisig sign`. If a wallet.Account is given and can sign,
// it's signed as well using it.
func InitAndSave(net netmode.Magic, tx *transaction.Transaction, acc *wallet.Account, filename string) error {
	scCtx := context.NewParameterContext("Neo.Network.P2P.Payloads.Transaction", net, tx)
	if acc != nil && acc.CanSign() {
		priv := acc.PrivateKey()
		pub := priv.PublicKey()
		sign := priv.SignHashable(uint32(net), tx)
		h, err := address.StringToUint160(acc.Address)
		if err != nil {
			return fmt.Errorf("invalid address: %s", acc.Address)
		}
		if err := scCtx.AddSignature(h, acc.Contract, pub, sign); err != nil {
			return fmt.Errorf("can't add signature: %w", err)
		}
	}
	return Save(scCtx, filename)
}

// Read reads the parameter context from the file.
func Read(filename string) (*context.ParameterContext, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("can't read input file: %w", err)
	}

	c := new(context.ParameterContext)
	if err := json.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("can't parse transaction: %w", err)
	}
	return c, nil
}

// Save writes the parameter context to the file.
func Save(c *context.ParameterContext, filename string) error {
	if data, err := json.Marshal(c); err != nil {
		return fmt.Errorf("can't marshal transaction: %w", err)
	} else if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("can't write transaction to file: %w", err)
	}
	return nil
}
