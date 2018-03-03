package transaction

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Source of this TX: https://neotracker.io/tx/2c6a45547b3898318e400e541628990a07acb00f3b9a15a8e966ae49525304da
var rawTXClaim = "020004bc67ba325d6412ff4c55b10f7e9afb54bbb2228d201b37363c3d697ac7c198f70300591cd454d7318d2087c0196abfbbd1573230380672f0f0cd004dcb4857e58cbd010031bcfbed573f5318437e95edd603922a4455ff3326a979fdd1c149a84c4cb0290000b51eb6159c58cac4fe23d90e292ad2bcb7002b0da2c474e81e1889c0649d2c490000000001e72d286979ee6cb1b7e65dfddfb2e384100b8d148e7758de42e4168b71792c603b555f00000000005d9de59d99c0d1f6ed1496444473f4a0b538302f014140456349cec43053009accdb7781b0799c6b591c812768804ab0a0b56b5eae7a97694227fcd33e70899c075848b2cee8fae733faac6865b484d3f7df8949e2aadb232103945fae1ed3c31d778f149192b76734fcc951b400ba3598faa81ff92ebe477eacac"

func TestDecodeEncodeClaimTransaction(t *testing.T) {
	b, err := hex.DecodeString(rawTXClaim)
	if err != nil {
		t.Fatal(err)
	}
	tx := &Transaction{}
	if err := tx.DecodeBinary(bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, tx.Type, ClaimTX)
	assert.IsType(t, tx.Data, &ClaimTransaction{})
	claimTX := tx.Data.(*ClaimTransaction)
	assert.Equal(t, 4, len(claimTX.Claims))
	assert.Equal(t, 0, len(tx.Attributes))
	assert.Equal(t, 0, len(tx.Inputs))
	assert.Equal(t, 1, len(tx.Outputs))
	assert.Equal(t, "AQJseD8iBmCD4sgfHRhMahmoi9zvopG6yz", tx.Outputs[0].ScriptHash.NEOAddress())
	assert.Equal(t, "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7", tx.Outputs[0].AssetID.String())
	assert.Equal(t, tx.Outputs[0].Amount.String(), "0.06247739")
	invoc := "40456349cec43053009accdb7781b0799c6b591c812768804ab0a0b56b5eae7a97694227fcd33e70899c075848b2cee8fae733faac6865b484d3f7df8949e2aadb"
	verif := "2103945fae1ed3c31d778f149192b76734fcc951b400ba3598faa81ff92ebe477eacac"
	assert.Equal(t, 1, len(tx.Scripts))
	assert.Equal(t, invoc, hex.EncodeToString(tx.Scripts[0].InvocationScript))
	assert.Equal(t, verif, hex.EncodeToString(tx.Scripts[0].VerificationScript))

	buf := new(bytes.Buffer)
	if err := tx.EncodeBinary(buf); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, rawTXClaim, hex.EncodeToString(buf.Bytes()))
}
