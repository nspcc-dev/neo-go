package native

import (
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"net"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// NameService represents native NameService contract.
type NameService struct {
	nonfungible
	NEO *NEO
}

type nameState struct {
	state.NFTTokenState
	// Expiration is token expiration height.
	Expiration uint32
	// HasAdmin is true if token has admin.
	HasAdmin bool
	// Admin is token admin.
	Admin util.Uint160
}

// RecordType represents name record type.
type RecordType byte

// Pre-defined record types.
const (
	RecordTypeA     RecordType = 1
	RecordTypeCNAME RecordType = 5
	RecordTypeTXT   RecordType = 16
	RecordTypeAAAA  RecordType = 28
)

const (
	nameServiceID = -8

	prefixRoots       = 10
	prefixDomainPrice = 22
	prefixExpiration  = 20
	prefixRecord      = 12

	secondsInYear = 365 * 24 * 3600

	// DefaultDomainPrice is the default price of register method.
	DefaultDomainPrice = 10_00000000
	// MinDomainNameLength is minimum domain length.
	MinDomainNameLength = 3
	// MaxDomainNameLength is maximum domain length.
	MaxDomainNameLength = 255
)

var (
	// Lookahead is not supported by Go, but it is simple `(?=.{3,255}$)`,
	// so we check name length explicitly.
	nameRegex = regexp.MustCompile("^([a-z0-9]{1,62}\\.)+[a-z][a-z0-9]{0,15}$")
	ipv4Regex = regexp.MustCompile("^(?:(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9][0-9]|[0-9])\\.){3}(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9][0-9]|[0-9])$")
	ipv6Regex = regexp.MustCompile("(?:^)(([0-9a-f]{1,4}:){7,7}[0-9a-f]{1,4}|([0-9a-f]{1,4}:){1,7}:|([0-9a-f]{1,4}:){1,6}:[0-9a-f]{1,4}|([0-9a-f]{1,4}:){1,5}(:[0-9a-f]{1,4}){1,2}|([0-9a-f]{1,4}:){1,4}(:[0-9a-f]{1,4}){1,3}|([0-9a-f]{1,4}:){1,3}(:[0-9a-f]{1,4}){1,4}|([0-9a-f]{1,4}:){1,2}(:[0-9a-f]{1,4}){1,5}|[0-9a-f]{1,4}:((:[0-9a-f]{1,4}){1,6})|:((:[0-9a-f]{1,4}){1,7}|:))$")
	rootRegex = regexp.MustCompile("^[a-z][a-z0-9]{0,15}$")
)

// matchName checks if provided name is valid.
func matchName(name string) bool {
	ln := len(name)
	return MinDomainNameLength <= ln && ln <= MaxDomainNameLength &&
		nameRegex.Match([]byte(name))
}

func newNameService() *NameService {
	nf := newNonFungible(nativenames.NameService, nameServiceID, "NNS", 0)
	nf.getTokenKey = func(tokenID []byte) []byte {
		return append([]byte{prefixNFTToken}, hash.Hash160(tokenID).BytesBE()...)
	}
	nf.newTokenState = func() nftTokenState {
		return new(nameState)
	}
	nf.onTransferred = func(tok nftTokenState) {
		tok.(*nameState).HasAdmin = false
	}

	n := &NameService{nonfungible: *nf}
	defer n.UpdateHash()

	desc := newDescriptor("addRoot", smartcontract.VoidType,
		manifest.NewParameter("root", smartcontract.StringType))
	md := newMethodAndPrice(n.addRoot, 3000000, callflag.States)
	n.AddMethod(md, desc)

	desc = newDescriptor("setPrice", smartcontract.VoidType,
		manifest.NewParameter("price", smartcontract.IntegerType))
	md = newMethodAndPrice(n.setPrice, 3000000, callflag.States)
	n.AddMethod(md, desc)

	desc = newDescriptor("getPrice", smartcontract.IntegerType)
	md = newMethodAndPrice(n.getPrice, 1000000, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("isAvailable", smartcontract.BoolType,
		manifest.NewParameter("name", smartcontract.StringType))
	md = newMethodAndPrice(n.isAvailable, 1000000, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("register", smartcontract.BoolType,
		manifest.NewParameter("name", smartcontract.StringType),
		manifest.NewParameter("owner", smartcontract.Hash160Type))
	md = newMethodAndPrice(n.register, 1000000, callflag.States)
	n.AddMethod(md, desc)

	desc = newDescriptor("renew", smartcontract.IntegerType,
		manifest.NewParameter("name", smartcontract.StringType))
	md = newMethodAndPrice(n.renew, 0, callflag.States)
	n.AddMethod(md, desc)

	desc = newDescriptor("setAdmin", smartcontract.VoidType,
		manifest.NewParameter("name", smartcontract.StringType),
		manifest.NewParameter("admin", smartcontract.Hash160Type))
	md = newMethodAndPrice(n.setAdmin, 3000000, callflag.States)
	n.AddMethod(md, desc)

	desc = newDescriptor("setRecord", smartcontract.VoidType,
		manifest.NewParameter("name", smartcontract.StringType),
		manifest.NewParameter("type", smartcontract.IntegerType),
		manifest.NewParameter("data", smartcontract.StringType))
	md = newMethodAndPrice(n.setRecord, 30000000, callflag.States)
	n.AddMethod(md, desc)

	desc = newDescriptor("getRecord", smartcontract.StringType,
		manifest.NewParameter("name", smartcontract.StringType),
		manifest.NewParameter("type", smartcontract.IntegerType))
	md = newMethodAndPrice(n.getRecord, 1000000, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("deleteRecord", smartcontract.VoidType,
		manifest.NewParameter("name", smartcontract.StringType),
		manifest.NewParameter("type", smartcontract.IntegerType))
	md = newMethodAndPrice(n.deleteRecord, 1000000, callflag.States)
	n.AddMethod(md, desc)

	desc = newDescriptor("resolve", smartcontract.StringType,
		manifest.NewParameter("name", smartcontract.StringType),
		manifest.NewParameter("type", smartcontract.IntegerType))
	md = newMethodAndPrice(n.resolve, 3000000, callflag.ReadStates)
	n.AddMethod(md, desc)

	return n
}

// Metadata implements interop.Contract interface.
func (n *NameService) Metadata() *interop.ContractMD {
	return &n.ContractMD
}

// Initialize implements interop.Contract interface.
func (n *NameService) Initialize(ic *interop.Context) error {
	if err := n.nonfungible.Initialize(ic); err != nil {
		return err
	}
	if err := setIntWithKey(n.ID, ic.DAO, []byte{prefixDomainPrice}, DefaultDomainPrice); err != nil {
		return err
	}
	roots := stringList{}
	return putSerializableToDAO(n.ID, ic.DAO, []byte{prefixRoots}, &roots)
}

// OnPersist implements interop.Contract interface.
func (n *NameService) OnPersist(ic *interop.Context) error {
	now := uint32(ic.Block.Timestamp/1000 + 1)
	keys := []string{}
	ic.DAO.Seek(n.ID, []byte{prefixExpiration}, func(k, v []byte) {
		if binary.BigEndian.Uint32(k) >= now {
			return
		}
		// Removal is done separately because of `Seek` takes storage mutex.
		keys = append(keys, string(k))
	})

	var keysToRemove [][]byte
	key := []byte{prefixExpiration}
	keyRecord := []byte{prefixRecord}
	for i := range keys {
		key[0] = prefixExpiration
		key = append(key[:1], []byte(keys[i])...)
		if err := ic.DAO.DeleteStorageItem(n.ID, key); err != nil {
			return err
		}

		keysToRemove = keysToRemove[:0]
		key[0] = prefixRecord
		key = append(key[:1], keys[i][4:]...)
		ic.DAO.Seek(n.ID, key, func(k, v []byte) {
			keysToRemove = append(keysToRemove, k)
		})
		for i := range keysToRemove {
			keyRecord = append(keyRecord[:0], key...)
			keyRecord = append(keyRecord, keysToRemove[i]...)
			err := ic.DAO.DeleteStorageItem(n.ID, keyRecord)
			if err != nil {
				return err
			}
		}

		key[0] = prefixNFTToken
		n.burnByKey(ic, key)
	}
	return nil
}

// PostPersist implements interop.Contract interface.
func (n *NameService) PostPersist(ic *interop.Context) error {
	return nil
}

func (n *NameService) addRoot(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	root := toString(args[0])
	if !rootRegex.Match([]byte(root)) {
		panic("invalid root")
	}

	n.checkCommittee(ic)
	roots, _ := n.getRootsInternal(ic.DAO)
	if !roots.add(root) {
		panic("name already exists")
	}

	err := putSerializableToDAO(n.ID, ic.DAO, []byte{prefixRoots}, &roots)
	if err != nil {
		panic(err)
	}
	return stackitem.Null{}
}

var maxPrice = big.NewInt(10000_00000000)

func (n *NameService) setPrice(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	price := toBigInt(args[0])
	if price.Sign() <= 0 || price.Cmp(maxPrice) >= 0 {
		panic("invalid price")
	}

	n.checkCommittee(ic)
	si := &state.StorageItem{Value: bigint.ToBytes(price)}
	err := ic.DAO.PutStorageItem(n.ID, []byte{prefixDomainPrice}, si)
	if err != nil {
		panic(err)
	}
	return stackitem.Null{}
}

func (n *NameService) getPrice(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(n.getPriceInternal(ic.DAO))
}

func (n *NameService) getPriceInternal(d dao.DAO) *big.Int {
	si := d.GetStorageItem(n.ID, []byte{prefixDomainPrice})
	return bigint.FromBytes(si.Value)
}

func (n *NameService) parseName(item stackitem.Item) (string, []string, []byte) {
	name := toName(item)
	names := strings.Split(name, ".")
	if len(names) != 2 {
		panic("invalid name")
	}
	return name, names, n.getTokenKey([]byte(name))
}

func (n *NameService) isAvailable(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	_, names, key := n.parseName(args[0])
	if ic.DAO.GetStorageItem(n.ID, key) != nil {
		return stackitem.NewBool(false)
	}

	roots, _ := n.getRootsInternal(ic.DAO)
	_, ok := roots.index(names[1])
	if !ok {
		panic("domain is not registered")
	}
	return stackitem.NewBool(true)
}

func (n *NameService) getRootsInternal(d dao.DAO) (stringList, bool) {
	var sl stringList
	err := getSerializableFromDAO(n.ID, d, []byte{prefixRoots}, &sl)
	if err != nil {
		// Roots are being stored in `Initialize()` and thus must always be present.
		panic(err)
	}
	return sl, true
}

func (n *NameService) register(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	name, names, key := n.parseName(args[0])
	owner := toUint160(args[1])
	if !n.checkWitness(ic, owner) {
		panic("owner is not witnessed")
	}

	if ic.DAO.GetStorageItem(n.ID, key) != nil {
		return stackitem.NewBool(false)
	}

	roots, _ := n.getRootsInternal(ic.DAO)
	if _, ok := roots.index(names[1]); !ok {
		panic("missing root")
	}
	if !ic.VM.AddGas(n.getPriceInternal(ic.DAO).Int64()) {
		panic("insufficient gas")
	}
	token := &nameState{
		NFTTokenState: state.NFTTokenState{
			Owner: owner,
			Name:  name,
		},
		Expiration: uint32(ic.Block.Timestamp/1000 + secondsInYear),
	}
	n.mint(ic, token)
	err := ic.DAO.PutStorageItem(n.ID,
		makeExpirationKey(token.Expiration, token.ID()),
		&state.StorageItem{Value: []byte{0}})
	if err != nil {
		panic(err)
	}
	return stackitem.NewBool(true)
}

func (n *NameService) renew(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	_, _, key := n.parseName(args[0])
	if !ic.VM.AddGas(n.getPriceInternal(ic.DAO).Int64()) {
		panic("insufficient gas")
	}
	token := new(nameState)
	err := getSerializableFromDAO(n.ID, ic.DAO, key, token)
	if err != nil {
		panic(err)
	}

	keyExpiration := makeExpirationKey(token.Expiration, token.ID())
	if err := ic.DAO.DeleteStorageItem(n.ID, keyExpiration); err != nil {
		panic(err)
	}

	token.Expiration += secondsInYear
	err = putSerializableToDAO(n.ID, ic.DAO, key, token)
	if err != nil {
		panic(err)
	}

	binary.BigEndian.PutUint32(key[1:], token.Expiration)
	si := &state.StorageItem{Value: []byte{0}}
	err = ic.DAO.PutStorageItem(n.ID, key, si)
	if err != nil {
		panic(err)
	}
	bi := new(big.Int).SetUint64(uint64(token.Expiration))
	return stackitem.NewBigInteger(bi)
}

func (n *NameService) setAdmin(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	_, _, key := n.parseName(args[0])

	var admin util.Uint160
	_, isNull := args[1].(stackitem.Null)
	if !isNull {
		admin = toUint160(args[1])
		if !n.checkWitness(ic, admin) {
			panic("not witnessed by admin")
		}
	}

	token := new(nameState)
	err := getSerializableFromDAO(n.ID, ic.DAO, key, token)
	if err != nil {
		panic(err)
	}
	if !n.checkWitness(ic, token.Owner) {
		panic("only owner can set admin")
	}
	token.HasAdmin = !isNull
	token.Admin = admin
	err = putSerializableToDAO(n.ID, ic.DAO, key, token)
	if err != nil {
		panic(err)
	}
	return stackitem.Null{}
}

func (n *NameService) checkWitness(ic *interop.Context, owner util.Uint160) bool {
	ok, err := runtime.CheckHashedWitness(ic, owner)
	if err != nil {
		panic(err)
	}
	return ok
}

func (n *NameService) checkCommittee(ic *interop.Context) {
	if !n.NEO.checkCommittee(ic) {
		panic("not witnessed by committee")
	}
}

func (n *NameService) checkAdmin(ic *interop.Context, token *nameState) bool {
	if n.checkWitness(ic, token.Owner) {
		return true
	}
	return token.HasAdmin && n.checkWitness(ic, token.Admin)
}

func (n *NameService) setRecord(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	name := toName(args[0])
	rt := toRecordType(args[1])
	data := toString(args[2])
	checkName(rt, data)

	domain := toDomain(name)
	token, _, err := n.tokenState(ic.DAO, []byte(domain))
	if err != nil {
		panic(err)
	}
	if !n.checkAdmin(ic, token.(*nameState)) {
		panic("not witnessed by admin")
	}
	key := makeRecordKey(domain, name, rt)
	si := &state.StorageItem{Value: []byte(data)}
	if err := ic.DAO.PutStorageItem(n.ID, key, si); err != nil {
		panic(err)
	}
	return stackitem.Null{}
}

func checkName(rt RecordType, name string) {
	var valid bool
	switch rt {
	case RecordTypeA:
		// We can't rely on `len(ip) == net.IPv4len` because
		// IPv4 can be parsed to mapped representation.
		valid = ipv4Regex.MatchString(name) &&
			net.ParseIP(name) != nil
	case RecordTypeCNAME:
		valid = matchName(name)
	case RecordTypeTXT:
		valid = utf8.RuneCountInString(name) <= 255
	case RecordTypeAAAA:
		valid = ipv6Regex.MatchString(name) &&
			net.ParseIP(name) != nil
	}
	if !valid {
		panic("invalid name")
	}
}

func (n *NameService) getRecord(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	name := toName(args[0])
	domain := toDomain(name)
	rt := toRecordType(args[1])
	key := makeRecordKey(domain, name, rt)
	si := ic.DAO.GetStorageItem(n.ID, key)
	if si == nil {
		return stackitem.Null{}
	}
	return stackitem.NewByteArray(si.Value)
}

func (n *NameService) deleteRecord(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	name := toName(args[0])
	rt := toRecordType(args[1])
	domain := toDomain(name)
	key := n.getTokenKey([]byte(domain))
	token := new(nameState)
	err := getSerializableFromDAO(n.ID, ic.DAO, key, token)
	if err != nil {
		panic(err)
	}

	if !n.checkAdmin(ic, token) {
		panic("not witnessed by admin")
	}

	key = makeRecordKey(domain, name, rt)
	if err := ic.DAO.DeleteStorageItem(n.ID, key); err != nil {
		panic(err)
	}
	return stackitem.Null{}
}

func (n *NameService) resolve(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	name := toString(args[0])
	rt := toRecordType(args[1])
	result, ok := n.resolveInternal(ic, name, rt, 2)
	if !ok {
		return stackitem.Null{}
	}
	return stackitem.NewByteArray([]byte(result))
}

func (n *NameService) resolveInternal(ic *interop.Context, name string, t RecordType, redirect int) (string, bool) {
	if redirect < 0 {
		panic("invalid redirect")
	}
	records := n.getRecordsInternal(ic.DAO, name)
	if data, ok := records[t]; ok {
		return data, true
	}
	data, ok := records[RecordTypeCNAME]
	if !ok {
		return "", false
	}
	return n.resolveInternal(ic, data, t, redirect-1)
}

func (n *NameService) getRecordsInternal(d dao.DAO, name string) map[RecordType]string {
	domain := toDomain(name)
	key := makeRecordKey(domain, name, 0)
	key = key[:len(key)-1]
	res := make(map[RecordType]string)
	d.Seek(n.ID, key, func(k, v []byte) {
		rt := RecordType(k[len(k)-1])
		var si state.StorageItem
		r := io.NewBinReaderFromBuf(v)
		si.DecodeBinary(r)
		if r.Err != nil {
			panic(r.Err)
		}
		res[rt] = string(si.Value)
	})
	return res
}

func makeRecordKey(domain, name string, rt RecordType) []byte {
	key := make([]byte, 1+util.Uint160Size+util.Uint160Size+1)
	key[0] = prefixRecord
	i := 1
	i += copy(key[i:], hash.Hash160([]byte(domain)).BytesBE())
	i += copy(key[i:], hash.Hash160([]byte(name)).BytesBE())
	key[i] = byte(rt)
	return key
}

func makeExpirationKey(expiration uint32, tokenID []byte) []byte {
	key := make([]byte, 1+4+util.Uint160Size)
	key[0] = prefixExpiration
	binary.BigEndian.PutUint32(key[1:], expiration)
	copy(key[5:], hash.Hash160(tokenID).BytesBE())
	return key
}

// ToMap implements nftTokenState interface.
func (s *nameState) ToMap() *stackitem.Map {
	m := s.NFTTokenState.ToMap()
	m.Add(stackitem.NewByteArray([]byte("expiration")),
		stackitem.NewBigInteger(new(big.Int).SetUint64(uint64(s.Expiration))))
	return m
}

// EncodeBinary implements io.Serializable.
func (s *nameState) EncodeBinary(w *io.BinWriter) {
	stackitem.EncodeBinaryStackItem(s.ToStackItem(), w)
}

// DecodeBinary implements io.Serializable.
func (s *nameState) DecodeBinary(r *io.BinReader) {
	item := stackitem.DecodeBinaryStackItem(r)
	if r.Err == nil {
		s.FromStackItem(item)
	}
}

// ToStackItem implements nftTokenState interface.
func (s *nameState) ToStackItem() stackitem.Item {
	item := s.NFTTokenState.ToStackItem().(*stackitem.Struct)
	exp := new(big.Int).SetUint64(uint64(s.Expiration))
	item.Append(stackitem.NewBigInteger(exp))
	if s.HasAdmin {
		item.Append(stackitem.NewByteArray(s.Admin.BytesBE()))
	} else {
		item.Append(stackitem.Null{})
	}
	return item
}

// FromStackItem implements nftTokenState interface.
func (s *nameState) FromStackItem(item stackitem.Item) error {
	err := s.NFTTokenState.FromStackItem(item)
	if err != nil {
		return err
	}
	elems := item.Value().([]stackitem.Item)
	if len(elems) < 4 {
		return errors.New("invalid stack item")
	}
	bi, err := elems[2].TryInteger()
	if err != nil || !bi.IsUint64() {
		return errors.New("invalid stack item")
	}

	_, isNull := elems[3].(stackitem.Null)
	if !isNull {
		bs, err := elems[3].TryBytes()
		if err != nil {
			return err
		}
		u, err := util.Uint160DecodeBytesBE(bs)
		if err != nil {
			return err
		}
		s.Admin = u
	}
	s.Expiration = uint32(bi.Uint64())
	s.HasAdmin = !isNull
	return nil
}

// Helpers

func domainFromString(name string) (string, bool) {
	i := strings.LastIndexAny(name, ".")
	if i < 0 {
		return "", false
	}
	i = strings.LastIndexAny(name[:i], ".")
	if i < 0 {
		return name, true
	}
	return name[i+1:], true

}

func toDomain(name string) string {
	domain, ok := domainFromString(name)
	if !ok {
		panic("invalid record")
	}
	return domain
}

func toRecordType(item stackitem.Item) RecordType {
	bi, err := item.TryInteger()
	if err != nil || !bi.IsInt64() {
		panic("invalid record type")
	}
	val := bi.Uint64()
	if val > math.MaxUint8 {
		panic("invalid record type")
	}
	switch rt := RecordType(val); rt {
	case RecordTypeA, RecordTypeCNAME, RecordTypeTXT, RecordTypeAAAA:
		return rt
	default:
		panic("invalid record type")
	}
}

func toName(item stackitem.Item) string {
	name := toString(item)
	if !matchName(name) {
		panic("invalid name")
	}
	return name
}

type stringList []string

// ToStackItem converts sl to stack item.
func (sl stringList) ToStackItem() stackitem.Item {
	arr := make([]stackitem.Item, len(sl))
	for i := range sl {
		arr[i] = stackitem.NewByteArray([]byte(sl[i]))
	}
	return stackitem.NewArray(arr)
}

// FromStackItem converts stack item to string list.
func (sl *stringList) FromStackItem(item stackitem.Item) error {
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		return errors.New("invalid stack item")
	}
	res := make([]string, len(arr))
	for i := range res {
		s, err := stackitem.ToString(arr[i])
		if err != nil {
			return err
		}
		res[i] = s
	}
	*sl = res
	return nil
}

// EncodeBinary implements io.Serializable.
func (sl stringList) EncodeBinary(w *io.BinWriter) {
	stackitem.EncodeBinaryStackItem(sl.ToStackItem(), w)
}

// DecodeBinary implements io.Serializable.
func (sl *stringList) DecodeBinary(r *io.BinReader) {
	item := stackitem.DecodeBinaryStackItem(r)
	if r.Err == nil {
		sl.FromStackItem(item)
	}
}

func (sl stringList) index(s string) (int, bool) {
	index := sort.Search(len(sl), func(i int) bool {
		return sl[i] >= s
	})
	return index, index < len(sl) && sl[index] == s
}

func (sl *stringList) remove(s string) bool {
	index, has := sl.index(s)
	if !has {
		return false
	}

	copy((*sl)[index:], (*sl)[index+1:])
	*sl = (*sl)[:len(*sl)-1]
	return true
}

func (sl *stringList) add(s string) bool {
	index, has := sl.index(s)
	if has {
		return false
	}

	*sl = append(*sl, "")
	copy((*sl)[index+1:], (*sl)[index:])
	(*sl)[index] = s
	return true
}
