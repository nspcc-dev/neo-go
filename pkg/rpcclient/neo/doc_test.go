package neo_test

import (
	"cmp"
	"context"
	"math/big"
	"slices"

	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/neo"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

func ExampleContractReader() {
	// No error checking done at all, intentionally.
	c, _ := rpcclient.New(context.Background(), "url", rpcclient.Options{})

	// Safe methods are reachable with just an invoker, no need for an account there.
	inv := invoker.New(c, nil)

	// Create a reader interface.
	neoToken := neo.NewReader(inv)

	// Account hash we're interested in.
	accHash, _ := address.StringToUint160("NdypBhqkz2CMMnwxBgvoC9X2XjKF5axgKo")

	// Get the account balance.
	balance, _ := neoToken.BalanceOf(accHash)
	_ = balance

	// Get the extended NEO-specific balance data.
	bNeo, _ := neoToken.GetAccountState(accHash)

	// Account can have no associated vote.
	if bNeo.VoteTo == nil {
		return
	}
	// Committee keys.
	comm, _ := neoToken.GetCommittee()

	// Check if the vote is made for a committee member.
	var votedForCommitteeMember bool
	for i := range comm {
		if bNeo.VoteTo.Equal(comm[i]) {
			votedForCommitteeMember = true
			break
		}
	}
	_ = votedForCommitteeMember
}

func ExampleContract() {
	// No error checking done at all, intentionally.
	w, _ := wallet.NewWalletFromFile("somewhere")
	defer w.Close()

	c, _ := rpcclient.New(context.Background(), "url", rpcclient.Options{})

	// Create a simple CalledByEntry-scoped actor (assuming there is an account
	// inside the wallet).
	a, _ := actor.NewSimple(c, w.Accounts[0])

	// Create a complete contract representation.
	neoToken := neo.New(a)

	tgtAcc, _ := address.StringToUint160("NdypBhqkz2CMMnwxBgvoC9X2XjKF5axgKo")

	// Send a transaction that transfers one token to another account.
	txid, vub, _ := neoToken.Transfer(a.Sender(), tgtAcc, big.NewInt(1), nil)
	_ = txid
	_ = vub

	// Get a list of candidates (it's limited, but should be sufficient in most cases).
	cands, _ := neoToken.GetCandidates()

	// Sort by votes.
	slices.SortFunc(cands, func(a, b result.Validator) int { return cmp.Compare(a.Votes, b.Votes) })

	// Get the extended NEO-specific balance data.
	bNeo, _ := neoToken.GetAccountState(a.Sender())

	// If not yet voted, or voted for suboptimal candidate (we want the one with the least votes),
	// send a new voting transaction
	if bNeo.VoteTo == nil || !bNeo.VoteTo.Equal(&cands[0].PublicKey) {
		txid, vub, _ = neoToken.Vote(a.Sender(), &cands[0].PublicKey)
		_ = txid
		_ = vub
	}
}
