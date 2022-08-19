package nep11

import (
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestNDOwnerOf(t *testing.T) {
	ta := new(testAct)
	tr := NewNonDivisibleReader(ta, util.Uint160{1, 2, 3})
	tt := NewNonDivisible(ta, util.Uint160{1, 2, 3})

	for name, fun := range map[string]func([]byte) (util.Uint160, error){
		"Reader": tr.OwnerOf,
		"Full":   tt.OwnerOf,
	} {
		t.Run(name, func(t *testing.T) {
			ta.err = errors.New("")
			_, err := fun([]byte{3, 2, 1})
			require.Error(t, err)

			ta.err = nil
			ta.res = &result.Invoke{
				State: "HALT",
				Stack: []stackitem.Item{
					stackitem.Make(100500),
				},
			}
			_, err = fun([]byte{3, 2, 1})
			require.Error(t, err)

			own := util.Uint160{1, 2, 3}
			ta.res = &result.Invoke{
				State: "HALT",
				Stack: []stackitem.Item{
					stackitem.Make(own.BytesBE()),
				},
			}
			owl, err := fun([]byte{3, 2, 1})
			require.NoError(t, err)
			require.Equal(t, own, owl)
		})
	}
}
