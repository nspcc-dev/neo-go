package oracle

import (
	"bytes"
	"errors"
	"unicode/utf8"

	json "github.com/nspcc-dev/go-ordered-json"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/services/oracle/jsonpath"
)

func filter(value []byte, path string) ([]byte, error) {
	if !utf8.Valid(value) {
		return nil, errors.New("not an UTF-8")
	}

	buf := bytes.NewBuffer(value)
	d := json.NewDecoder(buf)
	d.UseOrderedObject()

	var v interface{}
	if err := d.Decode(&v); err != nil {
		return nil, err
	}

	result, _, ok := jsonpath.Get(path, v, transaction.MaxOracleResultSize)
	if !ok {
		return nil, errors.New("invalid filter")
	}
	return json.Marshal(result)
}

func filterRequest(result []byte, req *state.OracleRequest) ([]byte, error) {
	if req.Filter != nil {
		return filter(result, *req.Filter)
	}
	return result, nil
}
