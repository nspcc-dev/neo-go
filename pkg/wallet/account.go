package wallet

import (
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

// Account represents a NEO account. It holds the private and the public key
// along with some metadata.
type Account struct {
	// NEO private key.
	privateKey *keys.PrivateKey

	// Script hash corresponding to the Address.
	scriptHash util.Uint160

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
	var (
		haveAcc bool
		pos     int
	)
	if a.Locked {
		return errors.New("account is locked")
	}
	if a.Contract == nil {
		return errors.New("account has no contract")
	}
	for i := range t.Signers {
		if t.Signers[i].Account.Equals(a.ScriptHash()) {
			haveAcc = true
			pos = i
			break
		}
	}
	if !haveAcc {
		return errors.New("transaction is not signed by this account")
	}
	if len(t.Scripts) < pos {
		return errors.New("transaction is not yet signed by the previous signer")
	}
	if len(t.Scripts) == pos {
		t.Scripts = append(t.Scripts, transaction.Witness{
			VerificationScript: a.Contract.Script, // Can be nil for deployed contract.
		})
	}
	if len(a.Contract.Parameters) == 0 {
		return nil
	}
	if a.privateKey == nil {
		return errors.New("account key is not available (need to decrypt?)")
	}
	sign := a.privateKey.SignHashable(uint32(net), t)

	invoc := append([]byte{byte(opcode.PUSHDATA1), 64}, sign...)
	if len(a.Contract.Parameters) == 1 {
		t.Scripts[pos].InvocationScript = invoc
	} else {
		t.Scripts[pos].InvocationScript = append(t.Scripts[pos].InvocationScript, invoc...)
	}

	return nil
}

// SignHashable signs the given Hashable item and returns the signature. If this
// account can't sign (CanSign() returns false) nil is returned.
func (a *Account) SignHashable(net netmode.Magic, item hash.Hashable) []byte {
	if !a.CanSign() {
		return nil
	}
	return a.privateKey.SignHashable(uint32(net), item)
}

// CanSign returns true when account is not locked and has a decrypted private
// key inside, so it's ready to create real signatures.
func (a *Account) CanSign() bool {
	return !a.Locked && a.privateKey != nil
}

// GetVerificationScript returns account's verification script.
func (a *Account) GetVerificationScript() []byte {
	if a.Contract != nil {
		return a.Contract.Script
	}
	return a.privateKey.PublicKey().GetVerificationScript()
}

// Decrypt decrypts the EncryptedWIF with the given passphrase returning error
// if anything goes wrong.
func (a *Account) Decrypt(passphrase string, scrypt keys.ScryptParams) error {
	var err error

	if a.EncryptedWIF == "" {
		return errors.New("no encrypted wif in the account")
	}
	a.privateKey, err = keys.NEP2Decrypt(a.EncryptedWIF, passphrase, scrypt)
	if err != nil {
		return err
	}

	return nil
}

// Encrypt encrypts the wallet's PrivateKey with the given passphrase
// under the NEP-2 standard.
func (a *Account) Encrypt(passphrase string, scrypt keys.ScryptParams) error {
	wif, err := keys.NEP2Encrypt(a.privateKey, passphrase, scrypt)
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

// PublicKey returns the public key associated with the private key corresponding to
// the account. It can return nil if account is locked (use CanSign to check).
func (a *Account) PublicKey() *keys.PublicKey {
	if !a.CanSign() {
		return nil
	}
	return a.privateKey.PublicKey()
}

// ScriptHash returns the script hash (account) that the Account.Address is
// derived from. It never returns an error, so if this Account has an invalid
// Address you'll just get a zero script hash.
func (a *Account) ScriptHash() util.Uint160 {
	if a.scriptHash.Equals(util.Uint160{}) {
		a.scriptHash, _ = address.StringToUint160(a.Address)
	}
	return a.scriptHash
}

// Close cleans up the private key used by Account and disassociates it from
// Account. The Account can no longer sign anything after this call, but Decrypt
// can make it usable again.
func (a *Account) Close() {
	if a.privateKey == nil {
		return
	}
	a.privateKey.Destroy()
	a.privateKey = nil
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
func NewAccountFromEncryptedWIF(wif string, pass string, scrypt keys.ScryptParams) (*Account, error) {
	priv, err := keys.NEP2Decrypt(wif, pass, scrypt)
	if err != nil {
		return nil, err
	}

	a := NewAccountFromPrivateKey(priv)
	a.EncryptedWIF = wif

	return a, nil
}

// ConvertMultisig sets a's contract to multisig contract with m sufficient signatures.
func (a *Account) ConvertMultisig(m int, pubs []*keys.PublicKey) error {
	if a.Locked {
		return errors.New("account is locked")
	}
	if a.privateKey == nil {
		return errors.New("account key is not available (need to decrypt?)")
	}
	var found bool
	accKey := a.privateKey.PublicKey()
	for i := range pubs {
		if accKey.Equal(pubs[i]) {
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

	a.scriptHash = hash.Hash160(script)
	a.Address = address.Uint160ToString(a.scriptHash)
	a.Contract = &Contract{
		Script:     script,
		Parameters: getContractParams(m),
	}

	return nil
}

// NewAccountFromPrivateKey creates a wallet from the given PrivateKey.
func NewAccountFromPrivateKey(p *keys.PrivateKey) *Account {
	pubKey := p.PublicKey()

	a := &Account{
		privateKey: p,
		scriptHash: p.GetScriptHash(),
		Address:    p.Address(),
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
