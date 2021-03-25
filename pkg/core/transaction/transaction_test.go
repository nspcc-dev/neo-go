package transaction

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWitnessEncodeDecode(t *testing.T) {
	verif, err := hex.DecodeString("552102486fd15702c4490a26703112a5cc1d0923fd697a33406bd5a1c00e0013b09a7021024c7b7fb6c310fccf1ba33b082519d82964ea93868d676662d4a59ad548df0e7d2102aaec38470f6aad0042c6e877cfd8087d2676b0f516fddd362801b9bd3936399e2103b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c2103b8d9d5771d8f513aa0869b9cc8d50986403b78c6da36890638c3d46a5adce04a2102ca0e27697b9c248f6f16e085fd0061e26f44da85b58ee835c110caa5ec3ba5542102df48f60e8f3e01c48ff40b9b7f1310d7a8b2a193188befe1c2e3df740e89509357ae")
	assert.Nil(t, err)
	invoc, err := hex.DecodeString("404edf5005771de04619235d5a4c7a9a11bb78e008541f1da7725f654c33380a3c87e2959a025da706d7255cb3a3fa07ebe9c6559d0d9e6213c68049168eb1056f4038a338f879930c8adc168983f60aae6f8542365d844f004976346b70fb0dd31aa1dbd4abd81e4a4aeef9941ecd4e2dd2c1a5b05e1cc74454d0403edaee6d7a4d4099d33c0b889bf6f3e6d87ab1b11140282e9a3265b0b9b918d6020b2c62d5a040c7e0c2c7c1dae3af9b19b178c71552ebd0b596e401c175067c70ea75717c8c00404e0ebd369e81093866fe29406dbf6b402c003774541799d08bf9bb0fc6070ec0f6bad908ab95f05fa64e682b485800b3c12102a8596e6c715ec76f4564d5eff34070e0521979fcd2cbbfa1456d97cc18d9b4a6ad87a97a2a0bcdedbf71b6c9676c645886056821b6f3fec8694894c66f41b762bc4e29e46ad15aee47f05d27d822")
	assert.Nil(t, err)

	lenInvoc := len(invoc)
	lenVerif := len(verif)
	t.Log(lenInvoc)
	t.Log(lenVerif)

	wit := &Witness{
		InvocationScript:   invoc,
		VerificationScript: verif,
	}

	witDecode := &Witness{}
	testserdes.EncodeDecodeBinary(t, wit, witDecode)

	t.Log(len(witDecode.VerificationScript))
	t.Log(len(witDecode.InvocationScript))
}

func TestDecodeEncodeInvocationTX(t *testing.T) {
	tx := decodeTransaction(rawInvocationTX, t)

	script := "CwMAQNndiE0KAAwUgM7HtvW1b1BXj3N/Fi06sU1GZQ0MFN7uecGJ8wCYsLpqLrkLOpJYpsf/FMAfDAh0cmFuc2ZlcgwUz3bii9AGLEpHjuNVYQETGfPPpNJBYn1bUjk="
	assert.Equal(t, script, base64.StdEncoding.EncodeToString(tx.Script))
	assert.Equal(t, uint32(431760600), tx.Nonce)
	assert.Equal(t, int64(11000000), tx.SystemFee)
	assert.Equal(t, int64(4500000), tx.NetworkFee)
	assert.Equal(t, uint32(1000), tx.ValidUntilBlock)
	assert.Equal(t, "25426643feed564cd3e57f346d6c68692f5622b3063da11c5572d99ee1a5b49a", tx.Hash().StringLE())

	assert.Equal(t, 1, len(tx.Signers))
	assert.Equal(t, CalledByEntry, tx.Signers[0].Scopes)
	assert.Equal(t, "ffc7a658923a0bb92e6abab09800f389c179eede", tx.Signers[0].Account.StringLE())

	assert.Equal(t, 0, len(tx.Attributes))
	invoc1 := "DEDWn0D7z2ELqpN8ghcM/PtfFwo56/BfEasfHuSKECJMYxvU47r2ZtSihg59lGxSZzHsvxTy6nsyvJ22ycNhINdJDECl61cg937N/HujKsLMu2wJMS7C54bzJ3q22Czqllvw3Yp809USgKDs+W+3QD7rI+SFs0OhIn0gooCUU6f/13WjDEDr9XdeT5CGTO8CL0JigzcTcucs0GBcqHs8fToO6zPuuCfS7Wh6dyxSCijT4A4S+7BUdW3dsO7828ke1fj8oNxm"
	verif1 := "EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFEF7zmyl"
	assert.Equal(t, 1, len(tx.Scripts))
	assert.Equal(t, invoc1, base64.StdEncoding.EncodeToString(tx.Scripts[0].InvocationScript))
	assert.Equal(t, verif1, base64.StdEncoding.EncodeToString(tx.Scripts[0].VerificationScript))

	data, err := testserdes.EncodeBinary(tx)
	assert.NoError(t, err)
	assert.Equal(t, rawInvocationTX, base64.StdEncoding.EncodeToString(data))
}

func TestNew(t *testing.T) {
	script := []byte{0x51}
	tx := New(script, 1)
	tx.Signers = []Signer{{Account: util.Uint160{1, 2, 3}}}
	tx.Scripts = []Witness{{InvocationScript: []byte{}, VerificationScript: []byte{}}}
	assert.Equal(t, int64(1), tx.SystemFee)
	assert.Equal(t, script, tx.Script)
	// Update hash fields to match tx2 that is gonna autoupdate them on decode.
	_ = tx.Hash()
	_ = tx.Size()
	testserdes.EncodeDecodeBinary(t, tx, &Transaction{})
}

func TestNewTransactionFromBytes(t *testing.T) {
	script := []byte{0x51}
	tx := New(script, 1)
	tx.NetworkFee = 123
	tx.Signers = []Signer{{Account: util.Uint160{1, 2, 3}}}
	tx.Scripts = []Witness{{InvocationScript: []byte{}, VerificationScript: []byte{}}}
	data, err := testserdes.EncodeBinary(tx)
	require.NoError(t, err)

	// set cached fields
	tx.Hash()
	tx.FeePerByte()

	tx1, err := NewTransactionFromBytes(data)
	require.NoError(t, err)
	require.Equal(t, tx, tx1)

	data = append(data, 42)
	_, err = NewTransactionFromBytes(data)
	require.Error(t, err)
}

func TestEncodingTXWithNoScript(t *testing.T) {
	_, err := testserdes.EncodeBinary(new(Transaction))
	require.Error(t, err)
}

func TestDecodingTXWithNoScript(t *testing.T) {
	txBin, err := hex.DecodeString("00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")
	require.NoError(t, err)
	err = testserdes.DecodeBinary(txBin, new(Transaction))
	require.Error(t, err)
}

func TestDecodingTxWithInvalidWitnessesNumber(t *testing.T) {
	tx := New([]byte{byte(opcode.RET)}, 1)
	tx.Signers = []Signer{{Account: util.Uint160{1, 2, 3}}}
	tx.Scripts = []Witness{{InvocationScript: []byte{}, VerificationScript: []byte{}}, {InvocationScript: []byte{}, VerificationScript: []byte{}}}
	data, err := testserdes.EncodeBinary(tx)
	require.NoError(t, err)
	require.True(t, errors.Is(testserdes.DecodeBinary(data, new(Transaction)), ErrInvalidWitnessNum))
}

func TestUnmarshalNeoFSTX(t *testing.T) {
	txjson := []byte(`
{
  "hash": "0xc90c77d6c17fbb959b16df820370204a534ca13826cbd9fc027beb1b55d31e5a",
  "size": 232,
  "version": 0,
  "nonce": 737880259,
  "sender": "NiRqSd5MtRZT5yUhgWd7oG11brkDG76Jim",
  "sysfee": "223719420",
  "netfee": "1215550",
  "validuntilblock": 1931,
  "attributes": [],
  "signers": [
    {
      "account": "0x8f0ecd714c31c5624b6647e5fd661e5031c8f8f6",
      "scopes": "Global"
    }
  ],
  "script": "DCECs2Ir9AF73+MXxYrtX0x1PyBrfbiWBG+n13S7xL9/jcIRwBESwAwEdm90ZQwUo4Hyc4fSGC6JbZtdrGb9LBbtWJtBYn1bUg==",
  "witnesses": [
    {
      "invocation": "DEDr2gA/8T/wxQvgOZVfCdkbj6uGrprkDgJvpOJCcbl+tvlKZkZytCZEWm6NoZhJyIlEI3VQSLtU3AHuJfShAT5L",
      "verification": "DCEDAS1H52IQrsc745qz0YbgpA/o2Gv6PU+r/aV7oTuI+WoLQZVEDXg="
    }
  ]
}`)
	tx := new(Transaction)
	require.NoError(t, json.Unmarshal(txjson, tx))
}

func TestMarshalUnmarshalJSONInvocationTX(t *testing.T) {
	tx := &Transaction{
		Version:    0,
		Signers:    []Signer{{Account: util.Uint160{1, 2, 3}}},
		Script:     []byte{1, 2, 3, 4},
		Attributes: []Attribute{{Type: HighPriority}},
		Scripts:    []Witness{},
		SystemFee:  int64(fixedn.Fixed8FromFloat(123.45)),
		NetworkFee: int64(fixedn.Fixed8FromFloat(0.123)),
		Trimmed:    false,
	}

	testserdes.MarshalUnmarshalJSON(t, tx, new(Transaction))
}

func TestTransaction_HasAttribute(t *testing.T) {
	tx := New([]byte{1}, 0)
	require.False(t, tx.HasAttribute(HighPriority))
	tx.Attributes = append(tx.Attributes, Attribute{Type: HighPriority})
	require.True(t, tx.HasAttribute(HighPriority))
	tx.Attributes = append(tx.Attributes, Attribute{Type: HighPriority})
	require.True(t, tx.HasAttribute(HighPriority))
}

func TestTransaction_isValid(t *testing.T) {
	newTx := func() *Transaction {
		return &Transaction{
			Version:    0,
			SystemFee:  100,
			NetworkFee: 100,
			Signers: []Signer{
				{Account: util.Uint160{1, 2, 3}},
				{
					Account: util.Uint160{4, 5, 6},
					Scopes:  Global,
				},
			},
			Script:     []byte{1, 2, 3, 4},
			Attributes: []Attribute{},
			Scripts:    []Witness{},
			Trimmed:    false,
		}
	}

	t.Run("Valid", func(t *testing.T) {
		t.Run("NoAttributes", func(t *testing.T) {
			tx := newTx()
			require.NoError(t, tx.isValid())
		})
		t.Run("HighPriority", func(t *testing.T) {
			tx := newTx()
			tx.Attributes = []Attribute{{Type: HighPriority}}
			require.NoError(t, tx.isValid())
		})
	})
	t.Run("InvalidVersion", func(t *testing.T) {
		tx := newTx()
		tx.Version = 1
		require.True(t, errors.Is(tx.isValid(), ErrInvalidVersion))
	})
	t.Run("NegativeSystemFee", func(t *testing.T) {
		tx := newTx()
		tx.SystemFee = -1
		require.True(t, errors.Is(tx.isValid(), ErrNegativeSystemFee))
	})
	t.Run("NegativeNetworkFee", func(t *testing.T) {
		tx := newTx()
		tx.NetworkFee = -1
		require.True(t, errors.Is(tx.isValid(), ErrNegativeNetworkFee))
	})
	t.Run("TooBigFees", func(t *testing.T) {
		tx := newTx()
		tx.SystemFee = math.MaxInt64 - tx.NetworkFee + 1
		require.True(t, errors.Is(tx.isValid(), ErrTooBigFees))
	})
	t.Run("EmptySigners", func(t *testing.T) {
		tx := newTx()
		tx.Signers = tx.Signers[:0]
		require.True(t, errors.Is(tx.isValid(), ErrEmptySigners))
	})
	t.Run("NonUniqueSigners", func(t *testing.T) {
		tx := newTx()
		tx.Signers[1].Account = tx.Signers[0].Account
		require.True(t, errors.Is(tx.isValid(), ErrNonUniqueSigners))
	})
	t.Run("MultipleHighPriority", func(t *testing.T) {
		tx := newTx()
		tx.Attributes = []Attribute{
			{Type: HighPriority},
			{Type: HighPriority},
		}
		require.True(t, errors.Is(tx.isValid(), ErrInvalidAttribute))
	})
	t.Run("MultipleOracle", func(t *testing.T) {
		tx := newTx()
		tx.Attributes = []Attribute{
			{Type: OracleResponseT},
			{Type: OracleResponseT},
		}
		require.True(t, errors.Is(tx.isValid(), ErrInvalidAttribute))
	})
	t.Run("NoScript", func(t *testing.T) {
		tx := newTx()
		tx.Script = []byte{}
		require.True(t, errors.Is(tx.isValid(), ErrEmptyScript))
	})
}

func TestTransaction_GetAttributes(t *testing.T) {
	attributesTypes := []AttrType{
		HighPriority,
		OracleResponseT,
		NotValidBeforeT,
	}
	t.Run("no attributes", func(t *testing.T) {
		tx := new(Transaction)
		for _, typ := range attributesTypes {
			require.Nil(t, tx.GetAttributes(typ))
		}
	})
	t.Run("single attributes", func(t *testing.T) {
		attrs := make([]Attribute, len(attributesTypes))
		for i, typ := range attributesTypes {
			attrs[i] = Attribute{Type: typ}
		}
		tx := &Transaction{Attributes: attrs}
		for _, typ := range attributesTypes {
			require.Equal(t, []Attribute{{Type: typ}}, tx.GetAttributes(typ))
		}
	})
	t.Run("multiple attributes", func(t *testing.T) {
		typ := AttrType(ReservedLowerBound + 1)
		conflictsAttrs := []Attribute{{Type: typ}, {Type: typ}}
		tx := Transaction{
			Attributes: append([]Attribute{{Type: HighPriority}}, conflictsAttrs...),
		}
		require.Equal(t, conflictsAttrs, tx.GetAttributes(typ))
	})
}

func TestTransaction_HasSigner(t *testing.T) {
	u1, u2 := random.Uint160(), random.Uint160()
	tx := Transaction{
		Signers: []Signer{
			{Account: u1}, {Account: u2},
		},
	}
	require.True(t, tx.HasSigner(u1))
	require.False(t, tx.HasSigner(util.Uint160{}))
}
