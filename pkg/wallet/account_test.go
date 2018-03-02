package wallet

import (
	"encoding/hex"
	"testing"
)

func TestNewAccount(t *testing.T) {
	for _, testCase := range testKeyCases {
		acc, err := NewAccountFromWIF(testCase.wif)
		if err != nil {
			t.Fatal(err)
		}
		compareFields(t, testCase, acc)
	}
}

func TestDecryptAccount(t *testing.T) {
	for _, testCase := range testKeyCases {
		acc, err := DecryptAccount(testCase.encryptedWif, testCase.passphrase)
		if err != nil {
			t.Fatal(err)
		}
		compareFields(t, testCase, acc)
	}
}

func TestNewFromWif(t *testing.T) {
	for _, testCase := range testKeyCases {
		acc, err := NewAccountFromWIF(testCase.wif)
		if err != nil {
			t.Fatal(err)
		}
		compareFields(t, testCase, acc)
	}
}

func compareFields(t *testing.T, tk testKey, acc *Account) {
	if want, have := tk.address, acc.Address; want != have {
		t.Fatalf("expected %s got %s", want, have)
	}
	if want, have := tk.wif, acc.wif; want != have {
		t.Fatalf("expected %s got %s", want, have)
	}
	if want, have := tk.publicKey, hex.EncodeToString(acc.publicKey); want != have {
		t.Fatalf("expected %s got %s", want, have)
	}
	if want, have := tk.privateKey, acc.privateKey.String(); want != have {
		t.Fatalf("expected %s got %s", want, have)
	}
}
