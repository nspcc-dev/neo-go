package wallet

import "github.com/CityOfZeon/neo-go/pkg/util"

// Account represents a NEO account. It holds the private and public key
// along with some metadata.
type Account struct {
	// NEO private key.
	privateKey *PrivateKey

	// NEO public key.
	publicKey []byte

	// Account import file.
	wif string

	// NEO public addresss.
	Address string `json:"address"`

	// Encrypted WIF of the account also known as the key.
	EncryptedWIF string `json:"key"`

	// Label is a label the user had made for this account.
	Label string `json:"label"`

	// contract is a Contract object which describes the details of the contract.
	// This field can be null (for watch-only address).
	Contract *Contract `json:"contract"`

	// Indicates whether the account is locked by the user.
	// the client shouldn't spend the funds in a locked account.
	Locked bool `json:"lock"`

	// Indicates whether the account is the default change account.
	Default bool `json:"isDefault"`
}

type ContractParameter struct {
	Type byte
	Name string
}

// Contract represents a subset of the smartcontract to embed in the
// Account so it's NEP-6 compliant.
type Contract struct {
	// Script hash of the contract deployed on the blockchain.
	Script util.Uint160 `json:"script"`

	// A list of parameters used for deploying this contract.
	Parameters []ContractParameter `json:"parameters"`

	// Indicates whether the contract has been deployed to the blockchain.
	Deployed bool `json:"deployed"`
}

// NewAccount creates a new Account with a random generated PrivateKey.
func NewAccount() (*Account, error) {
	priv, err := NewPrivateKey()
	if err != nil {
		return nil, err
	}
	return newAccountFromPrivateKey(priv)
}

// Decrypt tries to decrypt the account with the given passphrase and returns
// whether the operation was successfull.
func (a *Account) Decrypt(passphrase string) bool {
	wif, err := NEP2Decrypt(a.EncryptedWIF, passphrase)
	if err != nil {
		return false
	}
	a.wif = wif
	return true
}

// Encrypt encrypts the wallet's PrivateKey with the given passphrase
// under the NEP-2 standard.
func (a *Account) Encrypt(passphrase string) error {
	wif, err := NEP2Encrypt(a.privateKey, passphrase)
	if err != nil {
		return err
	}
	a.EncryptedWIF = wif
	return nil
}

// NewAccountFromWIF creates a new Account from the given WIF.
func NewAccountFromWIF(wif string) (*Account, error) {
	privKey, err := NewPrivateKeyFromWIF(wif)
	if err != nil {
		return nil, err
	}
	return newAccountFromPrivateKey(privKey)
}

// newAccountFromPrivateKey created a wallet from the given PrivateKey.
func newAccountFromPrivateKey(p *PrivateKey) (*Account, error) {
	pubKey, err := p.PublicKey()
	if err != nil {
		return nil, err
	}
	pubAddr, err := p.Address()
	if err != nil {
		return nil, err
	}
	wif, err := p.WIF()
	if err != nil {
		return nil, err
	}

	// TODO(pawan) - We can store public key in KeyPair struct instead of private key,
	// so that we don't recalculate it.
	sh, err := p.ScriptHashUint160()
	if err != nil {
		return nil, err
	}
	c := &Contract{
		Script: sh,
		Parameters: []ContractParameter{ContractParameter{
			Name: "signature",
			Type: Signature,
		}},
	}

	a := &Account{
		publicKey:  pubKey,
		privateKey: p,
		Address:    pubAddr,
		Contract:   c,
		wif:        wif,
	}

	return a, nil
}
