package wallet

import (
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
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
	// Script of the contract deployed on the blockchain.
	Script []byte `json:"script"`

	// A list of parameters used deploying this contract.
	Parameters []interface{} `json:"parameters"`

	// Indicates whether the contract has been deployed to the blockchain.
	Deployed bool `json:"deployed"`
}

// contract is an intermediate struct used for json unmarshalling.
type contract struct {
	// Script is a hex-encoded script of the contract.
	Script string `json:"script"`

	// A list of parameters used deploying this contract.
	Parameters []interface{} `json:"parameters"`

	// Indicates whether the contract has been deployed to the blockchain.
	Deployed bool `json:"deployed"`
}

// MarshalJSON implements json.Marshaler interface.
func (c Contract) MarshalJSON() ([]byte, error) {
	var cc contract

	cc.Script = hex.EncodeToString(c.Script)
	cc.Parameters = c.Parameters
	cc.Deployed = c.Deployed

	return json.Marshal(cc)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (c *Contract) UnmarshalJSON(data []byte) error {
	var cc contract

	if err := json.Unmarshal(data, &cc); err != nil {
		return err
	}

	script, err := hex.DecodeString(cc.Script)
	if err != nil {
		return err
	}

	c.Script = script
	c.Parameters = cc.Parameters
	c.Deployed = cc.Deployed

	return nil
}

// NewAccount creates a new Account with a random generated PrivateKey.
func NewAccount() (*Account, error) {
	priv, err := keys.NewPrivateKey()
	if err != nil {
		return nil, err
	}
	return newAccountFromPrivateKey(priv), nil
}

// Decrypt decrypts the EncryptedWIF with the given passphrase returning error
// if anything goes wrong.
func (a *Account) Decrypt(passphrase string) error {
	var err error

	if a.EncryptedWIF == "" {
		return errors.New("no encrypted wif in the account")
	}
	a.privateKey, err = keys.NEP2Decrypt(a.EncryptedWIF, passphrase)
	return err
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
