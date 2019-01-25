package wallet

import (
	"testing"
)

func TestNEP2Encrypt(t *testing.T) {
	for _, testCase := range testKeyCases {

		privKey, err := NewPrivateKeyFromHex(testCase.privateKey)
		if err != nil {
			t.Fatal(err)
		}

		encryptedWif, err := NEP2Encrypt(privKey, testCase.passphrase)
		if err != nil {
			t.Fatal(err)
		}

		if want, have := testCase.encryptedWif, encryptedWif; want != have {
			t.Fatalf("expected %s got %s", want, have)
		}
	}
}

func TestNEP2Decrypt(t *testing.T) {
	for _, testCase := range testKeyCases {

		privKeyString, err := NEP2Decrypt(testCase.encryptedWif, testCase.passphrase)

		if err != nil {
			t.Fatal(err)
		}

		privKey, err := NewPrivateKeyFromWIF(privKeyString)
		if err != nil {
			t.Fatal(err)
		}

		if want, have := testCase.privateKey, privKey.String(); want != have {
			t.Fatalf("expected %s got %s", want, have)
		}

		wif, err := privKey.WIF()
		if err != nil {
			t.Fatal(err)
		}
		if want, have := testCase.wif, wif; want != have {
			t.Fatalf("expected %s got %s", want, have)
		}

		address, err := privKey.Address()
		if err != nil {
			t.Fatal(err)
		}
		if want, have := testCase.address, address; want != have {
			t.Fatalf("expected %s got %s", want, have)
		}
	}
}
