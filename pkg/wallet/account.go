package wallet

import "github.com/CityOfZion/neo-go/pkg/util"

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

// Contract represents a subset of the smartcontract to embedd in the
// Account so it's NEP-6 compliant.
type Contract struct {
	// Script hash of the contract deployed on the blockchain.
	Script util.Uint160 `json:"script"`

	// A list of parameters used deploying this contract.
	Parameters []interface{} `json:"parameters"`

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

// DecryptAccount decrypt the encryptedWIF with the given passphrase and
// return the decrypted Account.
func DecryptAccount(encryptedWIF, passphrase string) (*Account, error) {
	wif, err := NEP2Decrypt(encryptedWIF, passphrase)
	if err != nil {
		return nil, err
	}
	return NewAccountFromWIF(wif)
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

	a := &Account{
		publicKey:  pubKey,
		privateKey: p,
		Address:    pubAddr,
		wif:        wif,
	}

	return a, nil
}
