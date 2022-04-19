package native

import (
	"crypto/elliptic"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// NEO represents NEO native contract.
type NEO struct {
	nep17TokenNative
	GAS    *GAS
	Policy *Policy

	// Configuration and standby keys are set in constructor and then
	// only read from.
	cfg         config.ProtocolConfiguration
	standbyKeys keys.PublicKeys
}

type NeoCache struct {
	// gasPerBlock represents current value of generated gas per block.
	// It is append-only and doesn't need to be copied when used.
	gasPerBlock        gasRecord
	gasPerBlockChanged bool

	registerPrice        int64
	registerPriceChanged bool

	votesChanged   bool
	nextValidators keys.PublicKeys
	validators     keys.PublicKeys
	// committee contains cached committee members and their votes.
	// It is updated once in a while depending on committee size
	// (every 28 blocks for mainnet). It's value
	// is always equal to value stored by `prefixCommittee`.
	committee keysWithVotes
	// committeeHash contains script hash of the committee.
	committeeHash util.Uint160

	// gasPerVoteCache contains last updated value of GAS per vote reward for candidates.
	// It is set in state-modifying methods only and read in `PostPersist` thus is not protected
	// by any mutex.
	gasPerVoteCache map[string]big.Int
}

const (
	neoContractID = -5
	// NEOTotalSupply is the total amount of NEO in the system.
	NEOTotalSupply = 100000000
	// DefaultRegisterPrice is default price for candidate register.
	DefaultRegisterPrice = 1000 * GASFactor
	// prefixCandidate is a prefix used to store validator's data.
	prefixCandidate = 33
	// prefixVotersCount is a prefix for storing total amount of NEO of voters.
	prefixVotersCount = 1
	// prefixVoterRewardPerCommittee is a prefix for storing committee GAS reward.
	prefixVoterRewardPerCommittee = 23
	// voterRewardFactor is a factor by which voter reward per committee is multiplied
	// to make calculations more precise.
	voterRewardFactor = 100_000_000
	// prefixGASPerBlock is a prefix for storing amount of GAS generated per block.
	prefixGASPerBlock = 29
	// prefixRegisterPrice is a prefix for storing candidate register price.
	prefixRegisterPrice = 13
	// effectiveVoterTurnout represents minimal ratio of total supply to total amount voted value
	// which is require to use non-standby validators.
	effectiveVoterTurnout = 5
	// neoHolderRewardRatio is a percent of generated GAS that is distributed to NEO holders.
	neoHolderRewardRatio = 10
	// neoHolderRewardRatio is a percent of generated GAS that is distributed to committee.
	committeeRewardRatio = 10
	// neoHolderRewardRatio is a percent of generated GAS that is distributed to voters.
	voterRewardRatio = 80
)

var (
	// prefixCommittee is a key used to store committee.
	prefixCommittee = []byte{14}

	bigCommitteeRewardRatio  = big.NewInt(committeeRewardRatio)
	bigVoterRewardRatio      = big.NewInt(voterRewardRatio)
	bigVoterRewardFactor     = big.NewInt(voterRewardFactor)
	bigEffectiveVoterTurnout = big.NewInt(effectiveVoterTurnout)
	big100                   = big.NewInt(100)
)

var (
	_ interop.Contract            = (*NEO)(nil)
	_ storage.NativeContractCache = (*NeoCache)(nil)
)

// Copy implements NativeContractCache interface.
func (c *NeoCache) Copy() storage.NativeContractCache {
	cp := &NeoCache{}
	copyNeoCache(c, cp)
	return cp
}

// Persist implements NativeContractCache interface.
func (c *NeoCache) Persist(ps storage.NativeContractCache) (storage.NativeContractCache, error) {
	if ps == nil {
		ps = &NeoCache{}
	}
	psCache, ok := ps.(*NeoCache)
	if !ok {
		return nil, errors.New("not a NEO native cache")
	}
	copyNeoCache(c, psCache)
	return psCache, nil
}

func copyNeoCache(src, dst *NeoCache) {
	dst.votesChanged = src.votesChanged
	dst.nextValidators = src.nextValidators.Copy()
	dst.validators = src.validators.Copy()

	dst.committee = make(keysWithVotes, len(src.committee))
	copy(dst.committee, src.committee)
	dst.committeeHash = src.committeeHash

	dst.registerPriceChanged = src.registerPriceChanged
	dst.registerPrice = src.registerPrice

	dst.gasPerBlockChanged = src.gasPerBlockChanged
	dst.gasPerBlock = make(gasRecord, len(src.gasPerBlock))
	copy(dst.gasPerBlock, src.gasPerBlock)

	dst.gasPerVoteCache = make(map[string]big.Int)
	for k, v := range src.gasPerVoteCache {
		dst.gasPerVoteCache[k] = v
	}
}

// makeValidatorKey creates a key from account script hash.
func makeValidatorKey(key *keys.PublicKey) []byte {
	b := key.Bytes()
	// Don't create a new buffer.
	b = append(b, 0)
	copy(b[1:], b[0:])
	b[0] = prefixCandidate
	return b
}

// newNEO returns NEO native contract.
func newNEO(cfg config.ProtocolConfiguration) *NEO {
	n := &NEO{}
	defer n.UpdateHash()

	nep17 := newNEP17Native(nativenames.Neo, neoContractID)
	nep17.symbol = "NEO"
	nep17.decimals = 0
	nep17.factor = 1
	nep17.incBalance = n.increaseBalance
	nep17.balFromBytes = n.balanceFromBytes

	n.nep17TokenNative = *nep17

	err := n.initConfigCache(cfg)
	if err != nil {
		panic(fmt.Errorf("failed to initialize NEO config cache: %w", err))
	}

	desc := newDescriptor("unclaimedGas", smartcontract.IntegerType,
		manifest.NewParameter("account", smartcontract.Hash160Type),
		manifest.NewParameter("end", smartcontract.IntegerType))
	md := newMethodAndPrice(n.unclaimedGas, 1<<17, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("registerCandidate", smartcontract.BoolType,
		manifest.NewParameter("pubkey", smartcontract.PublicKeyType))
	md = newMethodAndPrice(n.registerCandidate, 0, callflag.States)
	n.AddMethod(md, desc)

	desc = newDescriptor("unregisterCandidate", smartcontract.BoolType,
		manifest.NewParameter("pubkey", smartcontract.PublicKeyType))
	md = newMethodAndPrice(n.unregisterCandidate, 1<<16, callflag.States)
	n.AddMethod(md, desc)

	desc = newDescriptor("vote", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type),
		manifest.NewParameter("voteTo", smartcontract.PublicKeyType))
	md = newMethodAndPrice(n.vote, 1<<16, callflag.States)
	n.AddMethod(md, desc)

	desc = newDescriptor("getCandidates", smartcontract.ArrayType)
	md = newMethodAndPrice(n.getCandidatesCall, 1<<22, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("getAccountState", smartcontract.ArrayType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodAndPrice(n.getAccountState, 1<<15, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("getCommittee", smartcontract.ArrayType)
	md = newMethodAndPrice(n.getCommittee, 1<<16, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("getNextBlockValidators", smartcontract.ArrayType)
	md = newMethodAndPrice(n.getNextBlockValidators, 1<<16, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("getGasPerBlock", smartcontract.IntegerType)
	md = newMethodAndPrice(n.getGASPerBlock, 1<<15, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("setGasPerBlock", smartcontract.VoidType,
		manifest.NewParameter("gasPerBlock", smartcontract.IntegerType))
	md = newMethodAndPrice(n.setGASPerBlock, 1<<15, callflag.States)
	n.AddMethod(md, desc)

	desc = newDescriptor("getRegisterPrice", smartcontract.IntegerType)
	md = newMethodAndPrice(n.getRegisterPrice, 1<<15, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("setRegisterPrice", smartcontract.VoidType,
		manifest.NewParameter("registerPrice", smartcontract.IntegerType))
	md = newMethodAndPrice(n.setRegisterPrice, 1<<15, callflag.States)
	n.AddMethod(md, desc)

	return n
}

// Initialize initializes NEO contract.
func (n *NEO) Initialize(ic *interop.Context) error {
	if err := n.nep17TokenNative.Initialize(ic); err != nil {
		return err
	}

	_, totalSupply := n.nep17TokenNative.getTotalSupply(ic.DAO)
	if totalSupply.Sign() != 0 {
		return errors.New("already initialized")
	}

	cache := &NeoCache{
		gasPerVoteCache:      make(map[string]big.Int),
		votesChanged:         true,
		registerPriceChanged: true,
	}

	// We need cache to be present in DAO before the subsequent call to `mint`.
	ic.DAO.Store.SetCache(n.ID, cache)

	committee0 := n.standbyKeys[:n.cfg.GetCommitteeSize(ic.Block.Index)]
	cvs := toKeysWithVotes(committee0)
	err := n.updateCache(cache, cvs, ic.Chain)
	if err != nil {
		return err
	}

	ic.DAO.PutStorageItem(n.ID, prefixCommittee, cvs.Bytes())

	h, err := getStandbyValidatorsHash(ic)
	if err != nil {
		return err
	}
	n.mint(ic, h, big.NewInt(NEOTotalSupply), false)

	var index uint32
	value := big.NewInt(5 * GASFactor)
	n.putGASRecord(ic.DAO, index, value)

	gr := &gasRecord{{Index: index, GASPerBlock: *value}}
	cache.gasPerBlock = *gr
	cache.gasPerBlockChanged = false
	ic.DAO.PutStorageItem(n.ID, []byte{prefixVotersCount}, state.StorageItem{})

	setIntWithKey(n.ID, ic.DAO, []byte{prefixRegisterPrice}, DefaultRegisterPrice)
	cache.registerPrice = int64(DefaultRegisterPrice)
	cache.registerPriceChanged = false

	return nil
}

// InitializeCache initializes all NEO cache with the proper values from storage.
// Cache initialisation should be done apart from Initialize because Initialize is
// called only when deploying native contracts.
func (n *NEO) InitializeCache(bc interop.Ledger, d *dao.Simple) error {
	cache := &NeoCache{
		gasPerVoteCache:      make(map[string]big.Int),
		votesChanged:         true,
		registerPriceChanged: true,
	}

	var committee = keysWithVotes{}
	si := d.GetStorageItem(n.ID, prefixCommittee)
	if err := committee.DecodeBytes(si); err != nil {
		return fmt.Errorf("failed to decode committee: %w", err)
	}
	if err := n.updateCache(cache, committee, bc); err != nil {
		return fmt.Errorf("failed to update cache: %w", err)
	}

	cache.gasPerBlock = n.getSortedGASRecordFromDAO(d)
	cache.gasPerBlockChanged = false

	d.Store.SetCache(n.ID, cache)
	return nil
}

func (n *NEO) initConfigCache(cfg config.ProtocolConfiguration) error {
	var err error

	n.cfg = cfg
	n.standbyKeys, err = keys.NewPublicKeysFromStrings(n.cfg.StandbyCommittee)
	return err
}

func (n *NEO) updateCache(cache *NeoCache, cvs keysWithVotes, bc interop.Ledger) error {
	cache.committee = cvs

	var committee = getCommitteeMembers(cache)
	script, err := smartcontract.CreateMajorityMultiSigRedeemScript(committee.Copy())
	if err != nil {
		return err
	}
	cache.committeeHash = hash.Hash160(script)

	// TODO: use block height from interop context for proper historical calls handling.
	nextVals := committee[:n.cfg.GetNumOfCNs(bc.BlockHeight()+1)].Copy()
	sort.Sort(nextVals)
	cache.nextValidators = nextVals
	return nil
}

func (n *NEO) updateCommittee(cache *NeoCache, ic *interop.Context) error {
	if !cache.votesChanged {
		// We need to put in storage anyway, as it affects dumps
		ic.DAO.PutStorageItem(n.ID, prefixCommittee, cache.committee.Bytes())
		return nil
	}

	_, cvs, err := n.computeCommitteeMembers(ic.Chain, ic.DAO)
	if err != nil {
		return err
	}
	if err := n.updateCache(cache, cvs, ic.Chain); err != nil {
		return err
	}
	cache.votesChanged = false
	ic.DAO.PutStorageItem(n.ID, prefixCommittee, cvs.Bytes())
	return nil
}

// OnPersist implements Contract interface.
func (n *NEO) OnPersist(ic *interop.Context) error {
	if n.cfg.ShouldUpdateCommitteeAt(ic.Block.Index) {
		cache := ic.DAO.Store.GetRWCache(n.ID).(*NeoCache)
		oldKeys := cache.nextValidators
		oldCom := cache.committee
		if n.cfg.GetNumOfCNs(ic.Block.Index) != len(oldKeys) ||
			n.cfg.GetCommitteeSize(ic.Block.Index) != len(oldCom) {
			cache.votesChanged = true
		}
		if err := n.updateCommittee(cache, ic); err != nil {
			return err
		}
	}
	return nil
}

// PostPersist implements Contract interface.
func (n *NEO) PostPersist(ic *interop.Context) error {
	gas := n.GetGASPerBlock(ic.DAO, ic.Block.Index)
	cache := ic.DAO.Store.GetRWCache(n.ID).(*NeoCache)
	pubs := getCommitteeMembers(cache)
	committeeSize := n.cfg.GetCommitteeSize(ic.Block.Index)
	index := int(ic.Block.Index) % committeeSize
	committeeReward := new(big.Int).Mul(gas, bigCommitteeRewardRatio)
	n.GAS.mint(ic, pubs[index].GetScriptHash(), committeeReward.Div(committeeReward, big100), false)

	if n.cfg.ShouldUpdateCommitteeAt(ic.Block.Index) {
		var voterReward = new(big.Int).Set(bigVoterRewardRatio)
		voterReward.Mul(voterReward, gas)
		voterReward.Mul(voterReward, big.NewInt(voterRewardFactor*int64(committeeSize)))
		var validatorsCount = n.cfg.GetNumOfCNs(ic.Block.Index)
		voterReward.Div(voterReward, big.NewInt(int64(committeeSize+validatorsCount)))
		voterReward.Div(voterReward, big100)

		var cs = cache.committee
		var key = make([]byte, 38)
		for i := range cs {
			if cs[i].Votes.Sign() > 0 {
				var tmp = new(big.Int)
				if i < validatorsCount {
					tmp.Set(intTwo)
				} else {
					tmp.Set(intOne)
				}
				tmp.Mul(tmp, voterReward)
				tmp.Div(tmp, cs[i].Votes)

				key = makeVoterKey([]byte(cs[i].Key), key)

				var r *big.Int
				if g, ok := cache.gasPerVoteCache[cs[i].Key]; ok {
					r = &g
				} else {
					reward := n.getGASPerVote(ic.DAO, key[:34], []uint32{ic.Block.Index + 1})
					r = &reward[0]
				}
				tmp.Add(tmp, r)

				binary.BigEndian.PutUint32(key[34:], ic.Block.Index+1)
				cache.gasPerVoteCache[cs[i].Key] = *tmp

				ic.DAO.PutStorageItem(n.ID, key, bigint.ToBytes(tmp))
			}
		}
	}
	if cache.gasPerBlockChanged {
		cache.gasPerBlock = n.getSortedGASRecordFromDAO(ic.DAO)
		cache.gasPerBlockChanged = false
	}

	if cache.registerPriceChanged {
		p := getIntWithKey(n.ID, ic.DAO, []byte{prefixRegisterPrice})
		cache.registerPrice = p
		cache.registerPriceChanged = false
	}
	return nil
}

func (n *NEO) getGASPerVote(d *dao.Simple, key []byte, indexes []uint32) []big.Int {
	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i] < indexes[j]
	})
	start := make([]byte, 4)
	binary.BigEndian.PutUint32(start, indexes[len(indexes)-1])

	need := len(indexes)
	var reward = make([]big.Int, need)
	collected := 0
	d.Seek(n.ID, storage.SeekRange{
		Prefix:    key,
		Start:     start,
		Backwards: true,
	}, func(k, v []byte) bool {
		if len(k) == 4 {
			num := binary.BigEndian.Uint32(k)
			for i, ind := range indexes {
				if reward[i].Sign() == 0 && num <= ind {
					reward[i] = *bigint.FromBytes(v)
					collected++
				}
			}
		}
		return collected < need
	})
	return reward
}

func (n *NEO) increaseBalance(ic *interop.Context, h util.Uint160, si *state.StorageItem, amount *big.Int, checkBal *big.Int) error {
	acc, err := state.NEOBalanceFromBytes(*si)
	if err != nil {
		return err
	}
	if (amount.Sign() == -1 && acc.Balance.CmpAbs(amount) == -1) ||
		(amount.Sign() == 0 && checkBal != nil && acc.Balance.Cmp(checkBal) == -1) {
		return errors.New("insufficient funds")
	}
	if err := n.distributeGas(ic, h, acc); err != nil {
		return err
	}
	if amount.Sign() == 0 {
		*si = acc.Bytes()
		return nil
	}
	if err := n.ModifyAccountVotes(acc, ic.DAO, amount, false); err != nil {
		return err
	}
	if acc.VoteTo != nil {
		if err := n.modifyVoterTurnout(ic.DAO, amount); err != nil {
			return err
		}
	}
	acc.Balance.Add(&acc.Balance, amount)
	if acc.Balance.Sign() != 0 {
		*si = acc.Bytes()
	} else {
		*si = nil
	}
	return nil
}

func (n *NEO) balanceFromBytes(si *state.StorageItem) (*big.Int, error) {
	acc, err := state.NEOBalanceFromBytes(*si)
	if err != nil {
		return nil, err
	}
	return &acc.Balance, err
}

func (n *NEO) distributeGas(ic *interop.Context, h util.Uint160, acc *state.NEOBalance) error {
	if ic.Block == nil || ic.Block.Index == 0 || ic.Block.Index == acc.BalanceHeight {
		return nil
	}
	gen, err := n.calculateBonus(ic.DAO, acc.VoteTo, &acc.Balance, acc.BalanceHeight, ic.Block.Index)
	if err != nil {
		return err
	}
	acc.BalanceHeight = ic.Block.Index

	// Must store acc before GAS distribution to fix acc's BalanceHeight value in the storage for
	// further acc's queries from `onNEP17Payment` if so, see https://github.com/nspcc-dev/neo-go/pull/2181.
	key := makeAccountKey(h)
	ic.DAO.PutStorageItem(n.ID, key, acc.Bytes())

	n.GAS.mint(ic, h, gen, true)
	return nil
}

func (n *NEO) unclaimedGas(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	u := toUint160(args[0])
	end := uint32(toBigInt(args[1]).Int64())
	gen, err := n.CalculateBonus(ic.DAO, u, end)
	if err != nil {
		panic(err)
	}
	return stackitem.NewBigInteger(gen)
}

func (n *NEO) getGASPerBlock(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	gas := n.GetGASPerBlock(ic.DAO, ic.Block.Index)
	return stackitem.NewBigInteger(gas)
}

func (n *NEO) getSortedGASRecordFromDAO(d *dao.Simple) gasRecord {
	var gr = make(gasRecord, 0)
	d.Seek(n.ID, storage.SeekRange{Prefix: []byte{prefixGASPerBlock}}, func(k, v []byte) bool {
		gr = append(gr, gasIndexPair{
			Index:       binary.BigEndian.Uint32(k),
			GASPerBlock: *bigint.FromBytes(v),
		})
		return true
	})
	return gr
}

// GetGASPerBlock returns gas generated for block with provided index.
func (n *NEO) GetGASPerBlock(d *dao.Simple, index uint32) *big.Int {
	cache := d.Store.GetROCache(n.ID).(*NeoCache)
	var gr gasRecord
	if cache.gasPerBlockChanged {
		gr = n.getSortedGASRecordFromDAO(d)
	} else {
		gr = cache.gasPerBlock
	}
	for i := len(gr) - 1; i >= 0; i-- {
		if gr[i].Index <= index {
			g := gr[i].GASPerBlock
			return &g
		}
	}
	panic("contract not initialized")
}

// GetCommitteeAddress returns address of the committee.
func (n *NEO) GetCommitteeAddress(d *dao.Simple) util.Uint160 {
	cache := d.Store.GetROCache(n.ID).(*NeoCache)
	return cache.committeeHash
}

func (n *NEO) checkCommittee(ic *interop.Context) bool {
	ok, err := runtime.CheckHashedWitness(ic, n.GetCommitteeAddress(ic.DAO))
	if err != nil {
		panic(err)
	}
	return ok
}

func (n *NEO) setGASPerBlock(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	gas := toBigInt(args[0])
	err := n.SetGASPerBlock(ic, ic.Block.Index+1, gas)
	if err != nil {
		panic(err)
	}
	return stackitem.Null{}
}

// SetGASPerBlock sets gas generated for blocks after index.
func (n *NEO) SetGASPerBlock(ic *interop.Context, index uint32, gas *big.Int) error {
	if gas.Sign() == -1 || gas.Cmp(big.NewInt(10*GASFactor)) == 1 {
		return errors.New("invalid value for GASPerBlock")
	}
	if !n.checkCommittee(ic) {
		return errors.New("invalid committee signature")
	}
	cache := ic.DAO.Store.GetRWCache(n.ID).(*NeoCache)
	cache.gasPerBlockChanged = true
	n.putGASRecord(ic.DAO, index, gas)
	return nil
}

func (n *NEO) getRegisterPrice(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(n.getRegisterPriceInternal(ic.DAO)))
}

func (n *NEO) getRegisterPriceInternal(d *dao.Simple) int64 {
	cache := d.Store.GetROCache(n.ID).(*NeoCache)
	if !cache.registerPriceChanged {
		return cache.registerPrice
	}
	return getIntWithKey(n.ID, d, []byte{prefixRegisterPrice})
}

func (n *NEO) setRegisterPrice(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	price := toBigInt(args[0])
	if price.Sign() <= 0 || !price.IsInt64() {
		panic("invalid register price")
	}
	if !n.checkCommittee(ic) {
		panic("invalid committee signature")
	}

	setIntWithKey(n.ID, ic.DAO, []byte{prefixRegisterPrice}, price.Int64())
	cache := ic.DAO.Store.GetRWCache(n.ID).(*NeoCache)
	cache.registerPriceChanged = true
	return stackitem.Null{}
}

func (n *NEO) dropCandidateIfZero(d *dao.Simple, cache *NeoCache, pub *keys.PublicKey, c *candidate) (bool, error) {
	if c.Registered || c.Votes.Sign() != 0 {
		return false, nil
	}
	d.DeleteStorageItem(n.ID, makeValidatorKey(pub))

	voterKey := makeVoterKey(pub.Bytes())
	d.Seek(n.ID, storage.SeekRange{Prefix: voterKey}, func(k, v []byte) bool {
		d.DeleteStorageItem(n.ID, append(voterKey, k...)) // d.Seek cuts prefix, thus need to append it again.
		return true
	})
	delete(cache.gasPerVoteCache, string(voterKey))

	return true, nil
}

func makeVoterKey(pub []byte, prealloc ...[]byte) []byte {
	var key []byte
	if len(prealloc) != 0 {
		key = prealloc[0]
	} else {
		key = make([]byte, 34, 38)
	}
	key[0] = prefixVoterRewardPerCommittee
	copy(key[1:], pub)
	return key
}

// CalculateBonus calculates amount of gas generated for holding value NEO from start to end block
// and having voted for active committee member.
func (n *NEO) CalculateBonus(d *dao.Simple, acc util.Uint160, end uint32) (*big.Int, error) {
	key := makeAccountKey(acc)
	si := d.GetStorageItem(n.ID, key)
	if si == nil {
		return nil, storage.ErrKeyNotFound
	}
	st, err := state.NEOBalanceFromBytes(si)
	if err != nil {
		return nil, err
	}
	return n.calculateBonus(d, st.VoteTo, &st.Balance, st.BalanceHeight, end)
}

func (n *NEO) calculateBonus(d *dao.Simple, vote *keys.PublicKey, value *big.Int, start, end uint32) (*big.Int, error) {
	r, err := n.CalculateNEOHolderReward(d, value, start, end)
	if err != nil || vote == nil {
		return r, err
	}

	var key = makeVoterKey(vote.Bytes())
	var reward = n.getGASPerVote(d, key, []uint32{start, end})
	var tmp = (&reward[1]).Sub(&reward[1], &reward[0])
	tmp.Mul(tmp, value)
	tmp.Div(tmp, bigVoterRewardFactor)
	tmp.Add(tmp, r)
	return tmp, nil
}

// CalculateNEOHolderReward return GAS reward for holding `value` of NEO from start to end block.
func (n *NEO) CalculateNEOHolderReward(d *dao.Simple, value *big.Int, start, end uint32) (*big.Int, error) {
	if value.Sign() == 0 || start >= end {
		return big.NewInt(0), nil
	} else if value.Sign() < 0 {
		return nil, errors.New("negative value")
	}
	var gr gasRecord
	cache := d.Store.GetROCache(n.ID).(*NeoCache)
	if !cache.gasPerBlockChanged {
		gr = cache.gasPerBlock
	} else {
		gr = n.getSortedGASRecordFromDAO(d)
	}
	var sum, tmp big.Int
	for i := len(gr) - 1; i >= 0; i-- {
		if gr[i].Index >= end {
			continue
		} else if gr[i].Index <= start {
			tmp.SetInt64(int64(end - start))
			tmp.Mul(&tmp, &gr[i].GASPerBlock)
			sum.Add(&sum, &tmp)
			break
		}
		tmp.SetInt64(int64(end - gr[i].Index))
		tmp.Mul(&tmp, &gr[i].GASPerBlock)
		sum.Add(&sum, &tmp)
		end = gr[i].Index
	}
	res := new(big.Int).Mul(value, &sum)
	res.Mul(res, tmp.SetInt64(neoHolderRewardRatio))
	res.Div(res, tmp.SetInt64(100*NEOTotalSupply))
	return res, nil
}

func (n *NEO) registerCandidate(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	pub := toPublicKey(args[0])
	ok, err := runtime.CheckKeyedWitness(ic, pub)
	if err != nil {
		panic(err)
	} else if !ok {
		return stackitem.NewBool(false)
	}
	if !ic.VM.AddGas(n.getRegisterPriceInternal(ic.DAO)) {
		panic("insufficient gas")
	}
	err = n.RegisterCandidateInternal(ic, pub)
	return stackitem.NewBool(err == nil)
}

// RegisterCandidateInternal registers pub as a new candidate.
func (n *NEO) RegisterCandidateInternal(ic *interop.Context, pub *keys.PublicKey) error {
	key := makeValidatorKey(pub)
	si := ic.DAO.GetStorageItem(n.ID, key)
	var c *candidate
	if si == nil {
		c = &candidate{Registered: true}
	} else {
		c = new(candidate).FromBytes(si)
		c.Registered = true
	}
	return putConvertibleToDAO(n.ID, ic.DAO, key, c)
}

func (n *NEO) unregisterCandidate(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	pub := toPublicKey(args[0])
	ok, err := runtime.CheckKeyedWitness(ic, pub)
	if err != nil {
		panic(err)
	} else if !ok {
		return stackitem.NewBool(false)
	}
	err = n.UnregisterCandidateInternal(ic, pub)
	return stackitem.NewBool(err == nil)
}

// UnregisterCandidateInternal unregisters pub as a candidate.
func (n *NEO) UnregisterCandidateInternal(ic *interop.Context, pub *keys.PublicKey) error {
	key := makeValidatorKey(pub)
	si := ic.DAO.GetStorageItem(n.ID, key)
	if si == nil {
		return nil
	}
	cache := ic.DAO.Store.GetRWCache(n.ID).(*NeoCache)
	cache.validators = nil
	c := new(candidate).FromBytes(si)
	c.Registered = false
	ok, err := n.dropCandidateIfZero(ic.DAO, cache, pub, c)
	if ok {
		return err
	}
	return putConvertibleToDAO(n.ID, ic.DAO, key, c)
}

func (n *NEO) vote(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	acc := toUint160(args[0])
	var pub *keys.PublicKey
	if _, ok := args[1].(stackitem.Null); !ok {
		pub = toPublicKey(args[1])
	}
	err := n.VoteInternal(ic, acc, pub)
	return stackitem.NewBool(err == nil)
}

// VoteInternal votes from account h for validarors specified in pubs.
func (n *NEO) VoteInternal(ic *interop.Context, h util.Uint160, pub *keys.PublicKey) error {
	ok, err := runtime.CheckHashedWitness(ic, h)
	if err != nil {
		return err
	} else if !ok {
		return errors.New("invalid signature")
	}
	key := makeAccountKey(h)
	si := ic.DAO.GetStorageItem(n.ID, key)
	if si == nil {
		return errors.New("invalid account")
	}
	acc, err := state.NEOBalanceFromBytes(si)
	if err != nil {
		return err
	}
	// we should put it in storage anyway as it affects dumps
	ic.DAO.PutStorageItem(n.ID, key, si)
	if pub != nil {
		valKey := makeValidatorKey(pub)
		valSi := ic.DAO.GetStorageItem(n.ID, valKey)
		if valSi == nil {
			return errors.New("unknown validator")
		}
		cd := new(candidate).FromBytes(valSi)
		// we should put it in storage anyway as it affects dumps
		ic.DAO.PutStorageItem(n.ID, valKey, valSi)
		if !cd.Registered {
			return errors.New("validator must be registered")
		}
	}

	if (acc.VoteTo == nil) != (pub == nil) {
		val := &acc.Balance
		if pub == nil {
			val = new(big.Int).Neg(val)
		}
		if err := n.modifyVoterTurnout(ic.DAO, val); err != nil {
			return err
		}
	}
	if err := n.distributeGas(ic, h, acc); err != nil {
		return err
	}
	if err := n.ModifyAccountVotes(acc, ic.DAO, new(big.Int).Neg(&acc.Balance), false); err != nil {
		return err
	}
	acc.VoteTo = pub
	if err := n.ModifyAccountVotes(acc, ic.DAO, &acc.Balance, true); err != nil {
		return err
	}
	ic.DAO.PutStorageItem(n.ID, key, acc.Bytes())
	return nil
}

// ModifyAccountVotes modifies votes of the specified account by value (can be negative).
// typ specifies if this modify is occurring during transfer or vote (with old or new validator).
func (n *NEO) ModifyAccountVotes(acc *state.NEOBalance, d *dao.Simple, value *big.Int, isNewVote bool) error {
	cache := d.Store.GetRWCache(n.ID).(*NeoCache)
	cache.votesChanged = true
	if acc.VoteTo != nil {
		key := makeValidatorKey(acc.VoteTo)
		si := d.GetStorageItem(n.ID, key)
		if si == nil {
			return errors.New("invalid validator")
		}
		cd := new(candidate).FromBytes(si)
		cd.Votes.Add(&cd.Votes, value)
		if !isNewVote {
			ok, err := n.dropCandidateIfZero(d, cache, acc.VoteTo, cd)
			if ok {
				return err
			}
		}
		cache.validators = nil
		return putConvertibleToDAO(n.ID, d, key, cd)
	}
	return nil
}

func (n *NEO) getCandidates(d *dao.Simple, sortByKey bool) ([]keyWithVotes, error) {
	arr := make([]keyWithVotes, 0)
	buf := io.NewBufBinWriter()
	d.Seek(n.ID, storage.SeekRange{Prefix: []byte{prefixCandidate}}, func(k, v []byte) bool {
		c := new(candidate).FromBytes(v)
		emit.CheckSig(buf.BinWriter, k)
		if c.Registered && !n.Policy.IsBlockedInternal(d, hash.Hash160(buf.Bytes())) {
			arr = append(arr, keyWithVotes{Key: string(k), Votes: &c.Votes})
		}
		buf.Reset()
		return true
	})

	if !sortByKey {
		// sortByKey assumes to sort by serialized key bytes (that's the way keys
		// are stored and retrieved from the storage by default). Otherwise, need
		// to sort using big.Int comparator.
		sort.Slice(arr, func(i, j int) bool {
			// The most-voted validators should end up in the front of the list.
			cmp := arr[i].Votes.Cmp(arr[j].Votes)
			if cmp != 0 {
				return cmp > 0
			}
			// Ties are broken with deserialized public keys.
			// Sort by ECPoint's (X, Y) components: compare X first, and then compare Y.
			cmpX := strings.Compare(arr[i].Key[1:], arr[j].Key[1:])
			if cmpX != 0 {
				return cmpX == -1
			}
			// The case when X components are the same is extremely rare, thus we perform
			// key deserialization only if needed. No error can occur.
			ki, _ := keys.NewPublicKeyFromBytes([]byte(arr[i].Key), elliptic.P256())
			kj, _ := keys.NewPublicKeyFromBytes([]byte(arr[j].Key), elliptic.P256())
			return ki.Y.Cmp(kj.Y) == -1
		})
	}
	return arr, nil
}

// GetCandidates returns current registered validators list with keys
// and votes.
func (n *NEO) GetCandidates(d *dao.Simple) ([]state.Validator, error) {
	kvs, err := n.getCandidates(d, true)
	if err != nil {
		return nil, err
	}
	arr := make([]state.Validator, len(kvs))
	for i := range kvs {
		arr[i].Key, err = keys.NewPublicKeyFromBytes([]byte(kvs[i].Key), elliptic.P256())
		if err != nil {
			return nil, err
		}
		arr[i].Votes = kvs[i].Votes
	}
	return arr, nil
}

func (n *NEO) getCandidatesCall(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	validators, err := n.getCandidates(ic.DAO, true)
	if err != nil {
		panic(err)
	}
	arr := make([]stackitem.Item, len(validators))
	for i := range validators {
		arr[i] = stackitem.NewStruct([]stackitem.Item{
			stackitem.NewByteArray([]byte(validators[i].Key)),
			stackitem.NewBigInteger(validators[i].Votes),
		})
	}
	return stackitem.NewArray(arr)
}

func (n *NEO) getAccountState(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	key := makeAccountKey(toUint160(args[0]))
	si := ic.DAO.GetStorageItem(n.ID, key)
	if len(si) == 0 {
		return stackitem.Null{}
	}

	item, err := stackitem.Deserialize(si)
	if err != nil {
		panic(err) // no errors are expected but we better be sure
	}
	return item
}

// ComputeNextBlockValidators returns an actual list of current validators.
func (n *NEO) ComputeNextBlockValidators(bc interop.Ledger, d *dao.Simple) (keys.PublicKeys, error) {
	numOfCNs := n.cfg.GetNumOfCNs(bc.BlockHeight() + 1)
	cache := d.Store.GetRWCache(n.ID).(*NeoCache)
	if vals := cache.validators; vals != nil && numOfCNs == len(vals) {
		return vals.Copy(), nil
	}
	result, _, err := n.computeCommitteeMembers(bc, d)
	if err != nil {
		return nil, err
	}
	result = result[:numOfCNs]
	sort.Sort(result)
	cache.validators = result
	return result, nil
}

func (n *NEO) getCommittee(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	pubs := n.GetCommitteeMembers(ic.DAO)
	sort.Sort(pubs)
	return pubsToArray(pubs)
}

func (n *NEO) modifyVoterTurnout(d *dao.Simple, amount *big.Int) error {
	key := []byte{prefixVotersCount}
	si := d.GetStorageItem(n.ID, key)
	if si == nil {
		return errors.New("voters count not found")
	}
	votersCount := bigint.FromBytes(si)
	votersCount.Add(votersCount, amount)
	si = bigint.ToPreallocatedBytes(votersCount, si)
	d.PutStorageItem(n.ID, key, si)
	return nil
}

// GetCommitteeMembers returns public keys of nodes in committee using cached value.
func (n *NEO) GetCommitteeMembers(d *dao.Simple) keys.PublicKeys {
	cache := d.Store.GetROCache(n.ID).(*NeoCache)
	return getCommitteeMembers(cache)
}

func getCommitteeMembers(cache *NeoCache) keys.PublicKeys {
	var cvs = cache.committee
	var committee = make(keys.PublicKeys, len(cvs))
	var err error
	for i := range committee {
		committee[i], err = cvs[i].PublicKey()
		if err != nil {
			panic(err)
		}
	}
	return committee
}

func toKeysWithVotes(pubs keys.PublicKeys) keysWithVotes {
	ks := make(keysWithVotes, len(pubs))
	for i := range pubs {
		ks[i].UnmarshaledKey = pubs[i]
		ks[i].Key = string(pubs[i].Bytes())
		ks[i].Votes = big.NewInt(0)
	}
	return ks
}

// computeCommitteeMembers returns public keys of nodes in committee.
func (n *NEO) computeCommitteeMembers(bc interop.Ledger, d *dao.Simple) (keys.PublicKeys, keysWithVotes, error) {
	key := []byte{prefixVotersCount}
	si := d.GetStorageItem(n.ID, key)
	if si == nil {
		return nil, nil, errors.New("voters count not found")
	}
	votersCount := bigint.FromBytes(si)
	// votersCount / totalSupply must be >= 0.2
	votersCount.Mul(votersCount, bigEffectiveVoterTurnout)
	_, totalSupply := n.getTotalSupply(d)
	voterTurnout := votersCount.Div(votersCount, totalSupply)

	count := n.cfg.GetCommitteeSize(bc.BlockHeight() + 1)
	// Can be sorted and/or returned to outside users, thus needs to be copied.
	sbVals := keys.PublicKeys(n.standbyKeys[:count]).Copy()
	cs, err := n.getCandidates(d, false)
	if err != nil {
		return nil, nil, err
	}
	if voterTurnout.Sign() != 1 || len(cs) < count {
		kvs := make(keysWithVotes, count)
		for i := range kvs {
			kvs[i].UnmarshaledKey = sbVals[i]
			kvs[i].Key = string(sbVals[i].Bytes())
			votes := big.NewInt(0)
			for j := range cs {
				if cs[j].Key == kvs[i].Key {
					votes = cs[j].Votes
					break
				}
			}
			kvs[i].Votes = votes
		}
		return sbVals, kvs, nil
	}
	pubs := make(keys.PublicKeys, count)
	for i := range pubs {
		pubs[i], err = cs[i].PublicKey()
		if err != nil {
			return nil, nil, err
		}
	}
	return pubs, cs[:count], nil
}

func (n *NEO) getNextBlockValidators(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	result := n.GetNextBlockValidatorsInternal(ic.DAO)
	return pubsToArray(result)
}

// GetNextBlockValidatorsInternal returns next block validators.
func (n *NEO) GetNextBlockValidatorsInternal(d *dao.Simple) keys.PublicKeys {
	cache := d.Store.GetROCache(n.ID).(*NeoCache)
	return cache.nextValidators.Copy()
}

// BalanceOf returns native NEO token balance for the acc.
func (n *NEO) BalanceOf(d *dao.Simple, acc util.Uint160) (*big.Int, uint32) {
	key := makeAccountKey(acc)
	si := d.GetStorageItem(n.ID, key)
	if si == nil {
		return big.NewInt(0), 0
	}
	st, err := state.NEOBalanceFromBytes(si)
	if err != nil {
		panic(fmt.Errorf("failed to decode NEO balance state: %w", err))
	}
	return &st.Balance, st.BalanceHeight
}

func pubsToArray(pubs keys.PublicKeys) stackitem.Item {
	arr := make([]stackitem.Item, len(pubs))
	for i := range pubs {
		arr[i] = stackitem.NewByteArray(pubs[i].Bytes())
	}
	return stackitem.NewArray(arr)
}

func toPublicKey(s stackitem.Item) *keys.PublicKey {
	buf, err := s.TryBytes()
	if err != nil {
		panic(err)
	}
	pub := new(keys.PublicKey)
	if err := pub.DecodeBytes(buf); err != nil {
		panic(err)
	}
	return pub
}

// putGASRecord is a helper which creates key and puts GASPerBlock value into the storage.
func (n *NEO) putGASRecord(dao *dao.Simple, index uint32, value *big.Int) {
	key := make([]byte, 5)
	key[0] = prefixGASPerBlock
	binary.BigEndian.PutUint32(key[1:], index)
	dao.PutStorageItem(n.ID, key, bigint.ToBytes(value))
}
