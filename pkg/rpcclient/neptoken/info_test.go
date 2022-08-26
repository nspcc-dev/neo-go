package neptoken

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

type rpcClient struct {
	cnt     int
	cserr   error
	cs      *state.Contract
	inverrs []error
	invs    []*result.Invoke
}

func (r *rpcClient) InvokeContractVerify(contract util.Uint160, params []smartcontract.Parameter, signers []transaction.Signer, witnesses ...transaction.Witness) (*result.Invoke, error) {
	panic("not implemented")
}
func (r *rpcClient) InvokeFunction(contract util.Uint160, operation string, params []smartcontract.Parameter, signers []transaction.Signer) (*result.Invoke, error) {
	e, i := r.inverrs[r.cnt], r.invs[r.cnt]
	r.cnt = (r.cnt + 1) % len(r.invs)
	return i, e
}
func (r *rpcClient) InvokeScript(script []byte, signers []transaction.Signer) (*result.Invoke, error) {
	panic("not implemented")
}
func (r *rpcClient) TerminateSession(sessionID uuid.UUID) (bool, error) {
	panic("not implemented")
}
func (r *rpcClient) TraverseIterator(sessionID, iteratorID uuid.UUID, maxItemsCount int) ([]stackitem.Item, error) {
	panic("not implemented")
}
func (r *rpcClient) GetContractStateByHash(hash util.Uint160) (*state.Contract, error) {
	return r.cs, r.cserr
}

func TestInfo(t *testing.T) {
	c := &rpcClient{}
	hash := util.Uint160{1, 2, 3}

	// Error on contract state.
	c.cserr = errors.New("")
	_, err := Info(c, hash)
	require.Error(t, err)

	// Error on missing standard.
	c.cserr = nil
	c.cs = &state.Contract{
		ContractBase: state.ContractBase{
			Manifest: manifest.Manifest{
				Name:               "Vasiliy",
				SupportedStandards: []string{"RFC 1149"},
			},
		},
	}
	_, err = Info(c, hash)
	require.Error(t, err)

	// Error on Symbol()
	c.cs = &state.Contract{
		ContractBase: state.ContractBase{
			Manifest: manifest.Manifest{
				Name:               "Übertoken",
				SupportedStandards: []string{"NEP-17"},
			},
		},
	}
	c.inverrs = []error{errors.New(""), nil}
	c.invs = []*result.Invoke{nil, nil}
	_, err = Info(c, hash)
	require.Error(t, err)

	// Error on Decimals()
	c.cnt = 0
	c.inverrs[0], c.inverrs[1] = c.inverrs[1], c.inverrs[0]
	c.invs[0] = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make("UBT"),
		},
	}
	_, err = Info(c, hash)
	require.Error(t, err)

	// OK
	c.cnt = 0
	c.inverrs[1] = nil
	c.invs[1] = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(8),
		},
	}
	ti, err := Info(c, hash)
	require.NoError(t, err)
	require.Equal(t, &wallet.Token{
		Name:     "Übertoken",
		Hash:     hash,
		Decimals: 8,
		Symbol:   "UBT",
		Standard: "NEP-17",
	}, ti)

	// NEP-11
	c.cs = &state.Contract{
		ContractBase: state.ContractBase{
			Manifest: manifest.Manifest{
				Name:               "NFTizer",
				SupportedStandards: []string{"NEP-11"},
			},
		},
	}
	c.cnt = 0
	c.inverrs[1] = nil
	c.invs[0] = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make("NZ"),
		},
	}
	c.invs[1] = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(0),
		},
	}
	ti, err = Info(c, hash)
	require.NoError(t, err)
	require.Equal(t, &wallet.Token{
		Name:     "NFTizer",
		Hash:     hash,
		Decimals: 0,
		Symbol:   "NZ",
		Standard: "NEP-11",
	}, ti)
}
