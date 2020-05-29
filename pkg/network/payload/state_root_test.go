package payload

import (
	"math/rand"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
)

func TestGetStateRoot_Serializable(t *testing.T) {
	expected := &GetStateRoot{
		Index: rand.Uint32(),
	}
	testserdes.EncodeDecodeBinary(t, expected, new(GetStateRoot))
}
