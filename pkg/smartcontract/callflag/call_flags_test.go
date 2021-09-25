package callflag

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestCallFlag_Has(t *testing.T) {
	require.True(t, AllowCall.Has(AllowCall))
	require.True(t, (AllowCall | AllowNotify).Has(AllowCall))
	require.False(t, (AllowCall).Has(AllowCall|AllowNotify))
	require.True(t, All.Has(ReadOnly))
}

func TestCallFlagString(t *testing.T) {
	var cases = map[CallFlag]string{
		NoneFlag:               "None",
		All:                    "All",
		ReadStates:             "ReadStates",
		States:                 "States",
		ReadOnly:               "ReadOnly",
		States | AllowCall:     "ReadOnly, WriteStates",
		ReadOnly | AllowNotify: "ReadOnly, AllowNotify",
		States | AllowNotify:   "States, AllowNotify",
	}
	for f, s := range cases {
		require.Equal(t, s, f.String())
	}
}

func TestFromString(t *testing.T) {
	var cases = map[string]struct {
		flag CallFlag
		err  bool
	}{
		"None":                   {NoneFlag, false},
		"All":                    {All, false},
		"ReadStates":             {ReadStates, false},
		"States":                 {States, false},
		"ReadOnly":               {ReadOnly, false},
		"ReadOnly, WriteStates":  {States | AllowCall, false},
		"States, AllowCall":      {States | AllowCall, false},
		"AllowCall, States":      {States | AllowCall, false},
		"States, ReadOnly":       {States | AllowCall, false},
		" AllowCall,AllowNotify": {AllowNotify | AllowCall, false},
		"BlahBlah":               {NoneFlag, true},
		"States, All":            {NoneFlag, true},
		"ReadStates,,AllowCall":  {NoneFlag, true},
		"ReadStates;AllowCall":   {NoneFlag, true},
		"readstates":             {NoneFlag, true},
		"  All":                  {NoneFlag, true},
		"None, All":              {NoneFlag, true},
	}
	for s, res := range cases {
		f, err := FromString(s)
		require.True(t, res.err == (err != nil), "Input: '"+s+"'")
		require.Equal(t, res.flag, f)
	}
}

func TestMarshalUnmarshalJSON(t *testing.T) {
	var f = States
	testserdes.MarshalUnmarshalJSON(t, &f, new(CallFlag))
	f = States | AllowNotify
	testserdes.MarshalUnmarshalJSON(t, &f, new(CallFlag))

	forig := f
	err := f.UnmarshalJSON([]byte("42"))
	require.Error(t, err)
	require.Equal(t, forig, f)

	err = f.UnmarshalJSON([]byte(`"State"`))
	require.Error(t, err)
	require.Equal(t, forig, f)
}

func TestMarshalUnmarshalYAML(t *testing.T) {
	for _, expected := range []CallFlag{States, States | AllowNotify} {
		data, err := yaml.Marshal(expected)
		require.NoError(t, err)

		var f CallFlag
		require.NoError(t, yaml.Unmarshal(data, &f))
		require.Equal(t, expected, f)
	}

	var f CallFlag
	require.Error(t, yaml.Unmarshal([]byte(`[]`), &f))
}
