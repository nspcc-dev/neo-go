package native

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"sync/atomic"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativeids"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Oracle represents Oracle native contract.
type Oracle struct {
	interop.ContractMD
	GAS   IGAS
	NEO   INEO
	Desig IDesignate

	oracleScript []byte

	// Module is an oracle module capable of talking with the external world.
	Module atomic.Value
	// newRequests contains new requests created during the current block.
	newRequests map[uint64]*state.OracleRequest
}

type OracleCache struct {
	requestPrice int64
}

// OracleService specifies oracle module interface.
type OracleService interface {
	// AddRequests processes new requests.
	AddRequests(map[uint64]*state.OracleRequest)
	// RemoveRequests removes already processed requests.
	RemoveRequests([]uint64)
	// UpdateOracleNodes updates oracle nodes.
	UpdateOracleNodes(keys.PublicKeys)
	// UpdateNativeContract updates oracle contract native script and hash.
	UpdateNativeContract([]byte, []byte, util.Uint160, int)
	// Start runs oracle module.
	Start()
	// Shutdown shutdowns oracle module.
	Shutdown()
}

const (
	maxURLLength      = 256
	maxFilterLength   = 128
	maxCallbackLength = 32
	maxUserDataLength = 512
	// maxRequestsCount is the maximum number of requests per URL.
	maxRequestsCount = 256

	// DefaultOracleRequestPrice is the default amount GAS needed for an oracle request.
	DefaultOracleRequestPrice = 5000_0000

	// MinimumResponseGas is the minimum response fee permitted for a request.
	MinimumResponseGas = 10_000_000
)

var (
	prefixRequestPrice = []byte{5}
	prefixIDList       = []byte{6}
	prefixRequest      = []byte{7}
	prefixRequestID    = []byte{9}
)

// Various validation errors.
var (
	ErrBigArgument      = errors.New("some of the arguments are invalid")
	ErrInvalidWitness   = errors.New("witness check failed")
	ErrLowResponseGas   = errors.New("not enough gas for response")
	ErrNotEnoughGas     = errors.New("gas limit exceeded")
	ErrRequestNotFound  = errors.New("oracle request not found")
	ErrResponseNotFound = errors.New("oracle response not found")
)

var (
	_ interop.Contract        = (*Oracle)(nil)
	_ dao.NativeContractCache = (*OracleCache)(nil)
)

// Copy implements NativeContractCache interface.
func (c *OracleCache) Copy() dao.NativeContractCache {
	cp := &OracleCache{}
	copyOracleCache(c, cp)
	return cp
}

func copyOracleCache(src, dst *OracleCache) {
	*dst = *src
}

func newOracle() *Oracle {
	o := &Oracle{
		ContractMD:  *interop.NewContractMD(nativenames.Oracle, nativeids.OracleContract),
		newRequests: make(map[uint64]*state.OracleRequest),
	}
	defer o.BuildHFSpecificMD(o.ActiveIn())

	o.oracleScript = CreateOracleResponseScript(o.Hash)

	desc := NewDescriptor("request", smartcontract.VoidType,
		manifest.NewParameter("url", smartcontract.StringType),
		manifest.NewParameter("filter", smartcontract.StringType),
		manifest.NewParameter("callback", smartcontract.StringType),
		manifest.NewParameter("userData", smartcontract.AnyType),
		manifest.NewParameter("gasForResponse", smartcontract.IntegerType))
	md := NewMethodAndPrice(o.request, 0, callflag.States|callflag.AllowNotify)
	o.AddMethod(md, desc)

	desc = NewDescriptor("finish", smartcontract.VoidType)
	md = NewMethodAndPrice(o.finish, 0, callflag.States|callflag.AllowCall|callflag.AllowNotify)
	o.AddMethod(md, desc)

	desc = NewDescriptor("verify", smartcontract.BoolType)
	md = NewMethodAndPrice(o.verify, 1<<15, callflag.NoneFlag)
	o.AddMethod(md, desc)

	eDesc := NewEventDescriptor("OracleRequest", manifest.NewParameter("Id", smartcontract.IntegerType),
		manifest.NewParameter("RequestContract", smartcontract.Hash160Type),
		manifest.NewParameter("Url", smartcontract.StringType),
		manifest.NewParameter("Filter", smartcontract.StringType))
	eMD := NewEvent(eDesc)
	o.AddEvent(eMD)

	eDesc = NewEventDescriptor("OracleResponse", manifest.NewParameter("Id", smartcontract.IntegerType),
		manifest.NewParameter("OriginalTx", smartcontract.Hash256Type))
	eMD = NewEvent(eDesc)
	o.AddEvent(eMD)

	desc = NewDescriptor("getPrice", smartcontract.IntegerType)
	md = NewMethodAndPrice(o.getPrice, 1<<15, callflag.ReadStates)
	o.AddMethod(md, desc)

	desc = NewDescriptor("setPrice", smartcontract.VoidType,
		manifest.NewParameter("price", smartcontract.IntegerType))
	md = NewMethodAndPrice(o.setPrice, 1<<15, callflag.States)
	o.AddMethod(md, desc)

	return o
}

// GetOracleResponseScript returns a script for the transaction with an oracle response.
func (o *Oracle) GetOracleResponseScript() []byte {
	return bytes.Clone(o.oracleScript)
}

// OnPersist implements the Contract interface.
func (o *Oracle) OnPersist(ic *interop.Context) error {
	return nil
}

// PostPersist represents `postPersist` method.
func (o *Oracle) PostPersist(ic *interop.Context) error {
	p := o.getPriceInternal(ic.DAO)

	var nodes keys.PublicKeys
	var reward []big.Int
	single := big.NewInt(p)
	var removedIDs []uint64

	orc, _ := o.Module.Load().(*OracleService)
	for _, tx := range ic.Block.Transactions {
		resp := getResponse(tx)
		if resp == nil {
			continue
		}
		reqKey := makeRequestKey(resp.ID)
		req := new(state.OracleRequest)
		if err := o.getConvertibleFromDAO(ic.DAO, reqKey, req); err != nil {
			continue
		}
		ic.DAO.DeleteStorageItem(o.ID, reqKey)
		if orc != nil && *orc != nil {
			removedIDs = append(removedIDs, resp.ID)
		}

		idKey := makeIDListKey(req.URL)
		idList := new(IDList)
		if err := o.getConvertibleFromDAO(ic.DAO, idKey, idList); err != nil {
			return err
		}
		if !idList.Remove(resp.ID) {
			return errors.New("response ID wasn't found")
		}

		var err error
		if len(*idList) == 0 {
			ic.DAO.DeleteStorageItem(o.ID, idKey)
		} else {
			err = putConvertibleToDAO(o.ID, ic.DAO, idKey, idList)
		}
		if err != nil {
			return err
		}

		if nodes == nil {
			nodes, err = o.GetOracleNodes(ic.DAO)
			if err != nil {
				return err
			}
			reward = make([]big.Int, len(nodes))
		}

		if len(reward) > 0 {
			index := resp.ID % uint64(len(nodes))
			reward[index].Add(&reward[index], single)
		}
	}
	for i := range reward {
		o.GAS.Mint(ic, nodes[i].GetScriptHash(), &reward[i], false)
	}

	if len(removedIDs) != 0 {
		(*orc).RemoveRequests(removedIDs)
	}
	return o.updateCache(ic.DAO)
}

// Metadata returns contract metadata.
func (o *Oracle) Metadata() *interop.ContractMD {
	return &o.ContractMD
}

// Initialize initializes an Oracle contract.
func (o *Oracle) Initialize(ic *interop.Context, hf *config.Hardfork, newMD *interop.HFSpecificContractMD) error {
	switch hf {
	case o.ActiveIn():
		setIntWithKey(o.ID, ic.DAO, prefixRequestID, 0)
		setIntWithKey(o.ID, ic.DAO, prefixRequestPrice, DefaultOracleRequestPrice)

		cache := &OracleCache{
			requestPrice: int64(DefaultOracleRequestPrice),
		}
		ic.DAO.SetCache(o.ID, cache)
	default:
		orc, _ := o.Module.Load().(*OracleService)
		if orc != nil && *orc != nil {
			md, ok := newMD.GetMethod(manifest.MethodVerify, -1)
			if !ok {
				panic(fmt.Errorf("%s method not found", manifest.MethodVerify))
			}
			(*orc).UpdateNativeContract(newMD.NEF.Script, o.GetOracleResponseScript(),
				o.Hash, md.MD.Offset)
		}
	}

	return nil
}

func (o *Oracle) InitializeCache(_ interop.IsHardforkEnabled, blockHeight uint32, d *dao.Simple) error {
	cache := &OracleCache{}
	cache.requestPrice = getIntWithKey(o.ID, d, prefixRequestPrice)
	d.SetCache(o.ID, cache)
	return nil
}

// ActiveIn implements the Contract interface.
func (o *Oracle) ActiveIn() *config.Hardfork {
	return nil
}

// SetService implements IOracle interface.
func (o *Oracle) SetService(s OracleService) {
	o.Module.Store(&s)
}

func getResponse(tx *transaction.Transaction) *transaction.OracleResponse {
	for i := range tx.Attributes {
		if tx.Attributes[i].Type == transaction.OracleResponseT {
			return tx.Attributes[i].Value.(*transaction.OracleResponse)
		}
	}
	return nil
}

func (o *Oracle) finish(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	err := o.FinishInternal(ic)
	if err != nil {
		panic(err)
	}
	return stackitem.Null{}
}

// FinishInternal processes an oracle response.
func (o *Oracle) FinishInternal(ic *interop.Context) error {
	if len(ic.VM.Istack()) != 2 {
		return errors.New("'Oracle.finish' called from non-entry script")
	}
	if ic.Invocations[o.Hash] != 1 {
		return errors.New("'Oracle.finish' called multiple times")
	}
	resp := getResponse(ic.Tx)
	if resp == nil {
		return ErrResponseNotFound
	}
	req, err := o.GetRequestInternal(ic.DAO, resp.ID)
	if err != nil {
		return ErrRequestNotFound
	}
	err = ic.AddNotification(o.Hash, "OracleResponse", stackitem.NewArray([]stackitem.Item{
		stackitem.Make(resp.ID),
		stackitem.Make(req.OriginalTxID.BytesBE()),
	}))
	if err != nil {
		return err
	}

	origTx, _, err := ic.DAO.GetTransaction(req.OriginalTxID)
	if err != nil {
		return ErrRequestNotFound
	}
	ic.UseSigners(origTx.Signers)
	defer ic.UseSigners(nil)

	userData, err := stackitem.Deserialize(req.UserData)
	if err != nil {
		return err
	}
	args := []stackitem.Item{
		stackitem.Make(req.URL),
		userData,
		stackitem.Make(resp.Code),
		stackitem.Make(resp.Result),
	}
	cs, err := ic.GetContract(req.CallbackContract)
	if err != nil {
		return err
	}
	return contract.CallFromNative(ic, o.Hash, cs, req.CallbackMethod, args, false)
}

func (o *Oracle) request(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	url, err := stackitem.ToString(args[0])
	if err != nil {
		panic(err)
	}
	var filter *string
	_, ok := args[1].(stackitem.Null)
	if !ok {
		// Check UTF-8 validity.
		str, err := stackitem.ToString(args[1])
		if err != nil {
			panic(err)
		}
		filter = &str
	}
	cb, err := stackitem.ToString(args[2])
	if err != nil {
		panic(err)
	}
	userData := args[3]
	gas, err := args[4].TryInteger()
	if err != nil {
		panic(err)
	}
	if !ic.VM.AddGas(o.getPriceInternal(ic.DAO)) {
		panic("insufficient gas")
	}
	if err := o.RequestInternal(ic, url, filter, cb, userData, gas); err != nil {
		panic(err)
	}
	return stackitem.Null{}
}

// RequestInternal processes an oracle request.
func (o *Oracle) RequestInternal(ic *interop.Context, url string, filter *string, cb string, userData stackitem.Item, gas *big.Int) error {
	if len(url) > maxURLLength || (filter != nil && len(*filter) > maxFilterLength) || len(cb) > maxCallbackLength || !gas.IsInt64() {
		return ErrBigArgument
	}
	if gas.Int64() < MinimumResponseGas {
		return ErrLowResponseGas
	}
	if strings.HasPrefix(cb, "_") {
		return errors.New("disallowed callback method (starts with '_')")
	}

	if !ic.VM.AddGas(gas.Int64()) {
		return ErrNotEnoughGas
	}
	callingHash := ic.VM.GetCallingScriptHash()
	o.GAS.Mint(ic, o.Hash, gas, false)
	si := ic.DAO.GetStorageItem(o.ID, prefixRequestID)
	itemID := bigint.FromBytes(si)
	id := itemID.Uint64()
	itemID.Add(itemID, intOne)
	ic.DAO.PutBigInt(o.ID, prefixRequestID, itemID)

	// Should be executed from the contract.
	_, err := ic.GetContract(ic.VM.GetCallingScriptHash())
	if err != nil {
		return err
	}

	data, err := ic.DAO.GetItemCtx().Serialize(userData, false)
	if err != nil {
		return err
	}
	if len(data) > maxUserDataLength {
		return ErrBigArgument
	}
	data = bytes.Clone(data) // Serialization context will be used in PutRequestInternal again.

	var filterNotif stackitem.Item
	if filter != nil {
		filterNotif = stackitem.Make(*filter)
	} else {
		filterNotif = stackitem.Null{}
	}
	err = ic.AddNotification(o.Hash, "OracleRequest", stackitem.NewArray([]stackitem.Item{
		stackitem.Make(id),
		stackitem.Make(ic.VM.GetCallingScriptHash().BytesBE()),
		stackitem.Make(url),
		filterNotif,
	}))
	if err != nil {
		return err
	}
	req := &state.OracleRequest{
		OriginalTxID:     o.getOriginalTxID(ic.DAO, ic.Tx),
		GasForResponse:   gas.Uint64(),
		URL:              url,
		Filter:           filter,
		CallbackContract: callingHash,
		CallbackMethod:   cb,
		UserData:         data,
	}
	return o.PutRequestInternal(id, req, ic.DAO)
}

// PutRequestInternal puts the oracle request with the specified id to d.
func (o *Oracle) PutRequestInternal(id uint64, req *state.OracleRequest, d *dao.Simple) error {
	reqKey := makeRequestKey(id)
	if err := putConvertibleToDAO(o.ID, d, reqKey, req); err != nil {
		return err
	}
	orc, _ := o.Module.Load().(*OracleService)
	if orc != nil && *orc != nil {
		o.newRequests[id] = req
	}

	// Add request ID to the id list.
	lst := new(IDList)
	key := makeIDListKey(req.URL)
	if err := o.getConvertibleFromDAO(d, key, lst); err != nil && !errors.Is(err, storage.ErrKeyNotFound) {
		return err
	}
	if len(*lst) >= maxRequestsCount {
		return fmt.Errorf("there are too many pending requests for %s url", req.URL)
	}
	*lst = append(*lst, id)
	return putConvertibleToDAO(o.ID, d, key, lst)
}

// GetScriptHash returns script hash of oracle nodes.
func (o *Oracle) GetScriptHash(d *dao.Simple) (util.Uint160, error) {
	return o.Desig.GetLastDesignatedHash(d, noderoles.Oracle)
}

// GetOracleNodes returns public keys of oracle nodes.
func (o *Oracle) GetOracleNodes(d *dao.Simple) (keys.PublicKeys, error) {
	nodes, _, err := o.Desig.GetDesignatedByRole(d, noderoles.Oracle, math.MaxUint32)
	return nodes, err
}

// GetRequestInternal returns the request by ID and key under which it is stored.
func (o *Oracle) GetRequestInternal(d *dao.Simple, id uint64) (*state.OracleRequest, error) {
	key := makeRequestKey(id)
	req := new(state.OracleRequest)
	return req, o.getConvertibleFromDAO(d, key, req)
}

// GetIDListInternal returns the request by ID and key under which it is stored.
func (o *Oracle) GetIDListInternal(d *dao.Simple, url string) (*IDList, error) {
	key := makeIDListKey(url)
	idList := new(IDList)
	return idList, o.getConvertibleFromDAO(d, key, idList)
}

func (o *Oracle) verify(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBool(ic.Tx.HasAttribute(transaction.OracleResponseT))
}

func (o *Oracle) getPrice(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(o.getPriceInternal(ic.DAO)))
}

func (o *Oracle) getPriceInternal(d *dao.Simple) int64 {
	cache := d.GetROCache(o.ID).(*OracleCache)
	return cache.requestPrice
}

func (o *Oracle) setPrice(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	price := toBigInt(args[0])
	if price.Sign() <= 0 || !price.IsInt64() {
		panic("invalid register price")
	}
	if !o.NEO.CheckCommittee(ic) {
		panic("invalid committee signature")
	}
	setIntWithKey(o.ID, ic.DAO, prefixRequestPrice, price.Int64())
	cache := ic.DAO.GetRWCache(o.ID).(*OracleCache)
	cache.requestPrice = price.Int64()
	return stackitem.Null{}
}

func (o *Oracle) getOriginalTxID(d *dao.Simple, tx *transaction.Transaction) util.Uint256 {
	for i := range tx.Attributes {
		if tx.Attributes[i].Type == transaction.OracleResponseT {
			id := tx.Attributes[i].Value.(*transaction.OracleResponse).ID
			req, _ := o.GetRequestInternal(d, id)
			return req.OriginalTxID
		}
	}
	return tx.Hash()
}

// GetRequests returns all requests which have not been finished yet.
func (o *Oracle) GetRequests(d *dao.Simple) (map[uint64]*state.OracleRequest, error) {
	var reqs = make(map[uint64]*state.OracleRequest)
	var err error
	d.Seek(o.ID, storage.SeekRange{Prefix: prefixRequest}, func(k, v []byte) bool {
		if len(k) != 8 {
			err = errors.New("invalid request ID")
			return false
		}
		req := new(state.OracleRequest)
		err = stackitem.DeserializeConvertible(v, req)
		if err != nil {
			return false
		}
		id := binary.BigEndian.Uint64(k)
		reqs[id] = req
		return true
	})
	if err != nil {
		return nil, err
	}
	return reqs, nil
}

func makeRequestKey(id uint64) []byte {
	k := make([]byte, 9)
	k[0] = prefixRequest[0]
	binary.BigEndian.PutUint64(k[1:], id)
	return k
}

func makeIDListKey(url string) []byte {
	return append(prefixIDList, hash.Hash160([]byte(url)).BytesBE()...)
}

func (o *Oracle) getConvertibleFromDAO(d *dao.Simple, key []byte, item stackitem.Convertible) error {
	return getConvertibleFromDAO(o.ID, d, key, item)
}

// updateCache updates cached Oracle values if they've been changed.
func (o *Oracle) updateCache(d *dao.Simple) error {
	orc, _ := o.Module.Load().(*OracleService)
	if orc == nil || *orc == nil {
		return nil
	}

	reqs := o.newRequests
	o.newRequests = make(map[uint64]*state.OracleRequest)
	for id := range reqs {
		key := makeRequestKey(id)
		if si := d.GetStorageItem(o.ID, key); si == nil { // tx has failed
			delete(reqs, id)
		}
	}
	(*orc).AddRequests(reqs)
	return nil
}

// CreateOracleResponseScript returns a script that is used to create the native Oracle
// response.
func CreateOracleResponseScript(nativeOracleHash util.Uint160) []byte {
	script, err := smartcontract.CreateCallScript(nativeOracleHash, "finish")
	if err != nil {
		panic(fmt.Errorf("failed to create Oracle response script: %w", err))
	}
	return script
}
