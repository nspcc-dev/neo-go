package wallet

import (
	"testing"
)

func TestNewWallet(t *testing.T) {
	wall, err := NewWalletFromFile("/Users/anthony/Documents/wallet.wal")
	if err != nil {
		t.Fatal(err)
	}
	if err := wall.CreateAccount("help"); err != nil {
		t.Fatal(err)
	}
}
