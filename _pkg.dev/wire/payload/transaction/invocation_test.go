package transaction

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeInvoc(t *testing.T) {
	// taken from mainnet b2a22cd9dd7636ae23e25576866cd1d9e2f3d85a85e80874441f085cd60006d1

	rawtx := "d10151050034e23004141ad842821c7341d5a32b17d7177a1750d30014ca14628c9e5bc6a9346ca6bcdf050ceabdeb2bdc774953c1087472616e736665726703e1df72015bdef1a1b9567d4700635f23b1f406f100000000000000000220628c9e5bc6a9346ca6bcdf050ceabdeb2bdc7749f02f31363a30373a3032203a2030333366616431392d643638322d343035382d626437662d31356339333132343433653800000141403ced56c16f933e0a0a7d37470e114f6a4216ef9b834d61db67b74b9bd117370d10870857c0ee8adcf9956bc9fc92c5158de0c2db34ef459c17de042f20ad8fe92321027392870a5994b090d1750dda173a54df8dad324ed6d9ed25290d17c59059a112ac"
	rawtxBytes, _ := hex.DecodeString(rawtx)

	i := NewInvocation(30)

	r := bytes.NewReader(rawtxBytes)
	err := i.Decode(r)
	assert.Equal(t, nil, err)

	assert.Equal(t, types.Invocation, i.Type)

	assert.Equal(t, 2, len(i.Attributes))

	attr1 := i.Attributes[0]
	assert.Equal(t, Script, attr1.Usage)
	assert.Equal(t, "628c9e5bc6a9346ca6bcdf050ceabdeb2bdc7749", hex.EncodeToString(attr1.Data))

	attr2 := i.Attributes[1]
	assert.Equal(t, Remark, attr2.Usage)
	assert.Equal(t, "31363a30373a3032203a2030333366616431392d643638322d343035382d626437662d313563393331323434336538", hex.EncodeToString(attr2.Data))

	assert.Equal(t, "050034e23004141ad842821c7341d5a32b17d7177a1750d30014ca14628c9e5bc6a9346ca6bcdf050ceabdeb2bdc774953c1087472616e736665726703e1df72015bdef1a1b9567d4700635f23b1f406f1", hex.EncodeToString(i.Script))
	assert.Equal(t, "b2a22cd9dd7636ae23e25576866cd1d9e2f3d85a85e80874441f085cd60006d1", i.Hash.ReverseString())

	// Encode
	buf := new(bytes.Buffer)
	err = i.Encode(buf)
	assert.Equal(t, nil, err)
	assert.Equal(t, rawtxBytes, buf.Bytes())
}

func TestEncodeDecodeInvocAttributes(t *testing.T) {
	// taken from mainnet cb0b5edc7e87b3b1bd9e029112fd3ce17c16d3de20c43ca1c0c26f3add578ecb

	rawtx := "d1015308005b950f5e010000140000000000000000000000000000000000000000141a1e29d6232d2148e1e71e30249835ea41eb7a3d53c1087472616e7366657267fb1c540417067c270dee32f21023aa8b9b71abce000000000000000002201a1e29d6232d2148e1e71e30249835ea41eb7a3d8110f9f504da6334935a2db42b18296d88700000014140461370f6847c4abbdddff54a3e1337e453ecc8133c882ec5b9aabcf0f47dafd3432d47e449f4efc77447ef03519b7808c450a998cca3ecc10e6536ed9db862ba23210285264b6f349f0fe86e9bb3044fde8f705b016593cf88cd5e8a802b78c7d2c950ac"
	rawtxBytes, _ := hex.DecodeString(rawtx)

	i := NewInvocation(30)

	r := bytes.NewReader(rawtxBytes)
	err := i.Decode(r)
	assert.Equal(t, nil, err)

	assert.Equal(t, types.Invocation, i.Type)

	assert.Equal(t, 1, int(i.Version))

	assert.Equal(t, 2, len(i.Attributes))

	assert.Equal(t, Script, i.Attributes[0].Usage)
	assert.Equal(t, "1a1e29d6232d2148e1e71e30249835ea41eb7a3d", hex.EncodeToString(i.Attributes[0].Data))
	assert.Equal(t, DescriptionURL, i.Attributes[1].Usage)
	assert.Equal(t, "f9f504da6334935a2db42b18296d8870", hex.EncodeToString(i.Attributes[1].Data))

	assert.Equal(t, "08005b950f5e010000140000000000000000000000000000000000000000141a1e29d6232d2148e1e71e30249835ea41eb7a3d53c1087472616e7366657267fb1c540417067c270dee32f21023aa8b9b71abce", hex.EncodeToString(i.Script))
	assert.Equal(t, "cb0b5edc7e87b3b1bd9e029112fd3ce17c16d3de20c43ca1c0c26f3add578ecb", i.Hash.ReverseString())

	// Encode
	buf := new(bytes.Buffer)
	err = i.Encode(buf)
	assert.Equal(t, nil, err)
	assert.Equal(t, rawtxBytes, buf.Bytes())

}
