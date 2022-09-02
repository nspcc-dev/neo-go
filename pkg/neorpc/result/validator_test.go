package result

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatorUnmarshal(t *testing.T) {
	old := []byte(`{"publickey":"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62","votes":"100500","active":true}`)
	v := new(Validator)
	require.NoError(t, json.Unmarshal(old, v))
	require.Equal(t, int64(100500), v.Votes)

	newV := []byte(`{"publickey":"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62","votes":42}`)
	require.NoError(t, json.Unmarshal(newV, v))
	require.Equal(t, int64(42), v.Votes)

	bad := []byte(`{"publickey":"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62","votes":"notanumber"}`)
	require.Error(t, json.Unmarshal(bad, v))
}
