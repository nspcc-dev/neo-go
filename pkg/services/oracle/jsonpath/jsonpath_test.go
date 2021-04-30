package jsonpath

import (
	"encoding/json"
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

type pathTestCase struct {
	path   string
	result string
}

func unmarshalGet(t *testing.T, js string, path string) ([]interface{}, bool) {
	var v interface{}
	require.NoError(t, json.Unmarshal([]byte(js), &v))
	return Get(path, v)
}

func (p *pathTestCase) testUnmarshalGet(t *testing.T, js string) {
	res, ok := unmarshalGet(t, js, p.path)
	require.True(t, ok)

	data, err := json.Marshal(res)
	require.NoError(t, err)
	require.JSONEq(t, p.result, string(data))
}

func TestInvalidPaths(t *testing.T) {
	bigNum := strconv.FormatInt(int64(math.MaxInt32)+1, 10)

	// errCases contains invalid json path expressions.
	// These are either invalid(&) or unexpected token in some positions
	// or big number/invalid string.
	errCases := []string{
		".",
		"$1",
		"&",
		"$&",
		"$.&",
		"$.[0]",
		"$..",
		"$..*",
		"$..&",
		"$..1",
		"$[&]",
		"$[**]",
		"$[1&]",
		"$[" + bigNum + "]",
		"$[" + bigNum + ":]",
		"$[:" + bigNum + "]",
		"$[1," + bigNum + "]",
		"$[" + bigNum + "[]]",
		"$['a'&]",
		"$['a'1]",
		"$['a",
		"$['\\u123']",
		"$['s','\\u123']",
		"$[[]]",
		"$[1,'a']",
		"$[1,1&",
		"$[1,1[]]",
		"$[1:&]",
		"$[1:1[]]",
		"$[1:[]]",
		"$[1:[]]",
		"$[",
	}

	for _, tc := range errCases {
		t.Run(tc, func(t *testing.T) {
			_, ok := unmarshalGet(t, "{}", tc)
			require.False(t, ok)
		})
	}
}

func TestDescendByIdent(t *testing.T) {
	js := `{
		"store": {
			"name": "big",
			"sub": [
				{ "name": "sub1" },
				{ "name": "sub2" }
			],
			"partner": { "name": "ppp" }
		},
		"another": { "name": "small" }
	}`

	testCases := []pathTestCase{
		{"$.store.name", `["big"]`},
		{"$['store']['name']", `["big"]`},
		{"$[*].name", `["small","big"]`},
		{"$.*.name", `["small","big"]`},
		{"$..store.name", `["big"]`},
		{"$.store..name", `["big","ppp","sub1","sub2"]`},
		{"$..sub.name", `[]`},
		{"$..sub..name", `["sub1","sub2"]`},
	}
	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			tc.testUnmarshalGet(t, js)
		})
	}

	t.Run("big depth", func(t *testing.T) {
		js := `{"a":{"b":{"c":{"d":{"e":{"f":{"g":1}}}}}}}`
		t.Run("single field", func(t *testing.T) {
			t.Run("max", func(t *testing.T) {
				p := pathTestCase{"$.a.b.c.d.e.f", `[{"g":1}]`}
				p.testUnmarshalGet(t, js)
			})

			_, ok := unmarshalGet(t, js, "$.a.b.c.d.e.f.g")
			require.False(t, ok)
		})
		t.Run("wildcard", func(t *testing.T) {
			t.Run("max", func(t *testing.T) {
				p := pathTestCase{"$.*.*.*.*.*.*", `[{"g":1}]`}
				p.testUnmarshalGet(t, js)
			})

			_, ok := unmarshalGet(t, js, "$.*.*.*.*.*.*.*")
			require.False(t, ok)
		})
	})
}

func TestDescendByIndex(t *testing.T) {
	js := `["a","b","c","d"]`

	testCases := []pathTestCase{
		{"$[0]", `["a"]`},
		{"$[3]", `["d"]`},
		{"$[1:2]", `["b"]`},
		{"$[1:-1]", `["b","c"]`},
		{"$[-3:-1]", `["b","c"]`},
		{"$[-3:3]", `["b","c"]`},
		{"$[:3]", `["a","b","c"]`},
		{"$[:100]", `["a","b","c","d"]`},
		{"$[1:]", `["b","c","d"]`},
		{"$[2:1]", `[]`},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			tc.testUnmarshalGet(t, js)
		})
	}

	t.Run("big depth", func(t *testing.T) {
		js := `[[[[[[[1]]]]]]]`
		t.Run("single index", func(t *testing.T) {
			t.Run("max", func(t *testing.T) {
				p := pathTestCase{"$[0][0][0][0][0][0]", "[[1]]"}
				p.testUnmarshalGet(t, js)
			})

			_, ok := unmarshalGet(t, js, "$[0][0][0][0][0][0][0]")
			require.False(t, ok)
		})
		t.Run("slice", func(t *testing.T) {
			t.Run("max", func(t *testing.T) {
				p := pathTestCase{"$[0:][0:][0:][0:][0:][0:]", "[[1]]"}
				p.testUnmarshalGet(t, js)
			})

			_, ok := unmarshalGet(t, js, "$[0:][0:][0:][0:][0:][0:][0:]")
			require.False(t, ok)
		})
	})

	t.Run("$[:][1], skip wrong types", func(t *testing.T) {
		js := `[[1,2],{"1":"4"},[5,6]]`
		p := pathTestCase{"$[:][1]", "[2,6]"}
		p.testUnmarshalGet(t, js)
	})

	t.Run("$[*].*, flatten", func(t *testing.T) {
		js := `[[1,2],{"1":"4"},[5,6]]`
		p := pathTestCase{"$[*].*", "[1,2,\"4\",5,6]"}
		p.testUnmarshalGet(t, js)
	})

	t.Run("$[*].[1:], skip wrong types", func(t *testing.T) {
		js := `[[1,2],3,{"1":"4"},[5,6]]`
		p := pathTestCase{"$[*][1:]", "[2,6]"}
		p.testUnmarshalGet(t, js)
	})
}

func TestUnion(t *testing.T) {
	js := `["a",{"x":1,"y":2,"z":3},"c","d"]`

	testCases := []pathTestCase{
		{"$[0,2]", `["a","c"]`},
		{"$[1]['x','z']", `[1,3]`},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			tc.testUnmarshalGet(t, js)
		})
	}
}

// These tests are taken directly from C# code.
func TestCSharpCompat(t *testing.T) {
	js := `{
    "store": {
        "book": [
            {
                "category": "reference",
                "author": "Nigel Rees",
                "title": "Sayings of the Century",
                "price": 8.95
            },
            {
                "category": "fiction",
                "author": "Evelyn Waugh",
                "title": "Sword of Honour",
                "price": 12.99
            },
            {
                "category": "fiction",
                "author": "Herman Melville",
                "title": "Moby Dick",
                "isbn": "0-553-21311-3",
                "price": 8.99
            },
            {
                "category": "fiction",
                "author": "J. R. R. Tolkien",
                "title": "The Lord of the Rings",
                "isbn": "0-395-19395-8",
                "price": 22.99
            }
        ],
        "bicycle": {
                "color": "red",
            "price": 19.95
        }
        },
    "expensive": 10
}`

	// FIXME(fyrchik): some tests are commented because of how maps are processed.
	testCases := []pathTestCase{
		{"$.store.book[*].author", `["Nigel Rees","Evelyn Waugh","Herman Melville","J. R. R. Tolkien"]`},
		{"$..author", `["Nigel Rees","Evelyn Waugh","Herman Melville","J. R. R. Tolkien"]`},
		//{"$.store.*", `[[{"category":"reference","author":"Nigel Rees","title":"Sayings of the Century","price":8.95},{"category":"fiction","author":"Evelyn Waugh","title":"Sword of Honour","price":12.99},{"category":"fiction","author":"Herman Melville","title":"Moby Dick","isbn":"0-553-21311-3","price":8.99},{"category":"fiction","author":"J. R. R. Tolkien","title":"The Lord of the Rings","isbn":"0-395-19395-8","price":22.99}],{"color":"red","price":19.95}]`},
		{"$.store..price", `[19.95,8.95,12.99,8.99,22.99]`},
		{"$..book[2]", `[{"category":"fiction","author":"Herman Melville","title":"Moby Dick","isbn":"0-553-21311-3","price":8.99}]`},
		{"$..book[-2]", `[{"category":"fiction","author":"Herman Melville","title":"Moby Dick","isbn":"0-553-21311-3","price":8.99}]`},
		{"$..book[0,1]", `[{"category":"reference","author":"Nigel Rees","title":"Sayings of the Century","price":8.95},{"category":"fiction","author":"Evelyn Waugh","title":"Sword of Honour","price":12.99}]`},
		{"$..book[:2]", `[{"category":"reference","author":"Nigel Rees","title":"Sayings of the Century","price":8.95},{"category":"fiction","author":"Evelyn Waugh","title":"Sword of Honour","price":12.99}]`},
		{"$..book[1:2]", `[{"category":"fiction","author":"Evelyn Waugh","title":"Sword of Honour","price":12.99}]`},
		{"$..book[-2:]", `[{"category":"fiction","author":"Herman Melville","title":"Moby Dick","isbn":"0-553-21311-3","price":8.99},{"category":"fiction","author":"J. R. R. Tolkien","title":"The Lord of the Rings","isbn":"0-395-19395-8","price":22.99}]`},
		{"$..book[2:]", `[{"category":"fiction","author":"Herman Melville","title":"Moby Dick","isbn":"0-553-21311-3","price":8.99},{"category":"fiction","author":"J. R. R. Tolkien","title":"The Lord of the Rings","isbn":"0-395-19395-8","price":22.99}]`},
		//{"$..*", `[{"book":[{"category":"reference","author":"Nigel Rees","title":"Sayings of the Century","price":8.95},{"category":"fiction","author":"Evelyn Waugh","title":"Sword of Honour","price":12.99},{"category":"fiction","author":"Herman Melville","title":"Moby Dick","isbn":"0-553-21311-3","price":8.99},{"category":"fiction","author":"J. R. R. Tolkien","title":"The Lord of the Rings","isbn":"0-395-19395-8","price":22.99}],"bicycle":{"color":"red","price":19.95}},10,[{"category":"reference","author":"Nigel Rees","title":"Sayings of the Century","price":8.95},{"category":"fiction","author":"Evelyn Waugh","title":"Sword of Honour","price":12.99},{"category":"fiction","author":"Herman Melville","title":"Moby Dick","isbn":"0-553-21311-3","price":8.99},{"category":"fiction","author":"J. R. R. Tolkien","title":"The Lord of the Rings","isbn":"0-395-19395-8","price":22.99}],{"color":"red","price":19.95},{"category":"reference","author":"Nigel Rees","title":"Sayings of the Century","price":8.95},{"category":"fiction","author":"Evelyn Waugh","title":"Sword of Honour","price":12.99},{"category":"fiction","author":"Herman Melville","title":"Moby Dick","isbn":"0-553-21311-3","price":8.99},{"category":"fiction","author":"J. R. R. Tolkien","title":"The Lord of the Rings","isbn":"0-395-19395-8","price":22.99},"red",19.95,"reference","Nigel Rees","Sayings of the Century",8.95,"fiction","Evelyn Waugh","Sword of Honour",12.99,"fiction","Herman Melville","Moby Dick","0-553-21311-3",8.99,"fiction","J. R. R. Tolkien","The Lord of the Rings","0-395-19395-8",22.99]`},
		{"$..invalidfield", `[]`},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			tc.testUnmarshalGet(t, js)
		})
	}

	t.Run("bad cases", func(t *testing.T) {
		_, ok := unmarshalGet(t, js, `$..book[*].author"`)
		require.False(t, ok)
	})
}
