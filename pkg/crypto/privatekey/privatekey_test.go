package privatekey

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrivateKeyToPublicKey(t *testing.T) {
	input := "495d528227c7dcc234c690af1222e67cde916dac1652cad97e0263825a8268a6"

	privateKey, err := NewPrivateKeyFromHex(input)
	if err != nil {
		t.Fatal(err)
	}
	pubKey, _ := privateKey.PublicKey()
	pubKeyBytes := pubKey.Bytes()
	actual := hex.EncodeToString(pubKeyBytes)
	expected := "03cd4c4ee9c8e1fae9d12ecf7c96cb3a057b550393f9e82182c4dae1139871682e"
	assert.Equal(t, expected, actual)
}
func TestWIFEncode(t *testing.T) {
	input := "29bbf53185a973d2e3803cb92908fd08117486d1f2e7bab73ed0d00255511637"
	inputBytes, _ := hex.DecodeString(input)

	actual := WIFEncode(inputBytes)
	expected := "KxcqV28rGDcpVR3fYg7R9vricLpyZ8oZhopyFLAWuRv7Y8TE9WhW"
	assert.Equal(t, expected, actual)
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
