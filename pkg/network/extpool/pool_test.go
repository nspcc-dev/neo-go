package extpool

import (
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestAddGet(t *testing.T) {
	bc := newTestChain()
	bc.height = 10

	p := New(bc, 100, func(hs []util.Uint256) {})
	p.Start()
	t.Cleanup(p.Stop)
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
		p.testAdd(t, false, ErrInvalidHeight, ep)

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
		require.Panics(t, func() { New(bc, 0, nil) })
	})

	p := New(bc, 3, func(hs []util.Uint256) {})
	p.Start()
	t.Cleanup(p.Stop)

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

	p := New(bc, 2, func(hs []util.Uint256) {})
	p.Start()
	t.Cleanup(p.Stop)
	senders := []util.Uint160{{1}, {2}, {3}}
	for i := uint32(11); i < 17; i++ {
		ep := &payload.Extensible{Sender: senders[i%3], ValidBlockEnd: i}
		p.testAdd(t, true, nil, ep)
	}
}

func TestRemoveStale(t *testing.T) {
	bc := newTestChain()
	bc.height = 10

	p := New(bc, 100, func(hs []util.Uint256) {})
	p.Start()
	t.Cleanup(p.Stop)
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

func TestRebroadcast(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		bc := newTestChain()
		bc.height = 10
		bc.msPerBlock = 100500
		resendThreshold := time.Millisecond * time.Duration(bc.msPerBlock) / 2

		var (
			lock             sync.RWMutex
			expected, actual []util.Uint256
		)
		check := func(t *testing.T) {
			lock.RLock()
			require.ElementsMatch(t, expected, actual) // the order in pool's hashes map is undefined.
			lock.RUnlock()
		}
		p := New(bc, 100, func(hs []util.Uint256) {
			lock.Lock()
			defer lock.Unlock()
			actual = append(actual, hs...)
		})
		p.Start()
		t.Cleanup(p.Stop)
		ep1 := &payload.Extensible{ValidBlockEnd: 100, Category: payload.ConsensusCategory}
		p.testAdd(t, true, nil, ep1)

		// Check payload is rebroadcasted through resendThreshold, 2*resendThreshold, 3*resendThreshold.
		for i := range 3 {
			time.Sleep(resendThreshold<<i - 1) // timer is not yet fired.
			synctest.Wait()
			check(t)

			// Wait 1ns more, timer should fire.
			time.Sleep(1) //nolint:staticcheck
			synctest.Wait()
			expected = append(expected, ep1.Hash())
			check(t)
		}

		// Sleep almost until timer firing and reset it via Add.
		time.Sleep(resendThreshold<<3 - 1)
		synctest.Wait()
		check(t)
		ep2 := &payload.Extensible{ValidBlockEnd: 101, Category: payload.ConsensusCategory}
		p.testAdd(t, true, nil, ep2)
		// Sleeping the rest of 1ns won't trigger the timer since it was reset.
		time.Sleep(1) //nolint:staticcheck
		synctest.Wait()
		check(t)

		// Wait the rest of resendThreshold for timer firing.
		time.Sleep(resendThreshold - 1)
		synctest.Wait()
		expected = append(expected, ep1.Hash(), ep2.Hash())
		check(t)

		// Ensure the timer is fired after RemoveStale if there's at least one consensus extensible left.
		p.RemoveStale(100) // one payload should be left.
		synctest.Wait()
		require.Equal(t, []util.Uint256{ep2.Hash()}, p.GetCategory(payload.ConsensusCategory))
		check(t)
		time.Sleep(resendThreshold)
		synctest.Wait()
		expected = append(expected, ep2.Hash())
		check(t)

		// Ensure the timer won't fire if no consensus payloads left.
		p.RemoveStale(101) // one payload should be left.
		synctest.Wait()
		require.Equal(t, []util.Uint256(nil), p.GetCategory(payload.ConsensusCategory))
		check(t)
		time.Sleep(resendThreshold)
		synctest.Wait()
		check(t)
	})
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
	msPerBlock    uint32
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
func (c *testChain) BlockHeight() uint32             { return c.height }
func (c *testChain) GetMillisecondsPerBlock() uint32 { return c.msPerBlock }
