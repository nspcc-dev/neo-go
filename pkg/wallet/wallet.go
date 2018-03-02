package wallet

import (
	"encoding/json"
	"io"
	"os"
)

const (
	// The current version of neo-go wallet implementations.
	walletVersion = "1.0"
)

// Wallet respresents a NEO (NEP-2, NEP-6) compliant wallet.
type Wallet struct {
	// Version of the wallet, used for later upgrades.
	Version string `json:"version"`

	// A list of accounts which decribes the details of each account
	// in the wallet.
	Accounts []*Account `json:"accounts"`

	Scrypt scryptParams `json:"scrypt"`

	// Extra metadata can be used for storing abritrary data.
	// This field can be empty.
	Extra interface{} `json:"extra"`

	// Path where the wallet file is located..
	path string

	// ReadWriter for reading and writing wallet data.
	rw io.ReadWriter
}

// NewWallet creates a new NEO wallet at the given location.
func NewWallet(location string) (*Wallet, error) {
	file, err := os.Create(location)
	if err != nil {
		return nil, err
	}
	return newWallet(file), nil
}

// NewWalletFromFile creates a Wallet from the given wallet file path
func NewWalletFromFile(path string) (*Wallet, error) {
	file, err := os.OpenFile(path, os.O_RDWR, os.ModeAppend)
	if err != nil {
		return nil, err
	}
	wall := &Wallet{
		rw:   file,
		path: file.Name(),
	}
	if err := json.NewDecoder(file).Decode(wall); err != nil {
		return nil, err
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
		Scrypt:   newScryptParams(),
		rw:       rw,
		path:     path,
	}
}

// CreatAccount generates a new account for the end user and ecrypts
// the private key with the given passphrase.
func (w *Wallet) CreateAccount(name, passphrase string) error {
	acc, err := NewAccount()
	if err != nil {
		return err
	}
	acc.Label = name
	if err := acc.Encrypt(passphrase); err != nil {
		return err
	}
	w.AddAccount(acc)
	return w.Save()
}

// AddAccount adds an existing Account to the wallet.
func (w *Wallet) AddAccount(acc *Account) {
	w.Accounts = append(w.Accounts, acc)
}

// Path returns the location of the wallet on the filesystem.
func (w *Wallet) Path() string {
	return w.path
}

// Save saves the wallet data. It's the internal io.ReadWriter
// that is responsible for saving the data. This can
// be a buffer, file, etc..
func (w *Wallet) Save() error {
	return json.NewEncoder(w.rw).Encode(w)
}

// JSON outputs a pretty JSON representation of the wallet.
func (w *Wallet) JSON() ([]byte, error) {
	return json.MarshalIndent(w, " ", "	")
}

// Close closes the internal rw if its an io.ReadCloser.
func (w *Wallet) Close() {
	if rc, ok := w.rw.(io.ReadCloser); ok {
		rc.Close()
	}
}
