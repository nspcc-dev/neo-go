package keys

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/CityOfZion/neo-go/pkg/internal/keytestcases"
)


func TestPrivateKey(t *testing.T) {
	for _, testCase := range keytestcases.Arr {
		privKey, err := NewPrivateKeyFromHex(testCase.PrivateKey)
		if err != nil {
			t.Fatal(err)
		}
		address, err := privKey.Address()
		if err != nil {
			t.Fatal(err)
		}
		if want, have := testCase.Address, address; want != have {
			t.Fatalf("expected %s got %s", want, have)
		}
		wif, err := privKey.WIF()
		if err != nil {
			t.Fatal(err)
		}
		if want, have := testCase.Wif, wif; want != have {
			t.Fatalf("expected %s got %s", want, have)
		}
		pubKey, _ := privKey.PublicKey()
		assert.Equal(t, hex.EncodeToString(pubKey.Bytes()), testCase.PublicKey)
	}
}

func TestPrivateKeyFromWIF(t *testing.T) {
	for _, testCase := range keytestcases.Arr {
		key, err := NewPrivateKeyFromWIF(testCase.Wif)
		if err != nil {
			t.Fatal(err)
		}
		if want, have := testCase.PrivateKey, key.String(); want != have {
			t.Fatalf("expected %s got %s", want, have)
		}
	}
}

func TestSigning(t *testing.T) {
	// These were taken from the rfcPage:https://tools.ietf.org/html/rfc6979#page-33
	//   public key: U = xG
	//Ux = 60FED4BA255A9D31C961EB74C6356D68C049B8923B61FA6CE669622E60F29FB6
	//Uy = 7903FE1008B8BC99A41AE9E95628BC64F2F1B20C2D7E9F5177A3C294D4462299
	PrivateKey, _ := NewPrivateKeyFromHex("C9AFA9D845BA75166B5C215767B1D6934E50C3DB36E89B127B8A622B120F6721")

	data, err := PrivateKey.Sign([]byte("sample"))
	if err != nil {
		t.Fatal(err)
	}

	r := "EFD48B2AACB6A8FD1140DD9CD45E81D69D2C877B56AAF991C34D0EA84EAF3716"
	s := "F7CB1C942D657C41D436C7A1B6E29F65F3E900DBB9AFF4064DC4AB2F843ACDA8"
	assert.Equal(t, strings.ToLower(r+s), hex.EncodeToString(data))
}
