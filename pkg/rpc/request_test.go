package rpc_test

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/rpc"
	"github.com/stretchr/testify/assert"
)

func TestRequest(t *testing.T) {

	t.Run("NewRequest()", func(t *testing.T) {
		t.Run("ValidJSONRPCPayload", func(t *testing.T) {
			requestBodyData := `{"jsonrpc": "2.0", "method": "getbestblockhash", "params": [], "id": 1}`
			requestBody := ioutil.NopCloser(bytes.NewReader([]byte(requestBodyData)))

			request := rpc.NewRequest(requestBody)
			assert.False(t, request.HasError())

			id, err := request.ID()
			assert.NoError(t, err)
			assert.Equal(t, 1, id)

			assert.Equal(t, "2.0", request.JSONRPC)
			assert.Equal(t, "getbestblockhash", request.Method)
		})

		t.Run("InvalidVersion", func(t *testing.T) {
			requestBodyData := `{"jsonrpc": "two", "method": "getbestblockhash", "params": [], "id": 1}`
			requestBody := ioutil.NopCloser(bytes.NewReader([]byte(requestBodyData)))

			request := rpc.NewRequest(requestBody)
			assert.True(t, request.HasError())
		})
	})

	t.Run(".Params()", func(t *testing.T) {
		requestBodyData := `{"jsonrpc": "2.0", "method": "getbestblockhash", "params": ["1", 2], "id": 1}`
		requestBody := ioutil.NopCloser(bytes.NewReader([]byte(requestBodyData)))

		request := rpc.NewRequest(requestBody)
		assert.False(t, request.HasError())

		params := request.Params()

		assert.Len(t, params, 2)
		assert.Equal(t, "1", params.StringValueAt(0))
		assert.Equal(t, float64(2), params.FloatValueAt(1))
	})
}
