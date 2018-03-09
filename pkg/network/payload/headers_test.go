package payload

import (
	"bytes"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/stretchr/testify/assert"
)

func TestHeadersEncodeDecode(t *testing.T) {
	headers := &Headers{[]*core.Header{
		&core.Header{
			BlockBase: core.BlockBase{
				Version: 0,
				Index:   1,
				Script: &transaction.Witness{
					InvocationScript:   []byte{0x0},
					VerificationScript: []byte{0x1},
				},
			}},
		&core.Header{
			BlockBase: core.BlockBase{
				Version: 0,
				Index:   2,
				Script: &transaction.Witness{
					InvocationScript:   []byte{0x0},
					VerificationScript: []byte{0x1},
				},
			}},
		&core.Header{
			BlockBase: core.BlockBase{
				Version: 0,
				Index:   3,
				Script: &transaction.Witness{
					InvocationScript:   []byte{0x0},
					VerificationScript: []byte{0x1},
				},
			}},
	}}

	buf := new(bytes.Buffer)
	if err := headers.EncodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	headersDecode := &Headers{}
	if err := headersDecode.DecodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < len(headers.Hdrs); i++ {
		assert.Equal(t, headers.Hdrs[i].Version, headersDecode.Hdrs[i].Version)
		assert.Equal(t, headers.Hdrs[i].Index, headersDecode.Hdrs[i].Index)
		assert.Equal(t, headers.Hdrs[i].Script, headersDecode.Hdrs[i].Script)
	}
}
