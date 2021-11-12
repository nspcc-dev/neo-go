package transaction

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

type InvalidCondition struct{}

func (c InvalidCondition) Type() WitnessConditionType {
	return 0xff
}
func (c InvalidCondition) Match(_ MatchContext) (bool, error) {
	return true, nil
}
func (c InvalidCondition) EncodeBinary(w *io.BinWriter) {
	w.WriteB(byte(c.Type()))
}
func (c InvalidCondition) DecodeBinarySpecific(r *io.BinReader, _ int) {
}
func (c InvalidCondition) MarshalJSON() ([]byte, error) {
	aux := conditionAux{
		Type: c.Type().String(),
	}
	return json.Marshal(aux)
}

type condCase struct {
	condition WitnessCondition
	success   bool
}

func TestWitnessConditionSerDes(t *testing.T) {
	var someBool bool
	pk, err := keys.NewPrivateKey()
	require.NoError(t, err)
	var cases = []condCase{
		{(*ConditionBoolean)(&someBool), true},
		{&ConditionNot{(*ConditionBoolean)(&someBool)}, true},
		{&ConditionAnd{(*ConditionBoolean)(&someBool), (*ConditionBoolean)(&someBool)}, true},
		{&ConditionOr{(*ConditionBoolean)(&someBool), (*ConditionBoolean)(&someBool)}, true},
		{&ConditionScriptHash{1, 2, 3}, true},
		{(*ConditionGroup)(pk.PublicKey()), true},
		{ConditionCalledByEntry{}, true},
		{&ConditionCalledByContract{1, 2, 3}, true},
		{(*ConditionCalledByGroup)(pk.PublicKey()), true},
		{InvalidCondition{}, false},
		{&ConditionAnd{}, false},
		{&ConditionOr{}, false},
		{&ConditionNot{&ConditionNot{&ConditionNot{(*ConditionBoolean)(&someBool)}}}, false},
	}
	var maxSubCondAnd = &ConditionAnd{}
	var maxSubCondOr = &ConditionAnd{}
	for i := 0; i < maxSubitems+1; i++ {
		*maxSubCondAnd = append(*maxSubCondAnd, (*ConditionBoolean)(&someBool))
		*maxSubCondOr = append(*maxSubCondOr, (*ConditionBoolean)(&someBool))
	}
	cases = append(cases, condCase{maxSubCondAnd, false})
	cases = append(cases, condCase{maxSubCondOr, false})
	t.Run("binary", func(t *testing.T) {
		for i, c := range cases {
			w := io.NewBufBinWriter()
			c.condition.EncodeBinary(w.BinWriter)
			require.NoError(t, w.Err)
			b := w.Bytes()

			r := io.NewBinReaderFromBuf(b)
			res := DecodeBinaryCondition(r)
			if !c.success {
				require.Nil(t, res)
				require.Errorf(t, r.Err, "case %d", i)
				continue
			}
			require.NoErrorf(t, r.Err, "case %d", i)
			require.Equal(t, c.condition, res)
		}
	})
	t.Run("json", func(t *testing.T) {
		for i, c := range cases {
			jj, err := c.condition.MarshalJSON()
			require.NoError(t, err)
			res, err := UnmarshalConditionJSON(jj)
			if !c.success {
				require.Errorf(t, err, "case %d, json %s", i, jj)
				continue
			}
			require.NoErrorf(t, err, "case %d, json %s", i, jj)
			require.Equal(t, c.condition, res)
		}
	})
}

func TestWitnessConditionZeroDeser(t *testing.T) {
	r := io.NewBinReaderFromBuf([]byte{})
	res := DecodeBinaryCondition(r)
	require.Nil(t, res)
	require.Error(t, r.Err)
}

func TestWitnessConditionJSONErrors(t *testing.T) {
	var cases = []string{
		`[]`,
		`{}`,
		`{"type":"Boolean"}`,
		`{"type":"Not"}`,
		`{"type":"And"}`,
		`{"type":"Or"}`,
		`{"type":"ScriptHash"}`,
		`{"type":"Group"}`,
		`{"type":"CalledByContract"}`,
		`{"type":"CalledByGroup"}`,
		`{"type":"Boolean", "expression":42}`,
		`{"type":"Not", "expression":true}`,
		`{"type":"And", "expressions":[{"type":"CalledByGroup"},{"type":"Not", "expression":true}]}`,
		`{"type":"Or", "expressions":{"type":"CalledByGroup"}}`,
		`{"type":"Or", "expressions":[{"type":"CalledByGroup"},{"type":"Not", "expression":false}]}`,
		`{"type":"ScriptHash", "hash":"1122"}`,
		`{"type":"Group", "group":"032211"}`,
		`{"type":"CalledByContract", "hash":"1122"}`,
		`{"type":"CalledByGroup", "group":"032211"}`,
	}
	for i := range cases {
		res, err := UnmarshalConditionJSON([]byte(cases[i]))
		require.Errorf(t, err, "case %d, json %s", i, cases[i])
		require.Nil(t, res)
	}
}

type TestMC struct {
	calling util.Uint160
	current util.Uint160
	entry   util.Uint160
	goodKey *keys.PublicKey
	badKey  *keys.PublicKey
}

func (t *TestMC) GetCallingScriptHash() util.Uint160 {
	return t.calling
}
func (t *TestMC) GetCurrentScriptHash() util.Uint160 {
	return t.current
}
func (t *TestMC) GetEntryScriptHash() util.Uint160 {
	return t.entry
}
func (t *TestMC) CallingScriptHasGroup(k *keys.PublicKey) (bool, error) {
	res, err := t.CurrentScriptHasGroup(k)
	return !res, err // To differentiate from current we invert the logic value.
}
func (t *TestMC) CurrentScriptHasGroup(k *keys.PublicKey) (bool, error) {
	if k.Equal(t.goodKey) {
		return true, nil
	}
	if k.Equal(t.badKey) {
		return false, errors.New("baaad key")
	}
	return false, nil
}

func TestWitnessConditionMatch(t *testing.T) {
	pkGood, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pkBad, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pkNeutral, err := keys.NewPrivateKey()
	require.NoError(t, err)
	entrySC := util.Uint160{1, 2, 3}
	currentSC := util.Uint160{4, 5, 6}
	tmc := &TestMC{
		calling: entrySC,
		entry:   entrySC,
		current: currentSC,
		goodKey: pkGood.PublicKey(),
		badKey:  pkBad.PublicKey(),
	}

	t.Run("boolean", func(t *testing.T) {
		var b bool
		var c = (*ConditionBoolean)(&b)
		res, err := c.Match(tmc)
		require.NoError(t, err)
		require.False(t, res)
		b = true
		res, err = c.Match(tmc)
		require.NoError(t, err)
		require.True(t, res)
	})
	t.Run("not", func(t *testing.T) {
		var b bool
		var cInner = (*ConditionBoolean)(&b)
		var cInner2 = (*ConditionGroup)(pkBad.PublicKey())
		var c = &ConditionNot{cInner}
		var c2 = &ConditionNot{cInner2}

		res, err := c.Match(tmc)
		require.NoError(t, err)
		require.True(t, res)
		b = true
		res, err = c.Match(tmc)
		require.NoError(t, err)
		require.False(t, res)
		_, err = c2.Match(tmc)
		require.Error(t, err)
	})
	t.Run("and", func(t *testing.T) {
		var bFalse, bTrue bool
		var cInnerFalse = (*ConditionBoolean)(&bFalse)
		var cInnerTrue = (*ConditionBoolean)(&bTrue)
		var cInnerBad = (*ConditionGroup)(pkBad.PublicKey())
		var c = &ConditionAnd{cInnerTrue, cInnerFalse, cInnerFalse}
		var cBad = &ConditionAnd{cInnerTrue, cInnerBad}

		bTrue = true
		res, err := c.Match(tmc)
		require.NoError(t, err)
		require.False(t, res)
		bFalse = true
		res, err = c.Match(tmc)
		require.NoError(t, err)
		require.True(t, res)

		_, err = cBad.Match(tmc)
		require.Error(t, err)
	})
	t.Run("or", func(t *testing.T) {
		var bFalse, bTrue bool
		var cInnerFalse = (*ConditionBoolean)(&bFalse)
		var cInnerTrue = (*ConditionBoolean)(&bTrue)
		var cInnerBad = (*ConditionGroup)(pkBad.PublicKey())
		var c = &ConditionOr{cInnerTrue, cInnerFalse, cInnerFalse}
		var cBad = &ConditionOr{cInnerTrue, cInnerBad}

		bTrue = true
		res, err := c.Match(tmc)
		require.NoError(t, err)
		require.True(t, res)
		bTrue = false
		res, err = c.Match(tmc)
		require.NoError(t, err)
		require.False(t, res)

		_, err = cBad.Match(tmc)
		require.Error(t, err)
	})
	t.Run("script hash", func(t *testing.T) {
		var cEntry = (*ConditionScriptHash)(&entrySC)
		var cCurrent = (*ConditionScriptHash)(&currentSC)

		res, err := cEntry.Match(tmc)
		require.NoError(t, err)
		require.False(t, res)
		res, err = cCurrent.Match(tmc)
		require.NoError(t, err)
		require.True(t, res)
	})
	t.Run("group", func(t *testing.T) {
		var cBad = (*ConditionGroup)(pkBad.PublicKey())
		var cGood = (*ConditionGroup)(pkGood.PublicKey())
		var cNeutral = (*ConditionGroup)(pkNeutral.PublicKey())

		res, err := cGood.Match(tmc)
		require.NoError(t, err)
		require.True(t, res)

		res, err = cNeutral.Match(tmc)
		require.NoError(t, err)
		require.False(t, res)

		_, err = cBad.Match(tmc)
		require.Error(t, err)
	})
	t.Run("called by entry", func(t *testing.T) {
		var c = ConditionCalledByEntry{}

		res, err := c.Match(tmc)
		require.NoError(t, err)
		require.True(t, res)

		tmc2 := *tmc
		tmc2.entry = util.Uint160{0, 9, 8}
		res, err = c.Match(&tmc2)
		require.NoError(t, err)
		require.False(t, res)

		tmc3 := *tmc
		tmc3.calling = util.Uint160{}
		tmc3.current = tmc3.entry
		res, err = c.Match(&tmc3)
		require.NoError(t, err)
		require.True(t, res)
	})
	t.Run("called by contract", func(t *testing.T) {
		var cEntry = (*ConditionCalledByContract)(&entrySC)
		var cCurrent = (*ConditionCalledByContract)(&currentSC)

		res, err := cEntry.Match(tmc)
		require.NoError(t, err)
		require.True(t, res)
		res, err = cCurrent.Match(tmc)
		require.NoError(t, err)
		require.False(t, res)
	})
	t.Run("called by group", func(t *testing.T) {
		var cBad = (*ConditionCalledByGroup)(pkBad.PublicKey())
		var cGood = (*ConditionCalledByGroup)(pkGood.PublicKey())
		var cNeutral = (*ConditionCalledByGroup)(pkNeutral.PublicKey())

		res, err := cGood.Match(tmc)
		require.NoError(t, err)
		require.False(t, res)

		res, err = cNeutral.Match(tmc)
		require.NoError(t, err)
		require.True(t, res)

		_, err = cBad.Match(tmc)
		require.Error(t, err)
	})
}
