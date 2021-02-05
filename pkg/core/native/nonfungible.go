package native

import (
	"bytes"
	"errors"
	"math/big"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	istorage "github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

type nonfungible struct {
	interop.ContractMD

	tokenSymbol   string
	tokenDecimals byte

	onTransferred func(nftTokenState)
	getTokenKey   func([]byte) []byte
	newTokenState func() nftTokenState
}

type nftTokenState interface {
	io.Serializable
	ToStackItem() stackitem.Item
	FromStackItem(stackitem.Item) error
	ToMap() *stackitem.Map
	ID() []byte
	Base() *state.NFTTokenState
}

const (
	prefixNFTTotalSupply = 11
	prefixNFTAccount     = 7
	prefixNFTToken       = 5
)

var (
	nftTotalSupplyKey = []byte{prefixNFTTotalSupply}
)

func newNonFungible(name string, id int32, symbol string, decimals byte) *nonfungible {
	n := &nonfungible{
		ContractMD: *interop.NewContractMD(name, id),

		tokenSymbol:   symbol,
		tokenDecimals: decimals,

		getTokenKey: func(tokenID []byte) []byte {
			return append([]byte{prefixNFTToken}, tokenID...)
		},
		newTokenState: func() nftTokenState {
			return new(state.NFTTokenState)
		},
	}

	desc := newDescriptor("symbol", smartcontract.StringType)
	md := newMethodAndPrice(n.symbol, 0, callflag.NoneFlag)
	n.AddMethod(md, desc)

	desc = newDescriptor("decimals", smartcontract.IntegerType)
	md = newMethodAndPrice(n.decimals, 0, callflag.NoneFlag)
	n.AddMethod(md, desc)

	desc = newDescriptor("totalSupply", smartcontract.IntegerType)
	md = newMethodAndPrice(n.totalSupply, 1000000, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("ownerOf", smartcontract.Hash160Type,
		manifest.NewParameter("tokenId", smartcontract.ByteArrayType))
	md = newMethodAndPrice(n.OwnerOf, 1000000, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("balanceOf", smartcontract.IntegerType,
		manifest.NewParameter("owner", smartcontract.Hash160Type))
	md = newMethodAndPrice(n.BalanceOf, 1000000, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("properties", smartcontract.MapType,
		manifest.NewParameter("tokenId", smartcontract.ByteArrayType))
	md = newMethodAndPrice(n.Properties, 1000000, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("tokens", smartcontract.InteropInterfaceType)
	md = newMethodAndPrice(n.tokens, 1000000, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("tokensOf", smartcontract.InteropInterfaceType,
		manifest.NewParameter("owner", smartcontract.Hash160Type))
	md = newMethodAndPrice(n.tokensOf, 1000000, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("transfer", smartcontract.BoolType,
		manifest.NewParameter("to", smartcontract.Hash160Type),
		manifest.NewParameter("tokenId", smartcontract.ByteArrayType))
	md = newMethodAndPrice(n.transfer, 9000000, callflag.WriteStates|callflag.AllowNotify)
	n.AddMethod(md, desc)

	n.AddEvent("Transfer",
		manifest.NewParameter("from", smartcontract.Hash160Type),
		manifest.NewParameter("to", smartcontract.Hash160Type),
		manifest.NewParameter("amount", smartcontract.IntegerType),
		manifest.NewParameter("tokenId", smartcontract.ByteArrayType))

	return n
}

// Initialize implements interop.Contract interface.
func (n nonfungible) Initialize(ic *interop.Context) error {
	return setIntWithKey(n.ContractID, ic.DAO, nftTotalSupplyKey, 0)
}

func (n *nonfungible) symbol(_ *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewByteArray([]byte(n.tokenSymbol))
}

func (n *nonfungible) decimals(_ *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(int64(n.tokenDecimals)))
}

func (n *nonfungible) totalSupply(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(n.TotalSupply(ic.DAO))
}

func (n *nonfungible) TotalSupply(d dao.DAO) *big.Int {
	si := d.GetStorageItem(n.ContractID, nftTotalSupplyKey)
	if si == nil {
		panic(errors.New("total supply is not initialized"))
	}
	return bigint.FromBytes(si.Value)
}

func (n *nonfungible) setTotalSupply(d dao.DAO, ts *big.Int) {
	si := &state.StorageItem{Value: bigint.ToBytes(ts)}
	err := d.PutStorageItem(n.ContractID, nftTotalSupplyKey, si)
	if err != nil {
		panic(err)
	}
}

func (n *nonfungible) tokenState(d dao.DAO, tokenID []byte) (nftTokenState, []byte, error) {
	key := n.getTokenKey(tokenID)
	s := n.newTokenState()
	err := getSerializableFromDAO(n.ContractID, d, key, s)
	return s, key, err
}

func (n *nonfungible) accountState(d dao.DAO, owner util.Uint160) (*state.NFTAccountState, []byte, error) {
	acc := new(state.NFTAccountState)
	keyAcc := makeNFTAccountKey(owner)
	err := getSerializableFromDAO(n.ContractID, d, keyAcc, acc)
	return acc, keyAcc, err
}

func (n *nonfungible) putAccountState(d dao.DAO, key []byte, acc *state.NFTAccountState) {
	var err error
	if acc.Balance.Sign() == 0 {
		err = d.DeleteStorageItem(n.ContractID, key)
	} else {
		err = putSerializableToDAO(n.ContractID, d, key, acc)
	}
	if err != nil {
		panic(err)
	}
}

func (n *nonfungible) OwnerOf(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	tokenID, err := args[0].TryBytes()
	if err != nil {
		panic(err)
	}
	s, _, err := n.tokenState(ic.DAO, tokenID)
	if err != nil {
		panic(err)
	}
	return stackitem.NewByteArray(s.Base().Owner.BytesBE())
}

func (n *nonfungible) Properties(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	tokenID, err := args[0].TryBytes()
	if err != nil {
		panic(err)
	}
	s, _, err := n.tokenState(ic.DAO, tokenID)
	if err != nil {
		panic(err)
	}
	return s.ToMap()
}

func (n *nonfungible) BalanceOf(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	owner := toUint160(args[0])
	s, _, err := n.accountState(ic.DAO, owner)
	if err != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			return stackitem.NewBigInteger(big.NewInt(0))
		}
		panic(err)
	}
	return stackitem.NewBigInteger(&s.Balance)
}

func (n *nonfungible) tokens(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	prefix := []byte{prefixNFTToken}
	siMap, err := ic.DAO.GetStorageItemsWithPrefix(n.ContractID, prefix)
	if err != nil {
		panic(err)
	}
	filteredMap := stackitem.NewMap()
	for k, v := range siMap {
		filteredMap.Add(stackitem.NewByteArray(append(prefix, []byte(k)...)), stackitem.NewByteArray(v.Value))
	}
	sort.Slice(filteredMap.Value().([]stackitem.MapElement), func(i, j int) bool {
		return bytes.Compare(filteredMap.Value().([]stackitem.MapElement)[i].Key.Value().([]byte),
			filteredMap.Value().([]stackitem.MapElement)[j].Key.Value().([]byte)) == -1
	})
	iter := istorage.NewIterator(filteredMap, istorage.FindValuesOnly|istorage.FindDeserialize|istorage.FindPick1)
	return stackitem.NewInterop(iter)
}

func (n *nonfungible) tokensOf(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	owner := toUint160(args[0])
	s, _, err := n.accountState(ic.DAO, owner)
	if err != nil {
		panic(err)
	}
	arr := make([]stackitem.Item, len(s.Tokens))
	for i := range arr {
		arr[i] = stackitem.NewByteArray(s.Tokens[i])
	}
	iter, _ := vm.NewIterator(stackitem.NewArray(arr))
	return iter
}

func (n *nonfungible) mint(ic *interop.Context, s nftTokenState) {
	key := n.getTokenKey(s.ID())
	if ic.DAO.GetStorageItem(n.ContractID, key) != nil {
		panic("token is already minted")
	}
	if err := putSerializableToDAO(n.ContractID, ic.DAO, key, s); err != nil {
		panic(err)
	}

	owner := s.Base().Owner
	acc, keyAcc, err := n.accountState(ic.DAO, owner)
	if err != nil && !errors.Is(err, storage.ErrKeyNotFound) {
		panic(err)
	}
	acc.Add(s.ID())
	n.putAccountState(ic.DAO, keyAcc, acc)

	ts := n.TotalSupply(ic.DAO)
	ts.Add(ts, intOne)
	n.setTotalSupply(ic.DAO, ts)
	n.postTransfer(ic, nil, &owner, s.ID())
}

func (n *nonfungible) postTransfer(ic *interop.Context, from, to *util.Uint160, tokenID []byte) {
	ne := state.NotificationEvent{
		ScriptHash: n.Hash,
		Name:       "Transfer",
		Item: stackitem.NewArray([]stackitem.Item{
			addrToStackItem(from),
			addrToStackItem(to),
			stackitem.NewBigInteger(intOne),
			stackitem.NewByteArray(tokenID),
		}),
	}
	ic.Notifications = append(ic.Notifications, ne)
	if to == nil {
		return
	}
	cs, err := ic.GetContract(*to)
	if err != nil {
		return
	}

	fromArg := stackitem.Item(stackitem.Null{})
	if from != nil {
		fromArg = stackitem.NewByteArray((*from).BytesBE())
	}
	args := []stackitem.Item{
		fromArg,
		stackitem.NewBigInteger(intOne),
		stackitem.NewByteArray(tokenID),
	}
	if err := contract.CallFromNative(ic, n.Hash, cs, manifest.MethodOnNEP11Payment, args, false); err != nil {
		panic(err)
	}
}

func (n *nonfungible) burn(ic *interop.Context, tokenID []byte) {
	key := n.getTokenKey(tokenID)
	n.burnByKey(ic, key)
}

func (n *nonfungible) burnByKey(ic *interop.Context, key []byte) {
	token := n.newTokenState()
	err := getSerializableFromDAO(n.ContractID, ic.DAO, key, token)
	if err != nil {
		panic(err)
	}
	if err := ic.DAO.DeleteStorageItem(n.ContractID, key); err != nil {
		panic(err)
	}

	owner := token.Base().Owner
	acc, keyAcc, err := n.accountState(ic.DAO, owner)
	if err != nil {
		panic(err)
	}

	id := token.ID()
	acc.Remove(id)
	n.putAccountState(ic.DAO, keyAcc, acc)

	ts := n.TotalSupply(ic.DAO)
	ts.Sub(ts, intOne)
	n.setTotalSupply(ic.DAO, ts)
	n.postTransfer(ic, &owner, nil, id)
}

func (n *nonfungible) transfer(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	to := toUint160(args[0])
	tokenID, err := args[1].TryBytes()
	if err != nil {
		panic(err)
	}

	token, tokenKey, err := n.tokenState(ic.DAO, tokenID)
	if err != nil {
		panic(err)
	}

	from := token.Base().Owner
	ok, err := runtime.CheckHashedWitness(ic, from)
	if err != nil || !ok {
		return stackitem.NewBool(false)
	}
	if from != to {
		acc, key, err := n.accountState(ic.DAO, from)
		if err != nil {
			panic(err)
		}
		acc.Remove(tokenID)
		n.putAccountState(ic.DAO, key, acc)

		token.Base().Owner = to
		n.onTransferred(token)
		err = putSerializableToDAO(n.ContractID, ic.DAO, tokenKey, token)
		if err != nil {
			panic(err)
		}
		acc, key, err = n.accountState(ic.DAO, to)
		if err != nil && !errors.Is(err, storage.ErrKeyNotFound) {
			panic(err)
		}
		acc.Add(tokenID)
		n.putAccountState(ic.DAO, key, acc)
	}
	n.postTransfer(ic, &from, &to, tokenID)
	return stackitem.NewBool(true)
}

func makeNFTAccountKey(owner util.Uint160) []byte {
	return append([]byte{prefixNFTAccount}, owner.BytesBE()...)
}
