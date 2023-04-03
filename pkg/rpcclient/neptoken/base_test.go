package neptoken

import (
	"errors"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

type testInv struct {
	err error
	res *result.Invoke
}

func (t *testInv) Call(contract util.Uint160, operation string, params ...any) (*result.Invoke, error) {
	return t.res, t.err
}

func TestBaseErrors(t *testing.T) {
	ti := new(testInv)
	base := New(ti, util.Uint160{1, 2, 3})

	ti.err = errors.New("")
	_, err := base.Decimals()
	require.Error(t, err)
	_, err = base.Symbol()
	require.Error(t, err)
	_, err = base.TotalSupply()
	require.Error(t, err)
	_, err = base.BalanceOf(util.Uint160{1, 2, 3})
	require.Error(t, err)

	ti.err = nil
	ti.res = &result.Invoke{
		State:          "FAULT",
		FaultException: "bad thing happened",
	}
	_, err = base.Decimals()
	require.Error(t, err)
	_, err = base.Symbol()
	require.Error(t, err)
	_, err = base.TotalSupply()
	require.Error(t, err)
	_, err = base.BalanceOf(util.Uint160{1, 2, 3})
	require.Error(t, err)

	ti.res = &result.Invoke{
		State: "HALT",
	}
	_, err = base.Decimals()
	require.Error(t, err)
	_, err = base.Symbol()
	require.Error(t, err)
	_, err = base.TotalSupply()
	require.Error(t, err)
	_, err = base.BalanceOf(util.Uint160{1, 2, 3})
	require.Error(t, err)
}

func TestBaseDecimals(t *testing.T) {
	ti := new(testInv)
	base := New(ti, util.Uint160{1, 2, 3})

	ti.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(0),
		},
	}
	dec, err := base.Decimals()
	require.NoError(t, err)
	require.Equal(t, 0, dec)

	ti.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(-1),
		},
	}
	_, err = base.Decimals()
	require.Error(t, err)

	ti.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(100500),
		},
	}
	_, err = base.Decimals()
	require.Error(t, err)
}

func TestBaseSymbol(t *testing.T) {
	ti := new(testInv)
	base := New(ti, util.Uint160{1, 2, 3})

	ti.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make("SYM"),
		},
	}
	sym, err := base.Symbol()
	require.NoError(t, err)
	require.Equal(t, "SYM", sym)

	ti.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make("\xff"),
		},
	}
	_, err = base.Symbol()
	require.Error(t, err)
}

func TestBaseTotalSupply(t *testing.T) {
	ti := new(testInv)
	base := New(ti, util.Uint160{1, 2, 3})

	ti.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(100500),
		},
	}
	ts, err := base.TotalSupply()
	require.NoError(t, err)
	require.Equal(t, big.NewInt(100500), ts)

	ti.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{}),
		},
	}
	_, err = base.TotalSupply()
	require.Error(t, err)
}

func TestBaseBalanceOf(t *testing.T) {
	ti := new(testInv)
	base := New(ti, util.Uint160{1, 2, 3})

	ti.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(100500),
		},
	}
	bal, err := base.BalanceOf(util.Uint160{1, 2, 3})
	require.NoError(t, err)
	require.Equal(t, big.NewInt(100500), bal)

	ti.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{}),
		},
	}
	_, err = base.BalanceOf(util.Uint160{1, 2, 3})
	require.Error(t, err)
}
