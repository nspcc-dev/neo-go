package wallet

import (
	"bytes"
	"crypto/rand"
	"io"
	"math/big"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/stretchr/testify/assert"
)

type testKey struct {
	address,
	privateKey,
	publicKey,
	wif,
	passphrase,
	encryptedWif string
}

var testKeyCases = []testKey{
	{
		address:      "ALq7AWrhAueN6mJNqk6FHJjnsEoPRytLdW",
		privateKey:   "7d128a6d096f0c14c3a25a2b0c41cf79661bfcb4a8cc95aaaea28bde4d732344",
		publicKey:    "02028a99826edc0c97d18e22b6932373d908d323aa7f92656a77ec26e8861699ef",
		wif:          "L1QqQJnpBwbsPGAuutuzPTac8piqvbR1HRjrY5qHup48TBCBFe4g",
		passphrase:   "city of zion",
		encryptedWif: "6PYLHmDf6AjF4AsVtosmxHuPYeuyJL3SLuw7J1U8i7HxKAnYNsp61HYRfF",
	},
	{
		address:      "ALfnhLg7rUyL6Jr98bzzoxz5J7m64fbR4s",
		privateKey:   "9ab7e154840daca3a2efadaf0df93cd3a5b51768c632f5433f86909d9b994a69",
		publicKey:    "031d8e1630ce640966967bc6d95223d21f44304133003140c3b52004dc981349c9",
		wif:          "L2QTooFoDFyRFTxmtiVHt5CfsXfVnexdbENGDkkrrgTTryiLsPMG",
		passphrase:   "我的密码",
		encryptedWif: "6PYWVp3xfgvnuNKP7ZavSViYvvim2zuzx9Q33vuWZr8aURiKeJ6Zm7BfPQ",
	},
	{
		address:      "AVf4UGKevVrMR1j3UkPsuoYKSC4ocoAkKx",
		privateKey:   "3edee7036b8fd9cef91de47386b191dd76db2888a553e7736bb02808932a915b",
		publicKey:    "02232ce8d2e2063dce0451131851d47421bfc4fc1da4db116fca5302c0756462fa",
		wif:          "KyKvWLZsNwBJx5j9nurHYRwhYfdQUu9tTEDsLCUHDbYBL8cHxMiG",
		passphrase:   "MyL33tP@33w0rd",
		encryptedWif: "6PYNoc1EG5J38MTqGN9Anphfdd6UwbS4cpFCzHhrkSKBBbV1qkbJJZQnkn",
	},
}

func oldPrivateKey(buf *bytes.Buffer) *PrivateKey {
	reader := io.TeeReader(rand.Reader, buf)
	c := crypto.NewEllipticCurve()
	b := make([]byte, c.Params().N.BitLen()/8+8)
	if _, err := io.ReadFull(reader, b); err != nil {
		panic(err)
	}

	d := new(big.Int).SetBytes(b)
	d.Mod(d, new(big.Int).Sub(c.Params().N, big.NewInt(1)))
	d.Add(d, big.NewInt(1))

	p := &PrivateKey{b: d.Bytes()}
	return p
}

func TestOldNewPrivateKey(t *testing.T) {
	for i := 0; i < 100; i++ {
		buf := new(bytes.Buffer)

		sk1 := oldPrivateKey(buf)
		sk2, _ := newPrivateKey(buf)
		assert.Equal(t, sk1.ecdsa(), sk2.ecdsa())
	}
}

func TestPrivateKey(t *testing.T) {
	for _, testCase := range testKeyCases {
		privKey, err := NewPrivateKeyFromHex(testCase.privateKey)
		if err != nil {
			t.Fatal(err)
		}
		address, err := privKey.Address()
		if err != nil {
			t.Fatal(err)
		}
		if want, have := testCase.address, address; want != have {
			t.Fatalf("expected %s got %s", want, have)
		}
		wif, err := privKey.WIF()
		if err != nil {
			t.Fatal(err)
		}
		if want, have := testCase.wif, wif; want != have {
			t.Fatalf("expected %s got %s", want, have)
		}
	}
}

func TestPrivateKeyFromWIF(t *testing.T) {
	for _, testCase := range testKeyCases {
		key, err := NewPrivateKeyFromWIF(testCase.wif)
		if err != nil {
			t.Fatal(err)
		}
		if want, have := testCase.privateKey, key.String(); want != have {
			t.Fatalf("expected %s got %s", want, have)
		}
	}
}
