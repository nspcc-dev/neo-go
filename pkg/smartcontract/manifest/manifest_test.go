package manifest

import (
	"encoding/json"
	"fmt"
	"math/big"
	"slices"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

// Test vectors are taken from the main NEO repo
// https://github.com/neo-project/neo/blob/master/tests/neo.UnitTests/SmartContract/Manifest/UT_ContractManifest.cs#L10
func TestManifest_MarshalJSON(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		s := `{"groups":[],"features":{},"supportedstandards":[],"name":"Test","abi":{"methods":[],"events":[]},"permissions":[{"contract":"*","methods":"*"}],"trusts":[],"extra":null}`
		m := testUnmarshalMarshalManifest(t, s)
		require.Equal(t, DefaultManifest("Test"), m)
	})

	t.Run("permissions", func(t *testing.T) {
		s := `{"groups":[],"features":{},"supportedstandards":[],"name":"Test","abi":{"methods":[],"events":[]},"permissions":[{"contract":"0x0000000000000000000000000000000000000000","methods":["method1","method2"]}],"trusts":[],"extra":null}`
		testUnmarshalMarshalManifest(t, s)
	})

	t.Run("safe methods", func(t *testing.T) {
		s := `{"groups":[],"features":{},"supportedstandards":[],"name":"Test","abi":{"methods":[{"name":"safeMet","offset":123,"parameters":[],"returntype":"Integer","safe":true}],"events":[]},"permissions":[{"contract":"*","methods":"*"}],"trusts":[],"extra":null}`
		testUnmarshalMarshalManifest(t, s)
	})

	t.Run("trust", func(t *testing.T) {
		s := `{"groups":[],"features":{},"supportedstandards":[],"name":"Test","abi":{"methods":[],"events":[]},"permissions":[{"contract":"*","methods":"*"}],"trusts":["0x0000000000000000000000000000000000000001"],"extra":null}`
		testUnmarshalMarshalManifest(t, s)
	})

	t.Run("groups", func(t *testing.T) {
		s := `{"groups":[{"pubkey":"03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c","signature":"QUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQQ=="}],"features":{},"supportedstandards":[],"name":"Test","abi":{"methods":[],"events":[]},"permissions":[{"contract":"*","methods":"*"}],"trusts":[],"extra":null}`
		testUnmarshalMarshalManifest(t, s)
	})

	t.Run("extra", func(t *testing.T) {
		s := `{"groups":[],"features":{},"supportedstandards":[],"name":"Test","abi":{"methods":[],"events":[]},"permissions":[{"contract":"*","methods":"*"}],"trusts":[],"extra":{"key":"value"}}`
		testUnmarshalMarshalManifest(t, s)
	})
}

func testUnmarshalMarshalManifest(t *testing.T, s string) *Manifest {
	js := []byte(s)
	c := NewManifest("Test")
	require.NoError(t, json.Unmarshal(js, c))

	data, err := json.Marshal(c)
	require.NoError(t, err)
	require.JSONEq(t, s, string(data))

	return c
}

func TestManifest_CanCall(t *testing.T) {
	man1 := DefaultManifest("Test1")
	man2 := DefaultManifest("Test2")
	require.True(t, man1.CanCall(util.Uint160{}, man2, "method1"))
}

func TestPermission_IsAllowed(t *testing.T) {
	manifest := DefaultManifest("Test")

	t.Run("wildcard", func(t *testing.T) {
		h := random.Uint160()

		perm := NewPermission(PermissionWildcard)
		require.True(t, perm.IsAllowed(h, manifest, "AAA"))

		perm.Methods.Restrict()
		require.False(t, perm.IsAllowed(h, manifest, "AAA"))

		perm.Methods.Add("AAA")
		require.True(t, perm.IsAllowed(h, manifest, "AAA"))
	})

	t.Run("hash", func(t *testing.T) {
		perm := NewPermission(PermissionHash, util.Uint160{})
		require.True(t, perm.IsAllowed(util.Uint160{}, manifest, "AAA"))

		t.Run("restrict methods", func(t *testing.T) {
			perm.Methods.Restrict()
			require.False(t, perm.IsAllowed(util.Uint160{}, manifest, "AAA"))
			perm.Methods.Add("AAA")
			require.True(t, perm.IsAllowed(util.Uint160{}, manifest, "AAA"))
		})
	})

	t.Run("invalid hash", func(t *testing.T) {
		perm := NewPermission(PermissionHash, util.Uint160{1})
		require.False(t, perm.IsAllowed(util.Uint160{}, manifest, "AAA"))
	})

	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	manifest.Groups = []Group{{PublicKey: priv.PublicKey()}}

	t.Run("group", func(t *testing.T) {
		perm := NewPermission(PermissionGroup, priv.PublicKey())
		require.True(t, perm.IsAllowed(util.Uint160{}, manifest, "AAA"))

		priv2, err := keys.NewPrivateKey()
		require.NoError(t, err)

		perm = NewPermission(PermissionGroup, priv2.PublicKey())
		require.False(t, perm.IsAllowed(util.Uint160{}, manifest, "AAA"))

		manifest.Groups = append(manifest.Groups, Group{PublicKey: priv2.PublicKey()})
		perm = NewPermission(PermissionGroup, priv2.PublicKey())
		require.True(t, perm.IsAllowed(util.Uint160{}, manifest, "AAA"))
	})
}

func TestIsValid(t *testing.T) {
	contractHash := util.Uint160{1, 2, 3}
	m := &Manifest{}

	t.Run("invalid, no name", func(t *testing.T) {
		require.Error(t, m.IsValid(contractHash, true))
	})

	m = NewManifest("Test")

	t.Run("invalid, no ABI methods", func(t *testing.T) {
		require.Error(t, m.IsValid(contractHash, true))
	})

	m.ABI.Methods = append(m.ABI.Methods, Method{
		Name:       "dummy",
		ReturnType: smartcontract.VoidType,
		Parameters: []Parameter{},
	})

	t.Run("valid, no groups/events", func(t *testing.T) {
		require.NoError(t, m.IsValid(contractHash, true))
	})

	t.Run("invalid, no features", func(t *testing.T) {
		m.Features = nil
		require.Error(t, m.IsValid(contractHash, true))
	})
	m.Features = json.RawMessage(emptyFeatures)

	t.Run("invalid, bad features", func(t *testing.T) {
		m.Features = json.RawMessage(`{ "v" : true}`)
		require.Error(t, m.IsValid(contractHash, true))
	})
	m.Features = json.RawMessage(emptyFeatures)

	t.Run("valid, features with spaces", func(t *testing.T) {
		m.Features = json.RawMessage("{ \t\n\r }")
		require.NoError(t, m.IsValid(contractHash, true))
	})
	m.Features = json.RawMessage(emptyFeatures)

	m.ABI.Events = append(m.ABI.Events, Event{
		Name:       "itHappened",
		Parameters: []Parameter{},
	})

	t.Run("valid, with events", func(t *testing.T) {
		require.NoError(t, m.IsValid(contractHash, true))
	})

	m.ABI.Events = append(m.ABI.Events, Event{
		Name: "itHappened",
		Parameters: []Parameter{
			NewParameter("qwerty", smartcontract.IntegerType),
			NewParameter("qwerty", smartcontract.IntegerType),
		},
	})

	t.Run("invalid, bad event", func(t *testing.T) {
		require.Error(t, m.IsValid(contractHash, true))
	})
	m.ABI.Events = m.ABI.Events[:1]

	m.Permissions = append(m.Permissions, *NewPermission(PermissionHash, util.Uint160{1, 2, 3}))
	t.Run("valid, with permissions", func(t *testing.T) {
		require.NoError(t, m.IsValid(contractHash, true))
	})

	m.Permissions = append(m.Permissions, *NewPermission(PermissionHash, util.Uint160{1, 2, 3}))
	t.Run("invalid, with permissions", func(t *testing.T) {
		require.Error(t, m.IsValid(contractHash, true))
	})
	m.Permissions = m.Permissions[:1]

	m.SupportedStandards = append(m.SupportedStandards, "NEP-17")
	t.Run("valid, with standards", func(t *testing.T) {
		require.NoError(t, m.IsValid(contractHash, true))
	})

	m.SupportedStandards = append(m.SupportedStandards, "")
	t.Run("invalid, with nameless standard", func(t *testing.T) {
		require.Error(t, m.IsValid(contractHash, true))
	})
	m.SupportedStandards = m.SupportedStandards[:1]

	m.SupportedStandards = append(m.SupportedStandards, "NEP-17")
	t.Run("invalid, with duplicate standards", func(t *testing.T) {
		require.Error(t, m.IsValid(contractHash, true))
	})
	m.SupportedStandards = m.SupportedStandards[:1]

	t.Run("invalid, no trusts", func(t *testing.T) {
		m.Trusts.Value = nil
		m.Trusts.Wildcard = false
		require.Error(t, m.IsValid(contractHash, true))
	})
	m.Trusts.Restrict()

	d := PermissionDesc{Type: PermissionHash, Value: util.Uint160{1, 2, 3}}
	m.Trusts.Add(d)
	t.Run("valid, with trust", func(t *testing.T) {
		require.NoError(t, m.IsValid(contractHash, true))
	})

	m.Trusts.Add(PermissionDesc{Type: PermissionHash, Value: util.Uint160{3, 2, 1}})
	t.Run("valid, with trusts", func(t *testing.T) {
		require.NoError(t, m.IsValid(contractHash, true))
	})

	m.Trusts.Add(d)
	t.Run("invalid, with trusts", func(t *testing.T) {
		require.Error(t, m.IsValid(contractHash, true))
	})
	m.Trusts.Restrict()

	t.Run("with groups", func(t *testing.T) {
		m.Groups = make([]Group, 3)
		pks := make([]*keys.PrivateKey, 3)
		for i := range pks {
			pk, err := keys.NewPrivateKey()
			require.NoError(t, err)
			pks[i] = pk
			m.Groups[i] = Group{
				PublicKey: pk.PublicKey(),
				Signature: pk.Sign(contractHash.BytesBE()),
			}
		}

		t.Run("valid", func(t *testing.T) {
			require.NoError(t, m.IsValid(contractHash, true))
		})

		t.Run("invalid, wrong contract hash", func(t *testing.T) {
			require.Error(t, m.IsValid(util.Uint160{4, 5, 6}, true))
		})

		t.Run("invalid, wrong group signature", func(t *testing.T) {
			pk, err := keys.NewPrivateKey()
			require.NoError(t, err)
			m.Groups = append(m.Groups, Group{
				PublicKey: pk.PublicKey(),
				// actually, there shouldn't be such situation, as Signature is always the signature
				// of the contract hash.
				Signature: pk.Sign([]byte{1, 2, 3}),
			})
			require.Error(t, m.IsValid(contractHash, true))
		})
	})
	m.Groups = m.Groups[:0]

	t.Run("invalid, unserializable", func(t *testing.T) {
		for i := range stackitem.MaxSerialized {
			m.ABI.Events = append(m.ABI.Events, Event{
				Name:       fmt.Sprintf("Event%d", i),
				Parameters: []Parameter{},
			})
		}
		err := m.IsValid(contractHash, true)
		require.ErrorIs(t, err, stackitem.ErrTooBig)
	})
}

func TestManifestToStackItem(t *testing.T) {
	check := func(t *testing.T, expected *Manifest) {
		item, err := expected.ToStackItem()
		require.NoError(t, err)
		actual := new(Manifest)
		require.NoError(t, actual.FromStackItem(item))
		require.Equal(t, expected, actual)
	}

	t.Run("default", func(t *testing.T) {
		expected := DefaultManifest("manifest")
		check(t, expected)
	})

	t.Run("full", func(t *testing.T) {
		pk, _ := keys.NewPrivateKey()
		expected := &Manifest{
			Name: "manifest",
			ABI: ABI{
				Methods: []Method{{
					Name:   "method",
					Offset: 15,
					Parameters: []Parameter{{
						Name: "param",
						Type: smartcontract.StringType,
					}},
					ReturnType: smartcontract.BoolType,
					Safe:       true,
				}},
				Events: []Event{{
					Name: "event",
					Parameters: []Parameter{{
						Name: "param",
						Type: smartcontract.BoolType,
					}},
				}},
			},
			Features: json.RawMessage("{}"),
			Groups: []Group{{
				PublicKey: pk.PublicKey(),
				Signature: make([]byte, keys.SignatureLen),
			}},
			Permissions:        []Permission{*NewPermission(PermissionWildcard)},
			SupportedStandards: []string{"NEP-17"},
			Trusts: WildPermissionDescs{
				Value: []PermissionDesc{
					{
						Type:  PermissionHash,
						Value: util.Uint160{1, 2, 3},
					},
					{
						Type:  PermissionGroup,
						Value: pk.PublicKey(),
					},
				},
			},
			Extra: []byte(`"even string allowed"`),
		}
		check(t, expected)
	})

	t.Run("compat", func(t *testing.T) {
		// Compatibility test with NeoC#, see https://github.com/neo-project/neo/pull/2948.
		var mJSON = `{"name":"CallOracleContract-6","groups":[],"features":{},"supportedstandards":[],"abi":{"methods":[{"name":"request","parameters":[{"name":"url","type":"String"},{"name":"filter","type":"String"},{"name":"gasForResponse","type":"Integer"}],"returntype":"Void","offset":0,"safe":false},{"name":"callback","parameters":[{"name":"url","type":"String"},{"name":"userData","type":"Any"},{"name":"responseCode","type":"Integer"},{"name":"response","type":"ByteArray"}],"returntype":"Void","offset":86,"safe":false},{"name":"getStoredUrl","parameters":[],"returntype":"String","offset":129,"safe":false},{"name":"getStoredResponseCode","parameters":[],"returntype":"Integer","offset":142,"safe":false},{"name":"getStoredResponse","parameters":[],"returntype":"ByteArray","offset":165,"safe":false}],"events":[]},"permissions":[{"contract":"0xfe924b7cfe89ddd271abaf7210a80a7e11178758","methods":"*"},{"contract":"*","methods":"*"}],"trusts":["0xfe924b7cfe89ddd271abaf7210a80a7e11178758","*"],"extra":{}}`
		c := NewManifest("Test")
		require.NoError(t, json.Unmarshal([]byte(mJSON), c))

		si, err := c.ToStackItem()
		require.NoError(t, err)
		actual := new(Manifest)
		require.NoError(t, actual.FromStackItem(si))
		require.NotEqual(t, actual.Permissions[0].Contract.Type, PermissionWildcard)
		require.True(t, actual.Permissions[0].Methods.IsWildcard())
		require.Equal(t, actual.Permissions[1].Contract.Type, PermissionWildcard)
		require.True(t, actual.Permissions[1].Methods.IsWildcard())

		require.NotEqual(t, actual.Trusts.Value[0].Type, PermissionWildcard)
		require.Equal(t, actual.Trusts.Value[1].Type, PermissionWildcard)
	})
}

func TestManifest_FromStackItemErrors(t *testing.T) {
	name := stackitem.NewByteArray([]byte{})
	groups := stackitem.NewArray([]stackitem.Item{})
	features := stackitem.NewMap()
	sStandards := stackitem.NewArray([]stackitem.Item{})
	abi := stackitem.NewStruct([]stackitem.Item{stackitem.NewArray([]stackitem.Item{}), stackitem.NewArray([]stackitem.Item{})})
	permissions := stackitem.NewArray([]stackitem.Item{})
	trusts := stackitem.NewArray([]stackitem.Item{})
	extra := stackitem.NewByteArray([]byte{})

	// check OK
	goodSI := []stackitem.Item{name, groups, features, sStandards, abi, permissions, trusts, extra}
	m := new(Manifest)
	require.NoError(t, m.FromStackItem(stackitem.NewStruct(goodSI)))

	errCases := map[string]stackitem.Item{
		"not a struct":                     stackitem.NewArray([]stackitem.Item{}),
		"invalid length":                   stackitem.NewStruct([]stackitem.Item{}),
		"invalid name type":                stackitem.NewStruct(slices.Concat([]stackitem.Item{stackitem.NewInterop(nil)}, goodSI[1:])),
		"invalid Groups type":              stackitem.NewStruct(slices.Concat(goodSI[:1], []stackitem.Item{stackitem.Null{}}, goodSI[2:])),
		"invalid group":                    stackitem.NewStruct(slices.Concat(goodSI[:1], []stackitem.Item{stackitem.NewArray([]stackitem.Item{stackitem.Null{}})}, goodSI[2:])),
		"invalid Features type":            stackitem.NewStruct(slices.Concat(goodSI[:2], []stackitem.Item{stackitem.Null{}}, goodSI[3:])),
		"invalid supported standards type": stackitem.NewStruct(slices.Concat(goodSI[:3], []stackitem.Item{stackitem.Null{}}, goodSI[4:])),
		"invalid supported standard":       stackitem.NewStruct(slices.Concat(goodSI[:3], []stackitem.Item{stackitem.NewArray([]stackitem.Item{stackitem.Null{}})}, goodSI[4:])),
		"invalid ABI":                      stackitem.NewStruct(slices.Concat(goodSI[:4], []stackitem.Item{stackitem.Null{}}, goodSI[5:])),
		"invalid Permissions type":         stackitem.NewStruct(slices.Concat(goodSI[:5], []stackitem.Item{stackitem.Null{}}, goodSI[6:])),
		"invalid permission":               stackitem.NewStruct(slices.Concat(goodSI[:5], []stackitem.Item{stackitem.NewArray([]stackitem.Item{stackitem.Null{}})}, goodSI[6:])),
		"invalid Trusts type":              stackitem.NewStruct(slices.Concat(goodSI[:6], []stackitem.Item{stackitem.NewInterop(nil)}, goodSI[7:])),
		"invalid trust":                    stackitem.NewStruct(slices.Concat(goodSI[:6], []stackitem.Item{stackitem.NewArray([]stackitem.Item{stackitem.NewInterop(nil)})}, goodSI[7:])),
		"invalid Uint160 trust":            stackitem.NewStruct(slices.Concat(goodSI[:6], []stackitem.Item{stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray([]byte{1, 2, 3})})}, goodSI[7:])),
		"invalid extra type":               stackitem.NewStruct(slices.Concat(goodSI[:7], []stackitem.Item{stackitem.Null{}})),
	}
	for name, errCase := range errCases {
		t.Run(name, func(t *testing.T) {
			p := new(Manifest)
			require.Error(t, p.FromStackItem(errCase))
		})
	}
}

func TestABI_ToStackItemFromStackItem(t *testing.T) {
	a := &ABI{
		Methods: []Method{{
			Name:       "mur",
			Offset:     5,
			Parameters: []Parameter{{Name: "p1", Type: smartcontract.BoolType}},
			ReturnType: smartcontract.StringType,
			Safe:       true,
		}},
		Events: []Event{{
			Name:       "mur",
			Parameters: []Parameter{{Name: "p1", Type: smartcontract.BoolType}},
		}},
	}
	expected := stackitem.NewStruct([]stackitem.Item{
		stackitem.NewArray([]stackitem.Item{
			stackitem.NewStruct([]stackitem.Item{
				stackitem.NewByteArray([]byte("mur")),
				stackitem.NewArray([]stackitem.Item{
					stackitem.NewStruct([]stackitem.Item{
						stackitem.NewByteArray([]byte("p1")),
						stackitem.NewBigInteger(big.NewInt(int64(smartcontract.BoolType))),
					}),
				}),
				stackitem.NewBigInteger(big.NewInt(int64(smartcontract.StringType))),
				stackitem.NewBigInteger(big.NewInt(int64(5))),
				stackitem.NewBool(true),
			}),
		}),
		stackitem.NewArray([]stackitem.Item{
			stackitem.NewStruct([]stackitem.Item{
				stackitem.NewByteArray([]byte("mur")),
				stackitem.NewArray([]stackitem.Item{
					stackitem.NewStruct([]stackitem.Item{
						stackitem.NewByteArray([]byte("p1")),
						stackitem.NewBigInteger(big.NewInt(int64(smartcontract.BoolType))),
					}),
				}),
			}),
		}),
	})
	CheckToFromStackItem(t, a, expected)
}

func TestABI_FromStackItemErrors(t *testing.T) {
	errCases := map[string]stackitem.Item{
		"not a struct":         stackitem.NewArray([]stackitem.Item{}),
		"invalid length":       stackitem.NewStruct([]stackitem.Item{}),
		"invalid methods type": stackitem.NewStruct([]stackitem.Item{stackitem.NewInterop(nil), stackitem.Null{}}),
		"invalid method":       stackitem.NewStruct([]stackitem.Item{stackitem.NewArray([]stackitem.Item{stackitem.Null{}}), stackitem.Null{}}),
		"invalid events type":  stackitem.NewStruct([]stackitem.Item{stackitem.NewArray([]stackitem.Item{}), stackitem.Null{}}),
		"invalid event":        stackitem.NewStruct([]stackitem.Item{stackitem.NewArray([]stackitem.Item{}), stackitem.NewArray([]stackitem.Item{stackitem.Null{}})}),
	}
	for name, errCase := range errCases {
		t.Run(name, func(t *testing.T) {
			p := new(ABI)
			require.Error(t, p.FromStackItem(errCase))
		})
	}
}

func TestExtraToStackItem(t *testing.T) {
	testCases := []struct {
		raw, expected string
	}{
		{"null", "null"},
		{"1", "1"},
		{"1.23456789101112131415", "1.23456789101112131415"},
		{`"string with space"`, `"string with space"`},
		{`{ "a":1, "sss" : {"m" : 1, "a" : 2} , "x"  :  2  ,"c" :3,"z":4,  "s":"5"}`,
			`{"a":1,"sss":{"m":1,"a":2},"x":2,"c":3,"z":4,"s":"5"}`},
		{`  [ 1, "array", { "d": "z", "a":"x",  "c" : "y",  "b":3}]`,
			`[1,"array",{"d":"z","a":"x","c":"y","b":3}]`},
		{
			// C# double quotes marshalling compatibility test, ref. #3284.
			`{"Author":"NEOZEN","Description":"NEO\u0027s First Inscriptions Meta Protocol","Deployment":"{\"p\":\"neoz-20\",\"op\":\"deploy\",\"tick\":\"neoz\",\"max\":\"21000000\",\"lim\":\"1000\"}"}`,
			`{"Author":"NEOZEN","Description":"NEO\u0027s First Inscriptions Meta Protocol","Deployment":"{\u0022p\u0022:\u0022neoz-20\u0022,\u0022op\u0022:\u0022deploy\u0022,\u0022tick\u0022:\u0022neoz\u0022,\u0022max\u0022:\u002221000000\u0022,\u0022lim\u0022:\u00221000\u0022}"}`,
		},
	}

	for _, tc := range testCases {
		res := extraToStackItem([]byte(tc.raw))
		actual, ok := res.Value().([]byte)
		require.True(t, ok)
		require.Equal(t, tc.expected, string(actual))
	}
}

func TestManifest_IsStandardSupported(t *testing.T) {
	m := &Manifest{
		SupportedStandards: []string{NEP17StandardName, NEP27StandardName, NEP26StandardName},
	}
	for _, st := range m.SupportedStandards {
		require.True(t, m.IsStandardSupported(st))
	}
	require.False(t, m.IsStandardSupported(NEP11StandardName))
	require.False(t, m.IsStandardSupported(""))
	require.False(t, m.IsStandardSupported("unknown standard"))
}
