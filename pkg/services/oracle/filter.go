package oracle

import (
	"encoding/json"
	"errors"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/services/oracle/jsonpath"
)

func filter(value []byte, path string) ([]byte, error) {
	if !utf8.Valid(value) {
		return nil, errors.New("not an UTF-8")
	}
	var v interface{}
	if err := json.Unmarshal(value, &v); err != nil {
		return nil, err
	}

	result, ok := jsonpath.Get(path, v)
	if !ok {
		return nil, errors.New("invalid filter")
	}
	return json.Marshal(result)
}

func filterRequest(result []byte, req *state.OracleRequest) (transaction.OracleResponseCode, []byte) {
	if req.Filter != nil {
		var err error
		result, err = filter(result, *req.Filter)
		if err != nil {
			return transaction.Error, nil
		}
	}
	return transaction.Success, result
}
