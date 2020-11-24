package transaction

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

func TestCosignerEncodeDecode(t *testing.T) {
	expected := &Signer{
		Account:          util.Uint160{1, 2, 3, 4, 5},
		Scopes:           CustomContracts,
		AllowedContracts: []util.Uint160{{1, 2, 3, 4}, {6, 7, 8, 9}},
	}
	actual := &Signer{}
	testserdes.EncodeDecodeBinary(t, expected, actual)
}

func TestCosignerMarshallUnmarshallJSON(t *testing.T) {
	expected := &Signer{
		Account:          util.Uint160{1, 2, 3, 4, 5},
		Scopes:           CustomContracts,
		AllowedContracts: []util.Uint160{{1, 2, 3, 4}, {6, 7, 8, 9}},
	}
	actual := &Signer{}
	testserdes.MarshalUnmarshalJSON(t, expected, actual)
}
