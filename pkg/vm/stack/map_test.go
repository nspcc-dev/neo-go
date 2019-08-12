package stack

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMap(t *testing.T) {
	// define Map m for testing
	var a Item = testMakeStackInt(t, 10)
	var b Item = NewBoolean(true)
	var c Item = NewByteArray([]byte{1, 2, 34})
	var d Item = testMakeStackMap(t, map[Item]Item{
		a: c,
		b: a,
	})
	var e = NewContext([]byte{1, 2, 3, 4})
	var f = testMakeArray(t, []Item{a, b})

	val := map[Item]Item{
		a: c,
		b: a,
		c: b,
		d: a,
		e: d,
		f: e,
	}
	m := testMakeStackMap(t, val)

	// test ValueOfKey
	valueA, _ := m.ValueOfKey(testMakeStackInt(t, 10))
	assert.Equal(t, c, valueA)

	valueB, _ := m.ValueOfKey(b)
	assert.Equal(t, a, valueB)

	valueC, _ := m.ValueOfKey(NewByteArray([]byte{1, 2, 34}))
	assert.Equal(t, b, valueC)

	valueD, _ := m.ValueOfKey(testMakeStackMap(t, map[Item]Item{
		b: a,
		a: c,
	}))
	assert.Equal(t, a, valueD)

	valueE, _ := m.ValueOfKey(NewContext([]byte{1, 2, 3, 4}))
	assert.Equal(t, d, valueE)

	valueF, _ := m.ValueOfKey(testMakeArray(t, []Item{a, b}))
	assert.Equal(t, e, valueF)

	valueX, _ := m.ValueOfKey(NewByteArray([]byte{1, 2, 35}))
	assert.NotEqual(t, b, valueX)

	checkA, err := m.ContainsKey(a)
	assert.Nil(t, err)
	assert.Equal(t, true, checkA.Value())

	//test ContainsKey
	checkB, err := m.ContainsKey(b)
	assert.Nil(t, err)
	assert.Equal(t, true, checkB.Value())

	checkC, err := m.ContainsKey(c)
	assert.Nil(t, err)
	assert.Equal(t, true, checkC.Value())

	checkD, err := m.ContainsKey(d)
	assert.Nil(t, err)
	assert.Equal(t, true, checkD.Value())

	checkE, err := m.ContainsKey(e)
	assert.Nil(t, err)
	assert.Equal(t, true, checkE.Value())

	//test CompareHash
	val2 := map[Item]Item{
		f: e,
		e: d,
		d: a,
		c: b,
		b: a,
		a: c,
	}
	m2 := testMakeStackMap(t, val2)
	checkMap, err := CompareHash(m, m2)
	assert.Nil(t, err)
	assert.Equal(t, true, checkMap.Value())

	checkBoolean, err := CompareHash(b, NewBoolean(true))
	assert.Nil(t, err)
	assert.Equal(t, true, checkBoolean.Value())

	checkByteArray, err := CompareHash(c, NewByteArray([]byte{1, 2, 34}))
	assert.Nil(t, err)
	assert.Equal(t, true, checkByteArray.Value())

	checkContext, err := CompareHash(e, NewContext([]byte{1, 2, 3, 4}))
	assert.Nil(t, err)
	assert.Equal(t, true, checkContext.Value())

	checkArray, err := CompareHash(f, testMakeArray(t, []Item{a, b}))
	assert.Nil(t, err)
	assert.Equal(t, true, checkArray.Value())
}

func TestMapAdd(t *testing.T) {
	var a Item = testMakeStackInt(t, 10)
	var b Item = NewBoolean(true)
	var m = testMakeStackMap(t, map[Item]Item{})

	err := m.Add(a, a)
	assert.Nil(t, err)
	err = m.Add(b, a)
	assert.Nil(t, err)

	assert.Equal(t, 2, len(m.Value()))

	expected := testMakeStackMap(t, map[Item]Item{b: a, a: a})
	check, err := CompareHash(m, expected)
	assert.Nil(t, err)
	assert.Equal(t, true, check.Value())

}

func TestMapRemove(t *testing.T) {
	var a Item = testMakeStackInt(t, 10)
	var b Item = NewBoolean(true)
	var m = testMakeStackMap(t, map[Item]Item{b: a, a: a})

	err := m.Remove(a)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(m.Value()))

	expected := testMakeStackMap(t, map[Item]Item{b: a})
	check, err := CompareHash(m, expected)
	assert.Nil(t, err)
	assert.Equal(t, true, check.Value())

}
