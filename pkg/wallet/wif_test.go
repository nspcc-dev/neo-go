package wallet

import (
	"encoding/hex"
	"testing"
)

type wifTestCase struct {
	wif        string
	compressed bool
	privateKey string
	version    byte
}

var wifTestCases = []wifTestCase{
	{
		wif:        "KwDiBf89QgGbjEhKnhXJuH7LrciVrZi3qYjgd9M7rFU73sVHnoWn",
		compressed: true,
		privateKey: "0000000000000000000000000000000000000000000000000000000000000001",
		version:    0x80,
	},
	{
		wif:        "5HpHagT65TZzG1PH3CSu63k8DbpvD8s5ip4nEB3kEsreAnchuDf",
		compressed: false,
		privateKey: "0000000000000000000000000000000000000000000000000000000000000001",
		version:    0x80,
	},
	{
		wif:        "KxhEDBQyyEFymvfJD96q8stMbJMbZUb6D1PmXqBWZDU2WvbvVs9o",
		compressed: true,
		privateKey: "2bfe58ab6d9fd575bdc3a624e4825dd2b375d64ac033fbc46ea79dbab4f69a3e",
		version:    0x80,
	},
}

func TestWIFEncodeDecode(t *testing.T) {
	for _, testCase := range wifTestCases {
		b, err := hex.DecodeString(testCase.privateKey)
		if err != nil {
			t.Fatal(err)
		}
		wif, err := WIFEncode(b, testCase.version, testCase.compressed)
		if err != nil {
			t.Fatal(err)
		}
		if want, have := testCase.wif, wif; want != have {
			t.Fatalf("expected %s got %s", want, have)
		}

		WIF, err := WIFDecode(wif, testCase.version)
		if err != nil {
			t.Fatal(err)
		}
		if want, have := testCase.privateKey, WIF.PrivateKey.String(); want != have {
			t.Fatalf("expected %s got %s", want, have)
		}
		if want, have := testCase.compressed, WIF.Compressed; want != have {
			t.Fatalf("expected %v got %v", want, have)
		}
		if want, have := testCase.version, WIF.Version; want != have {
			t.Fatalf("expected %d got %d", want, have)
		}
	}
}
