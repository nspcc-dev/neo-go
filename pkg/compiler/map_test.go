package compiler_test

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

var mapTestCases = []testCase{
	{
		"map composite literal",
		`
		package foo
		func Main() int {
			t := map[int]int{
				1: 6,
				2: 9,
			}

			age := t[2]
			return age
		}
		`,
		big.NewInt(9),
	},
	{
		"nested map",
		`
		package foo
		func Main() int {
		t := map[int]map[int]int{
			1: map[int]int{2: 5, 3: 1},
			2: nil,
			5: map[int]int{3: 4, 7: 2},
		}

		x := t[5][3]
		return x
	}
	`,
		big.NewInt(4),
	},
	{
		"map with string index",
		`
		package foo
		func Main() string {
			t := map[string]string{
				"name": "Valera",
				"age": "33",
			}

			name := t["name"]
			return name
		}
		`,
		[]byte("Valera"),
	},
	{
		"delete key",
		`package foo
		func Main() int {
			m := map[int]int{1: 2, 3: 4}
			delete(m, 1)
			return len(m)
		}`,
		big.NewInt(1),
	},
	{
		"delete missing key",
		`package foo
		func Main() int {
			m := map[int]int{3: 4}
			delete(m, 1)
			return len(m)
		}`,
		big.NewInt(1),
	},
	{
		"int value, existing",
		`package foo
		func Main() int {
			var m = map[string]int{
				"key": 1,
			}
			v, ok := m["key"]
			if !ok {
				panic("key is missing")
			}
			return v
		}`,
		big.NewInt(1),
	},
	{
		"int value, missing",
		`package foo
		func Main() int {
			var m = make(map[string]int)
			v, ok := m["unknown"]
			if ok {
				panic("key is existing")
			}
			return v
		}`,
		big.NewInt(0),
	},
	{
		"slice value, existing",
		`package foo
		func Main() []int {
			var m = map[string][]int{
				"key": []int{0, 1, 2},
			}
			var v, ok = m["key"]
			if !ok {
				panic("key is missing")
			}
			return v
		}`,
		[]stackitem.Item{stackitem.Make(0), stackitem.Make(1), stackitem.Make(2)},
	},
	{
		"slice value, missing",
		`package foo
		func Main() []int {
			var m = make(map[string][]int)
			var v, ok = m["unknown"]
			if ok {
				panic("key is existing")
			}
			return v
		}`,
		nil,
	},
	{
		"map value, missing",
		`package foo
		func Main() map[string]int {
			var m = make(map[string]map[string]int)
			v, ok := m["unknown"]
			if ok {
				panic("key is existing")
			}
			return v
		}`,
		nil,
	},
	{
		"struct value, missing",
		`package foo
		type S struct {
			A int
		}
		func Main() S {
			var m = make(map[string]S)
			var v, ok = m["key"]
			if ok {
				panic("key is existing")
			}
			return v
		}`,
		[]stackitem.Item{stackitem.Make(0)},
	},
	{
		"if with comma-ok idiom",
		`package foo
		func Main() bool {
			var m = map[string]int{
				"key": 1,
			}
			if v, ok := m["key"]; !ok || v != 1{
				return false
			}
			if _, ok := m["unknown"]; ok {
				return false
			}
			return true
		}`,
		true,
	},
	{
		"map returned from function, existing",
		`package foo
		func Get() map[string]int {
			return map[string]int{"key": 1}
		}
		func Main() int {
			var v, ok = Get()["key"]
			if !ok {
				panic("key is missing")
			}
			return v
		}`,
		big.NewInt(1),
	},
	{
		"drop value, existing",
		`package foo
		func Main() bool {
			var m = map[string]int{
				"key": 1,
			}
			_, ok := m["key"]
			return ok
		}`,
		true,
	},
	{
		"drop flag, existing",
		`package foo
		func Main() int {
			var m = map[string]int{
				"key": 1,
			}
			var v, _ = m["key"]
			return v
		}`,
		big.NewInt(1),
	},
	{
		"drop value, missing",
		`package foo
		func Main() bool {
			var m = make(map[string]int)
			var _, ok = m["unknown"]
			return ok
		}`,
		false,
	},
	{
		"drop flag, missing",
		`package foo
		func Main() int {
			var m = make(map[string]int)
			v, _ := m["unknown"]
			return v
		}`,
		big.NewInt(0),
	},
	{
		"drop value and flag, missing",
		`package foo
		var v int
		func getKey() string {
			v = 5
			return "key"
		}
		func Main() int {
			var m = map[string]int{
				"key": 1,
			}
			_, _ = m[getKey()]
			return v
		}`,
		big.NewInt(5),
	},
	{
		"swap map elements",
		`package foo
		func Main() map[string]int {
			m := map[string]int{"a":1, "b":2}
			m["a"], m["b"] = m["b"], m["a"]
			return m
		}
		`,
		[]stackitem.MapElement{
			{
				Key:   stackitem.Make("a"),
				Value: stackitem.Make(2),
			},
			{
				Key:   stackitem.Make("b"),
				Value: stackitem.Make(1),
			},
		},
	},
}

func TestMaps(t *testing.T) {
	runTestCases(t, mapTestCases)
}
