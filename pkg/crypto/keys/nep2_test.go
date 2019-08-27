package keys

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/internal/keytestcases"
)

func TestNEP2Encrypt(t *testing.T) {
	for _, testCase := range keytestcases.Arr {

		privKey, err := NewPrivateKeyFromHex(testCase.PrivateKey)
		if err != nil {
			t.Fatal(err)
		}

		encryptedWif, err := NEP2Encrypt(privKey, testCase.Passphrase)
		if err != nil {
			t.Fatal(err)
		}

		if want, have := testCase.EncryptedWif, encryptedWif; want != have {
			t.Fatalf("expected %s got %s", want, have)
		}
	}
}

func TestNEP2Decrypt(t *testing.T) {
	for _, testCase := range keytestcases.Arr {

		privKeyString, err := NEP2Decrypt(testCase.EncryptedWif, testCase.Passphrase)

		if err != nil {
			t.Fatal(err)
		}

		privKey, err := NewPrivateKeyFromWIF(privKeyString)
		if err != nil {
			t.Fatal(err)
		}

		if want, have := testCase.PrivateKey, privKey.String(); want != have {
			t.Fatalf("expected %s got %s", want, have)
		}

		wif, err := privKey.WIF()
		if err != nil {
			t.Fatal(err)
		}
		if want, have := testCase.Wif, wif; want != have {
			t.Fatalf("expected %s got %s", want, have)
		}

		address, err := privKey.Address()
		if err != nil {
			t.Fatal(err)
		}
		if want, have := testCase.Address, address; want != have {
			t.Fatalf("expected %s got %s", want, have)
		}
	}
}
