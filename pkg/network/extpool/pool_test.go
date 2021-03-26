package extpool

import (
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestAddGet(t *testing.T) {
	bc := newTestChain()
	bc.height = 10

	p := New(bc)
	t.Run("invalid witness", func(t *testing.T) {
		ep := &payload.Extensible{ValidBlockEnd: 100, Sender: util.Uint160{0x42}}
		p.testAdd(t, false, errVerification, ep)
	})
	t.Run("disallowed sender", func(t *testing.T) {
		ep := &payload.Extensible{ValidBlockEnd: 100, Sender: util.Uint160{0x41}}
		p.testAdd(t, false, errDisallowedSender, ep)
	})
	t.Run("bad height", func(t *testing.T) {
		ep := &payload.Extensible{ValidBlockEnd: 9}
		p.testAdd(t, false, errInvalidHeight, ep)

		ep = &payload.Extensible{ValidBlockEnd: 10}
		p.testAdd(t, false, nil, ep)
	})
	t.Run("good", func(t *testing.T) {
		ep := &payload.Extensible{ValidBlockEnd: 100}
		p.testAdd(t, true, nil, ep)
		require.Equal(t, ep, p.Get(ep.Hash()))

		p.testAdd(t, false, nil, ep)
	})
}

func TestRemoveStale(t *testing.T) {
	bc := newTestChain()
	bc.height = 10

	p := New(bc)
	eps := []*payload.Extensible{
		{ValidBlockEnd: 11},                             // small height
		{ValidBlockEnd: 12},                             // good
		{Sender: util.Uint160{0x11}, ValidBlockEnd: 12}, // invalid sender
		{Sender: util.Uint160{0x12}, ValidBlockEnd: 12}, // invalid witness
	}
	for i := range eps {
		p.testAdd(t, true, nil, eps[i])
	}
	bc.verifyWitness = func(u util.Uint160) bool { println("call"); return u[0] != 0x12 }
	bc.isAllowed = func(u util.Uint160) bool { return u[0] != 0x11 }
	p.RemoveStale(11)
	require.Nil(t, p.Get(eps[0].Hash()))
	require.Equal(t, eps[1], p.Get(eps[1].Hash()))
	require.Nil(t, p.Get(eps[2].Hash()))
	require.Nil(t, p.Get(eps[3].Hash()))
}

func (p *Pool) testAdd(t *testing.T, expectedOk bool, expectedErr error, ep *payload.Extensible) {
	ok, err := p.Add(ep)
	if expectedErr != nil {
		require.True(t, errors.Is(err, expectedErr), "got: %v", err)
	} else {
		require.NoError(t, err)
	}
	require.Equal(t, expectedOk, ok)
}

type testChain struct {
	blockchainer.Blockchainer
	height        uint32
	verifyWitness func(util.Uint160) bool
	isAllowed     func(util.Uint160) bool
}

var errVerification = errors.New("verification failed")

func newTestChain() *testChain {
	return &testChain{
		verifyWitness: func(u util.Uint160) bool {
			return u[0] != 0x42
		},
		isAllowed: func(u util.Uint160) bool {
			return u[0] != 0x42 && u[0] != 0x41
		},
	}
}
func (c *testChain) VerifyWitness(u util.Uint160, _ hash.Hashable, _ *transaction.Witness, _ int64) error {
	if !c.verifyWitness(u) {
		return errVerification
	}
	return nil
}
func (c *testChain) IsExtensibleAllowed(u util.Uint160) bool {
	return c.isAllowed(u)
}
func (c *testChain) BlockHeight() uint32 { return c.height }
