package extpool

import (
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestAddGet(t *testing.T) {
	bc := newTestChain()
	bc.height = 10

	p := New(bc, 100)
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

func TestCapacityLimit(t *testing.T) {
	bc := newTestChain()
	bc.height = 10

	t.Run("invalid capacity", func(t *testing.T) {
		require.Panics(t, func() { New(bc, 0) })
	})

	p := New(bc, 3)

	first := &payload.Extensible{ValidBlockEnd: 11}
	p.testAdd(t, true, nil, first)

	for _, height := range []uint32{12, 13} {
		ep := &payload.Extensible{ValidBlockEnd: height}
		p.testAdd(t, true, nil, ep)
	}

	require.NotNil(t, p.Get(first.Hash()))

	ok, err := p.Add(&payload.Extensible{ValidBlockEnd: 14})
	require.True(t, ok)
	require.NoError(t, err)

	require.Nil(t, p.Get(first.Hash()))
}

// This test checks that sender count is updated
// when oldest payload is removed during `Add`.
func TestDecreaseSenderOnEvict(t *testing.T) {
	bc := newTestChain()
	bc.height = 10

	p := New(bc, 2)
	senders := []util.Uint160{{1}, {2}, {3}}
	for i := uint32(11); i < 17; i++ {
		ep := &payload.Extensible{Sender: senders[i%3], ValidBlockEnd: i}
		p.testAdd(t, true, nil, ep)
	}
}

func TestRemoveStale(t *testing.T) {
	bc := newTestChain()
	bc.height = 10

	p := New(bc, 100)
	eps := []*payload.Extensible{
		{ValidBlockEnd: 11},                             // small height
		{ValidBlockEnd: 12},                             // good
		{Sender: util.Uint160{0x11}, ValidBlockEnd: 12}, // invalid sender
		{Sender: util.Uint160{0x12}, ValidBlockEnd: 12}, // invalid witness
	}
	for i := range eps {
		p.testAdd(t, true, nil, eps[i])
	}
	bc.verifyWitness = func(u util.Uint160) bool { return u[0] != 0x12 }
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
		require.ErrorIs(t, err, expectedErr)
	} else {
		require.NoError(t, err)
	}
	require.Equal(t, expectedOk, ok)
}

type testChain struct {
	Ledger
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
func (c *testChain) VerifyWitness(u util.Uint160, _ hash.Hashable, _ *transaction.Witness, _ int64) (int64, error) {
	if !c.verifyWitness(u) {
		return 0, errVerification
	}
	return 0, nil
}
func (c *testChain) IsExtensibleAllowed(u util.Uint160) bool {
	return c.isAllowed(u)
}
func (c *testChain) BlockHeight() uint32 { return c.height }
