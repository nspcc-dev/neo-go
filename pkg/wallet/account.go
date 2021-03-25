package wallet

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// Account represents a NEO account. It holds the private and public key
// along with some metadata.
type Account struct {
	// NEO private key.
	privateKey *keys.PrivateKey

	// NEO public key.
	publicKey []byte

	// Account import file.
	wif string

	// NEO public address.
	Address string `json:"address"`

	// Encrypted WIF of the account also known as the key.
	EncryptedWIF string `json:"key"`

	// Label is a label the user had made for this account.
	Label string `json:"label"`

	// Contract is a Contract object which describes the details of the contract.
	// This field can be null (for watch-only address).
	Contract *Contract `json:"contract"`

	// Indicates whether the account is locked by the user.
	// the client shouldn't spend the funds in a locked account.
	Locked bool `json:"lock"`

	// Indicates whether the account is the default change account.
	Default bool `json:"isdefault"`
}

// Contract represents a subset of the smartcontract to embed in the
// Account so it's NEP-6 compliant.
type Contract struct {
	// Script of the contract deployed on the blockchain.
	Script []byte `json:"script"`

	// A list of parameters used deploying this contract.
	Parameters []ContractParam `json:"parameters"`

	// Indicates whether the contract has been deployed to the blockchain.
	Deployed bool `json:"deployed"`
}

// contract is an intermediate struct used for json unmarshalling.
type contract struct {
	// Script is a hex-encoded script of the contract.
	Script string `json:"script"`

	// A list of parameters used deploying this contract.
	Parameters []ContractParam `json:"parameters"`

	// Indicates whether the contract has been deployed to the blockchain.
	Deployed bool `json:"deployed"`
}

// ContractParam is a descriptor of a contract parameter
// containing type and optional name.
type ContractParam struct {
	Name string                  `json:"name"`
	Type smartcontract.ParamType `json:"type"`
}

// ScriptHash returns the hash of contract's script.
func (c Contract) ScriptHash() util.Uint160 {
	return hash.Hash160(c.Script)
}

// NewAccount creates a new Account with a random generated PrivateKey.
func NewAccount() (*Account, error) {
	priv, err := keys.NewPrivateKey()
	if err != nil {
		return nil, err
	}
	return NewAccountFromPrivateKey(priv), nil
}

// SignTx signs transaction t and updates it's Witnesses.
func (a *Account) SignTx(net netmode.Magic, t *transaction.Transaction) error {
	if a.privateKey == nil {
		return errors.New("account is not unlocked")
	}
	if len(a.Contract.Parameters) == 0 {
		t.Scripts = append(t.Scripts, transaction.Witness{})
		return nil
	}
	sign := a.privateKey.SignHashable(uint32(net), t)

	verif := a.GetVerificationScript()
	invoc := append([]byte{byte(opcode.PUSHDATA1), 64}, sign...)
	for i := range t.Scripts {
		if bytes.Equal(t.Scripts[i].VerificationScript, verif) {
			t.Scripts[i].InvocationScript = append(t.Scripts[i].InvocationScript, invoc...)
			return nil
		}
	}
	t.Scripts = append(t.Scripts, transaction.Witness{
		InvocationScript:   invoc,
		VerificationScript: verif,
	})

	return nil
}

// GetVerificationScript returns account's verification script.
func (a *Account) GetVerificationScript() []byte {
	if a.Contract != nil {
		return a.Contract.Script
	}
	return a.PrivateKey().PublicKey().GetVerificationScript()
}

// Decrypt decrypts the EncryptedWIF with the given passphrase returning error
// if anything goes wrong.
func (a *Account) Decrypt(passphrase string) error {
	var err error

	if a.EncryptedWIF == "" {
		return errors.New("no encrypted wif in the account")
	}
	a.privateKey, err = keys.NEP2Decrypt(a.EncryptedWIF, passphrase)
	if err != nil {
		return err
	}

	a.publicKey = a.privateKey.PublicKey().Bytes()
	a.wif = a.privateKey.WIF()

	return nil
}

// Encrypt encrypts the wallet's PrivateKey with the given passphrase
// under the NEP-2 standard.
func (a *Account) Encrypt(passphrase string) error {
	wif, err := keys.NEP2Encrypt(a.privateKey, passphrase)
	if err != nil {
		return err
	}
	a.EncryptedWIF = wif
	return nil
}

// PrivateKey returns private key corresponding to the account.
func (a *Account) PrivateKey() *keys.PrivateKey {
	return a.privateKey
}

// NewAccountFromWIF creates a new Account from the given WIF.
func NewAccountFromWIF(wif string) (*Account, error) {
	privKey, err := keys.NewPrivateKeyFromWIF(wif)
	if err != nil {
		return nil, err
	}
	return NewAccountFromPrivateKey(privKey), nil
}

// NewAccountFromEncryptedWIF creates a new Account from the given encrypted WIF.
func NewAccountFromEncryptedWIF(wif string, pass string) (*Account, error) {
	priv, err := keys.NEP2Decrypt(wif, pass)
	if err != nil {
		return nil, err
	}

	a := NewAccountFromPrivateKey(priv)
	a.EncryptedWIF = wif

	return a, nil
}

// ConvertMultisig sets a's contract to multisig contract with m sufficient signatures.
func (a *Account) ConvertMultisig(m int, pubs []*keys.PublicKey) error {
	var found bool
	for i := range pubs {
		if bytes.Equal(a.publicKey, pubs[i].Bytes()) {
			found = true
			break
		}
	}

	if !found {
		return errors.New("own public key was not found among multisig keys")
	}

	script, err := smartcontract.CreateMultiSigRedeemScript(m, pubs)
	if err != nil {
		return err
	}

	a.Address = address.Uint160ToString(hash.Hash160(script))
	a.Contract = &Contract{
		Script:     script,
		Parameters: getContractParams(m),
	}

	return nil
}

// NewAccountFromPrivateKey creates a wallet from the given PrivateKey.
func NewAccountFromPrivateKey(p *keys.PrivateKey) *Account {
	pubKey := p.PublicKey()
	pubAddr := p.Address()
	wif := p.WIF()

	a := &Account{
		publicKey:  pubKey.Bytes(),
		privateKey: p,
		Address:    pubAddr,
		wif:        wif,
		Contract: &Contract{
			Script:     pubKey.GetVerificationScript(),
			Parameters: getContractParams(1),
		},
	}

	return a
}

func getContractParams(n int) []ContractParam {
	params := make([]ContractParam, n)
	for i := range params {
		params[i].Name = fmt.Sprintf("parameter%d", i)
		params[i].Type = smartcontract.SignatureType
	}

	return params
}
