package wallet

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/encoding/address"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	walletTemplate = "testWallet"
)

func TestNewWallet(t *testing.T) {
	wallet := checkWalletConstructor(t)
	require.NotNil(t, wallet)
}

func TestNewWalletFromFile_Negative_EmptyFile(t *testing.T) {
	_ = checkWalletConstructor(t)
	walletFromFile, err2 := NewWalletFromFile(walletTemplate)
	require.Errorf(t, err2, "EOF")
	require.Nil(t, walletFromFile)
}

func TestNewWalletFromFile_Negative_NoFile(t *testing.T) {
	_, err := NewWalletFromFile(walletTemplate)
	require.Errorf(t, err, "open testWallet: no such file or directory")
}

func TestCreateAccount(t *testing.T) {
	wallet := checkWalletConstructor(t)

	errAcc := wallet.CreateAccount("testName", "testPass")
	require.NoError(t, errAcc)
	accounts := wallet.Accounts
	require.Len(t, accounts, 1)
}

func TestAddAccount(t *testing.T) {
	wallet := checkWalletConstructor(t)

	wallet.AddAccount(&Account{
		privateKey:   nil,
		publicKey:    nil,
		wif:          "",
		Address:      "",
		EncryptedWIF: "",
		Label:        "",
		Contract:     nil,
		Locked:       false,
		Default:      false,
	})
	accounts := wallet.Accounts
	require.Len(t, accounts, 1)
}

func TestPath(t *testing.T) {
	wallet := checkWalletConstructor(t)

	path := wallet.Path()
	require.NotEmpty(t, path)
}

func TestSave(t *testing.T) {
	file, err := ioutil.TempFile("", walletTemplate)
	require.NoError(t, err)
	wallet, err := NewWallet(file.Name())
	require.NoError(t, err)
	wallet.AddAccount(&Account{
		privateKey:   nil,
		publicKey:    nil,
		wif:          "",
		Address:      "",
		EncryptedWIF: "",
		Label:        "",
		Contract:     nil,
		Locked:       false,
		Default:      false,
	})

	defer removeWallet(t, file.Name())
	errForSave := wallet.Save()
	require.NoError(t, errForSave)

	openedWallet, err := NewWalletFromFile(wallet.path)
	require.NoError(t, err)
	require.Equal(t, wallet.Accounts, openedWallet.Accounts)
}

func TestJSONMarshallUnmarshal(t *testing.T) {
	wallet := checkWalletConstructor(t)

	bytes, err := wallet.JSON()
	require.NoError(t, err)
	require.NotNil(t, bytes)

	unmarshalledWallet := &Wallet{}
	errUnmarshal := json.Unmarshal(bytes, unmarshalledWallet)

	require.NoError(t, errUnmarshal)
	require.Equal(t, wallet.Version, unmarshalledWallet.Version)
	require.Equal(t, wallet.Accounts, unmarshalledWallet.Accounts)
	require.Equal(t, wallet.Scrypt, unmarshalledWallet.Scrypt)
}

func checkWalletConstructor(t *testing.T) *Wallet {
	file, err := ioutil.TempFile("", walletTemplate)
	require.NoError(t, err)
	wallet, err := NewWallet(file.Name())
	defer removeWallet(t, file.Name())
	require.NoError(t, err)
	return wallet
}

func removeWallet(t *testing.T, walletPath string) {
	err := os.RemoveAll(walletPath)
	require.NoError(t, err)
}

func TestWallet_GetAccount(t *testing.T) {
	wallet := checkWalletConstructor(t)
	accounts := []*Account{
		{
			Contract: &Contract{
				Script: []byte{0, 1, 2, 3},
			},
		},
		{
			Contract: &Contract{
				Script: []byte{3, 2, 1, 0},
			},
		},
	}

	for _, acc := range accounts {
		wallet.AddAccount(acc)
	}

	for i, acc := range accounts {
		h := acc.Contract.ScriptHash()
		assert.Equal(t, acc, wallet.GetAccount(h), "can't get %d account", i)
	}
}

func TestWalletGetChangeAddress(t *testing.T) {
	w1, err := NewWalletFromFile("testdata/wallet1.json")
	require.NoError(t, err)
	sh := w1.GetChangeAddress()
	// No default address, the first one is used.
	expected, err := address.StringToUint160("AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs")
	require.NoError(t, err)
	require.Equal(t, expected, sh)
	w2, err := NewWalletFromFile("testdata/wallet2.json")
	require.NoError(t, err)
	sh = w2.GetChangeAddress()
	// Default address.
	expected, err = address.StringToUint160("AWLYWXB8C9Lt1nHdDZJnC5cpYJjgRDLk17")
	require.NoError(t, err)
	require.Equal(t, expected, sh)
}
