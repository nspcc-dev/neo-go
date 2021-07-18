package wallet

import (
	"crypto/elliptic"
	"encoding/hex"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
	"github.com/stretchr/testify/require"
)

func testParseMultisigContract(t *testing.T, s []byte, nsigs int, keys ...*keys.PublicKey) {
	ns, ks, ok := parseMultisigContract(s)
	if len(keys) == 0 {
		require.False(t, ok)
		return
	}
	require.True(t, ok)
	require.Equal(t, nsigs, ns)
	require.Equal(t, len(keys), len(ks))
	for i := range keys {
		require.Equal(t, keys[i], ks[i])
	}
}

func TestParseMultisigContract(t *testing.T) {
	t.Run("single multisig", func(t *testing.T) {
		s := fromHex(t, "512102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc251ae")
		pub := pubFromHex(t, "02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2")
		t.Run("good, no ret", func(t *testing.T) {
			testParseMultisigContract(t, s, 1, pub)
		})
		t.Run("good, with ret", func(t *testing.T) {
			s := append(s, opRet)
			testParseMultisigContract(t, s, 1, pub)
		})
		t.Run("bad, no check multisig", func(t *testing.T) {
			sBad := slice.Copy(s)
			sBad[len(sBad)-1] ^= 0xFF
			testParseMultisigContract(t, sBad, 0)
		})
		t.Run("bad, invalid number of keys", func(t *testing.T) {
			sBad := slice.Copy(s)
			sBad[len(sBad)-2] = opPush1 + 1
			testParseMultisigContract(t, sBad, 0)
		})
		t.Run("bad, invalid first instruction", func(t *testing.T) {
			sBad := slice.Copy(s)
			sBad[0] = 0xFF
			testParseMultisigContract(t, sBad, 0)
		})
		t.Run("bad, invalid public key", func(t *testing.T) {
			sBad := slice.Copy(s)
			sBad[2] = 0xFF
			testParseMultisigContract(t, sBad, 0)
		})
		t.Run("bad, many sigs", func(t *testing.T) {
			sBad := slice.Copy(s)
			sBad[0] = opPush1 + 1
			testParseMultisigContract(t, sBad, 0)
		})
		t.Run("empty, no panic", func(t *testing.T) {
			testParseMultisigContract(t, []byte{}, 0)
		})
	})
	t.Run("3/4 multisig", func(t *testing.T) {
		// From privnet consensus wallet.
		s := fromHex(t, "532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae")
		ks := keys.PublicKeys{
			pubFromHex(t, "02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e"),
			pubFromHex(t, "02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62"),
			pubFromHex(t, "02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2"),
			pubFromHex(t, "03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699"),
		}
		t.Run("good", func(t *testing.T) {
			testParseMultisigContract(t, s, 3, ks...)
		})
		t.Run("good, with pushbytes1", func(t *testing.T) {
			s := append([]byte{opPushBytes1, 3}, s[1:]...)
			testParseMultisigContract(t, s, 3, ks...)
		})
		t.Run("good, with pushbytes2", func(t *testing.T) {
			s := append([]byte{opPushBytes2, 3, 0}, s[1:]...)
			testParseMultisigContract(t, s, 3, ks...)
		})
		t.Run("bad, no panic on prefix", func(t *testing.T) {
			for i := minMultisigLen; i < len(s)-1; i++ {
				testParseMultisigContract(t, s[:i], 0)
			}
		})
	})
}

func fromHex(t *testing.T, s string) []byte {
	bs, err := hex.DecodeString(s)
	require.NoError(t, err)
	return bs
}

func pubFromHex(t *testing.T, s string) *keys.PublicKey {
	bs := fromHex(t, s)
	pub, err := keys.NewPublicKeyFromBytes(bs, elliptic.P256())
	require.NoError(t, err)
	return pub
}
