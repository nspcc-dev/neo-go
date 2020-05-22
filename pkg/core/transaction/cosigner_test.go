package transaction

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

func TestCosignerEncodeDecode(t *testing.T) {
	expected := &Cosigner{
		Account:          util.Uint160{1, 2, 3, 4, 5},
		Scopes:           CustomContracts,
		AllowedContracts: []util.Uint160{{1, 2, 3, 4}, {6, 7, 8, 9}},
	}
	actual := &Cosigner{}
	testserdes.EncodeDecodeBinary(t, expected, actual)
}

func TestCosignerMarshallUnmarshallJSON(t *testing.T) {
	expected := &Cosigner{
		Account:          util.Uint160{1, 2, 3, 4, 5},
		Scopes:           CustomContracts,
		AllowedContracts: []util.Uint160{{1, 2, 3, 4}, {6, 7, 8, 9}},
	}
	actual := &Cosigner{}
	testserdes.MarshalUnmarshalJSON(t, expected, actual)
}
