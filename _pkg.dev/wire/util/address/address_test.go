package address

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScriptHash(t *testing.T) {
	address := "AJeAEsmeD6t279Dx4n2HWdUvUmmXQ4iJvP"

	hash := ToScriptHash(address)
	reverseHash := ToReverseScriptHash(address)

	assert.Equal(t, "b28427088a3729b2536d10122960394e8be6721f", reverseHash)
	assert.Equal(t, "1f72e68b4e39602912106d53b229378a082784b2", hash)
}
