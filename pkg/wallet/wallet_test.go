package wallet

import (
	"encoding/hex"
	"testing"
)

func TestNewWallet(t *testing.T) {
	for _, testCase := range testKeyCases {
		wall, err := NewFromWIF(testCase.wif)
		if err != nil {
			t.Fatal(err)
		}
		compareFields(t, testCase, wall)
	}
}

func TestDecryptWallet(t *testing.T) {
	for _, testCase := range testKeyCases {
		wall, err := Decrypt(testCase.encryptedWif, testCase.passphrase)
		if err != nil {
			t.Fatal(err)
		}
		compareFields(t, testCase, wall)
	}
}

func TestNewFromWif(t *testing.T) {
	for _, testCase := range testKeyCases {
		wall, err := NewFromWIF(testCase.wif)
		if err != nil {
			t.Fatal(err)
		}
		compareFields(t, testCase, wall)
	}
}

func compareFields(t *testing.T, tk testKey, wall *Wallet) {
	if want, have := tk.address, wall.Address; want != have {
		t.Fatalf("expected %s got %s", want, have)
	}
	if want, have := tk.wif, wall.WIF; want != have {
		t.Fatalf("expected %s got %s", want, have)
	}
	if want, have := tk.publicKey, hex.EncodeToString(wall.PublicKey); want != have {
		t.Fatalf("expected %s got %s", want, have)
	}
	if want, have := tk.privateKey, wall.PrivateKey.String(); want != have {
		t.Fatalf("expected %s got %s", want, have)
	}
}
