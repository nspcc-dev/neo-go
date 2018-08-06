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
	assert.Equal(t, "b2a22cd9dd7636ae23e25576866cd1d9e2f3d85a85e80874441f085cd60006d1", i.Hash.String())

	// Encode
	buf := new(bytes.Buffer)
	err = i.Encode(buf)
	assert.Equal(t, nil, err)
	assert.Equal(t, rawtxBytes, buf.Bytes())
}
