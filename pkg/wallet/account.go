package wallet

import (
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/util"
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
	Default bool `json:"isDefault"`
}

// Contract represents a subset of the smartcontract to embed in the
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
	priv, err := keys.NewPrivateKey()
	if err != nil {
		return nil, err
	}
	return newAccountFromPrivateKey(priv), nil
}

// DecryptAccount decrypts the encryptedWIF with the given passphrase and
// return the decrypted Account.
func DecryptAccount(encryptedWIF, passphrase string) (*Account, error) {
	key, err := keys.NEP2Decrypt(encryptedWIF, passphrase)
	if err != nil {
		return nil, err
	}
	return newAccountFromPrivateKey(key), nil
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
	return newAccountFromPrivateKey(privKey), nil
}

// newAccountFromPrivateKey creates a wallet from the given PrivateKey.
func newAccountFromPrivateKey(p *keys.PrivateKey) *Account {
	pubKey := p.PublicKey()
	pubAddr := p.Address()
	wif := p.WIF()

	a := &Account{
		publicKey:  pubKey.Bytes(),
		privateKey: p,
		Address:    pubAddr,
		wif:        wif,
	}

	return a
}
