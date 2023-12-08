package wallet

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

const (
	// The current version of neo-go wallet implementations.
	walletVersion = "1.0"
)

var (
	// ErrPathIsEmpty appears if wallet was created without linking to file system path,
	// for instance with [NewInMemoryWallet] or [NewWalletFromBytes].
	// Despite this, there was an attempt to save it via [Wallet.Save] or [Wallet.SavePretty] without [Wallet.SetPath].
	ErrPathIsEmpty = errors.New("path is empty")
)

// Wallet represents a NEO (NEP-2, NEP-6) compliant wallet.
type Wallet struct {
	// Version of the wallet, used for later upgrades.
	Version string `json:"version"`

	// A list of accounts which describes the details of each account
	// in the wallet.
	Accounts []*Account `json:"accounts"`

	Scrypt keys.ScryptParams `json:"scrypt"`

	// Extra metadata can be used for storing arbitrary data.
	// This field can be empty.
	Extra Extra `json:"extra"`

	// Path where the wallet file is located..
	path string
}

// Extra stores imported token contracts.
type Extra struct {
	// Tokens is a list of imported token contracts.
	Tokens []*Token
}

// NewWallet creates a new NEO wallet at the given location.
func NewWallet(location string) (*Wallet, error) {
	file, err := os.Create(location)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return newWallet(file), nil
}

// NewInMemoryWallet creates a new NEO wallet without linking to the read file on file system.
// If wallet required to be written to the file system, [Wallet.SetPath] should be used to set the path.
func NewInMemoryWallet() *Wallet {
	return newWallet(nil)
}

// NewWalletFromFile creates a Wallet from the given wallet file path.
func NewWalletFromFile(path string) (*Wallet, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open wallet: %w", err)
	}
	defer file.Close()

	wall := &Wallet{
		path: file.Name(),
	}
	if err := json.NewDecoder(file).Decode(wall); err != nil {
		return nil, fmt.Errorf("unmarshal wallet: %w", err)
	}
	return wall, nil
}

// NewWalletFromBytes creates a [Wallet] from the given byte slice.
// Parameter wallet contains JSON representation of wallet, see [Wallet.JSON] for details.
//
// NewWalletFromBytes constructor doesn't set wallet's path. If you want to save the wallet to file system,
// use [Wallet.SetPath].
func NewWalletFromBytes(wallet []byte) (*Wallet, error) {
	wall := &Wallet{}
	if err := json.NewDecoder(bytes.NewReader(wallet)).Decode(wall); err != nil {
		return nil, fmt.Errorf("unmarshal wallet: %w", err)
	}

	return wall, nil
}

func newWallet(rw io.ReadWriter) *Wallet {
	var path string
	if f, ok := rw.(*os.File); ok {
		path = f.Name()
	}
	return &Wallet{
		Version:  walletVersion,
		Accounts: []*Account{},
		Scrypt:   keys.NEP2ScryptParams(),
		path:     path,
	}
}

// CreateAccount generates a new account for the end user and encrypts
// the private key with the given passphrase.
func (w *Wallet) CreateAccount(name, passphrase string) error {
	acc, err := NewAccount()
	if err != nil {
		return err
	}
	acc.Label = name
	if err := acc.Encrypt(passphrase, w.Scrypt); err != nil {
		return err
	}
	w.AddAccount(acc)
	return w.Save()
}

// AddAccount adds an existing Account to the wallet.
func (w *Wallet) AddAccount(acc *Account) {
	w.Accounts = append(w.Accounts, acc)
}

// RemoveAccount removes an Account with the specified addr
// from the wallet.
func (w *Wallet) RemoveAccount(addr string) error {
	for i, acc := range w.Accounts {
		if acc.Address == addr {
			copy(w.Accounts[i:], w.Accounts[i+1:])
			w.Accounts = w.Accounts[:len(w.Accounts)-1]
			return nil
		}
	}
	return errors.New("account wasn't found")
}

// AddToken adds a new token to a wallet.
func (w *Wallet) AddToken(tok *Token) {
	w.Extra.Tokens = append(w.Extra.Tokens, tok)
}

// RemoveToken removes the token with the specified hash from the wallet.
func (w *Wallet) RemoveToken(h util.Uint160) error {
	for i, tok := range w.Extra.Tokens {
		if tok.Hash.Equals(h) {
			copy(w.Extra.Tokens[i:], w.Extra.Tokens[i+1:])
			w.Extra.Tokens = w.Extra.Tokens[:len(w.Extra.Tokens)-1]
			return nil
		}
	}
	return errors.New("token wasn't found")
}

// Path returns the location of the wallet on the filesystem.
func (w *Wallet) Path() string {
	return w.path
}

// SetPath sets the location of the wallet on the filesystem.
func (w *Wallet) SetPath(path string) {
	w.path = path
}

// Save saves the wallet data to the file located at the path that was either provided
// via [NewWalletFromFile] constructor or via [Wallet.SetPath].
//
// Returns [ErrPathIsEmpty] if wallet path is not set. See [Wallet.SetPath].
func (w *Wallet) Save() error {
	data, err := json.Marshal(w)
	if err != nil {
		return err
	}

	return w.writeRaw(data)
}

// SavePretty saves the wallet in a beautiful JSON.
//
// Returns [ErrPathIsEmpty] if wallet path is not set. See [Wallet.SetPath].
func (w *Wallet) SavePretty() error {
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return err
	}

	return w.writeRaw(data)
}

func (w *Wallet) writeRaw(data []byte) error {
	if w.path == "" {
		return ErrPathIsEmpty
	}

	return os.WriteFile(w.path, data, 0644)
}

// JSON outputs a pretty JSON representation of the wallet.
func (w *Wallet) JSON() ([]byte, error) {
	return json.MarshalIndent(w, " ", "	")
}

// Close closes all Wallet accounts making them incapable of signing anything
// (unless they're decrypted again). It's not doing anything to the underlying
// wallet file.
func (w *Wallet) Close() {
	for _, acc := range w.Accounts {
		acc.Close()
	}
}

// GetAccount returns an account corresponding to the provided scripthash.
func (w *Wallet) GetAccount(h util.Uint160) *Account {
	addr := address.Uint160ToString(h)
	for _, acc := range w.Accounts {
		if acc.Address == addr {
			return acc
		}
	}

	return nil
}

// GetChangeAddress returns the default address to send transaction's change to.
func (w *Wallet) GetChangeAddress() util.Uint160 {
	var res util.Uint160
	var acc *Account

	for i := range w.Accounts {
		if acc == nil || w.Accounts[i].Default {
			if w.Accounts[i].Contract != nil && vm.IsSignatureContract(w.Accounts[i].Contract.Script) {
				acc = w.Accounts[i]
				if w.Accounts[i].Default {
					break
				}
			}
		}
	}
	if acc != nil {
		res = acc.Contract.ScriptHash()
	}
	return res
}
