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
		if want, have := testCase.address, wall.Address; want != have {
			t.Fatalf("expected %s got %s", want, have)
		}
		if want, have := testCase.wif, wall.WIF; want != have {
			t.Fatalf("expected %s got %s", want, have)
		}
		if want, have := testCase.publicKey, hex.EncodeToString(wall.PublicKey); want != have {
			t.Fatalf("expected %s got %s", want, have)
		}
		if want, have := testCase.privateKey, wall.PrivateKey.String(); want != have {
			t.Fatalf("expected %s got %s", want, have)
		}
	}
}

func TestDecryptWallet(t *testing.T) {
	for _, testCase := range testKeyCases {
		wif, err := NEP2Decrypt(testCase.encryptedWif, testCase.passphrase)
		if err != nil {
			t.Fatal(err)
		}
		if want, have := testCase.wif, wif; want != have {
			t.Fatalf("expected %s got %s", want, have)
		}
	}
}
