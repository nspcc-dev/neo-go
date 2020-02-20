package wallet

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/encoding/address"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
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
	// Script of the contract deployed on the blockchain.
	Script []byte `json:"script"`

	// A list of parameters used deploying this contract.
	Parameters []contractParam `json:"parameters"`

	// Indicates whether the contract has been deployed to the blockchain.
	Deployed bool `json:"deployed"`
}

// contract is an intermediate struct used for json unmarshalling.
type contract struct {
	// Script is a hex-encoded script of the contract.
	Script string `json:"script"`

	// A list of parameters used deploying this contract.
	Parameters []contractParam `json:"parameters"`

	// Indicates whether the contract has been deployed to the blockchain.
	Deployed bool `json:"deployed"`
}

type contractParam struct {
	Name string                  `json:"name"`
	Type smartcontract.ParamType `json:"type"`
}

// ScriptHash returns the hash of contract's script.
func (c Contract) ScriptHash() util.Uint160 {
	return hash.Hash160(c.Script)
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
	return newAccountFromPrivateKey(privKey), nil
}

// NewAccountFromEncryptedWIF creates a new Account from the given encrypted WIF.
func NewAccountFromEncryptedWIF(wif string, pass string) (*Account, error) {
	priv, err := keys.NEP2Decrypt(wif, pass)
	if err != nil {
		return nil, err
	}

	a := newAccountFromPrivateKey(priv)
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
		Contract: &Contract{
			Script:     pubKey.GetVerificationScript(),
			Parameters: getContractParams(1),
		},
	}

	return a
}

func getContractParams(n int) []contractParam {
	params := make([]contractParam, n)
	for i := range params {
		params[i].Name = fmt.Sprintf("parameter%d", i)
		params[i].Type = smartcontract.SignatureType
	}

	return params
}
