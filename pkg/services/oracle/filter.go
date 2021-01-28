package oracle

import (
	"encoding/json"
	"errors"
	"unicode/utf8"

	"github.com/PaesslerAG/jsonpath"
)

func filter(value []byte, path string) ([]byte, error) {
	if !utf8.Valid(value) {
		return nil, errors.New("not an UTF-8")
	}
	var v interface{}
	if err := json.Unmarshal(value, &v); err != nil {
		return nil, err
	}
	result, err := jsonpath.Get(path, v)
	if err != nil {
		return nil, err
	}
	return json.Marshal([]interface{}{result})
}
