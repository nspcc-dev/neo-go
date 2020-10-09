package native

import (
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Oracle represents Oracle native contract.
type Oracle struct {
	interop.ContractMD
	GAS *GAS
	NEO *NEO

	Desig *Designate
}

const (
	oracleContractID = -4
	oracleName       = "Oracle"
)

const (
	maxURLLength      = 256
	maxFilterLength   = 128
	maxCallbackLength = 32
	maxUserDataLength = 512

	oracleRequestPrice = 5000_0000
)

var (
	oracleInvokeScript []byte
	oracleScript       []byte
)

func init() {
	w := io.NewBufBinWriter()
	emit.String(w.BinWriter, oracleName)
	emit.Syscall(w.BinWriter, interopnames.NeoNativeCall)
	oracleInvokeScript = w.Bytes()
	h := hash.Hash160(oracleInvokeScript)

	w = io.NewBufBinWriter()
	emit.Int(w.BinWriter, 0)
	emit.Opcodes(w.BinWriter, opcode.NEWARRAY)
	emit.String(w.BinWriter, "finish")
	emit.Bytes(w.BinWriter, h.BytesBE())
	emit.Syscall(w.BinWriter, interopnames.SystemContractCall)
	oracleScript = w.Bytes()
}

var (
	prefixIDList    = []byte{6}
	prefixRequest   = []byte{7}
	prefixNodeList  = []byte{8}
	prefixRequestID = []byte{9}
)

// Various validation errors.
var (
	ErrBigArgument      = errors.New("some of the arguments are invalid")
	ErrInvalidWitness   = errors.New("witness check failed")
	ErrNotEnoughGas     = errors.New("gas limit exceeded")
	ErrRequestNotFound  = errors.New("oracle request not found")
	ErrResponseNotFound = errors.New("oracle response not found")
)

// GetOracleInvokeScript returns oracle contract script.
func GetOracleInvokeScript() []byte {
	b := make([]byte, len(oracleInvokeScript))
	copy(b, oracleInvokeScript)
	return b
}

// GetOracleResponseScript returns script for transaction with oracle response.
func GetOracleResponseScript() []byte {
	b := make([]byte, len(oracleScript))
	copy(b, oracleScript)
	return b
}

func newOracle() *Oracle {
	o := &Oracle{ContractMD: *interop.NewContractMD(oracleName)}
	o.ContractID = oracleContractID
	o.Manifest.Features = smartcontract.HasStorage

	desc := newDescriptor("request", smartcontract.VoidType,
		manifest.NewParameter("url", smartcontract.StringType),
		manifest.NewParameter("filter", smartcontract.StringType),
		manifest.NewParameter("callback", smartcontract.StringType),
		manifest.NewParameter("userData", smartcontract.AnyType),
		manifest.NewParameter("gasForResponse", smartcontract.IntegerType))
	md := newMethodAndPrice(o.request, oracleRequestPrice, smartcontract.AllowModifyStates)
	o.AddMethod(md, desc, false)

	desc = newDescriptor("name", smartcontract.StringType)
	md = newMethodAndPrice(nameMethod(oracleName), 0, smartcontract.NoneFlag)
	o.AddMethod(md, desc, true)

	desc = newDescriptor("finish", smartcontract.VoidType)
	md = newMethodAndPrice(o.finish, 0, smartcontract.AllowModifyStates)
	o.AddMethod(md, desc, false)

	desc = newDescriptor("verify", smartcontract.BoolType)
	md = newMethodAndPrice(o.verify, 100_0000, smartcontract.NoneFlag)
	o.AddMethod(md, desc, false)

	pp := chainOnPersist(postPersistBase, o.PostPersist)
	desc = newDescriptor("postPersist", smartcontract.VoidType)
	md = newMethodAndPrice(getOnPersistWrapper(pp), 0, smartcontract.AllowModifyStates)
	o.AddMethod(md, desc, false)

	return o
}

// PostPersist represents `postPersist` method.
func (o *Oracle) PostPersist(ic *interop.Context) error {
	var nodes keys.PublicKeys
	var reward []big.Int
	single := new(big.Int).SetUint64(oracleRequestPrice)
	for _, tx := range ic.Block.Transactions {
		resp := getResponse(tx)
		if resp == nil {
			continue
		}
		reqKey := makeRequestKey(resp.ID)
		req := new(state.OracleRequest)
		if err := o.getSerializableFromDAO(ic.DAO, reqKey, req); err != nil {
			return err
		}
		if err := ic.DAO.DeleteStorageItem(o.ContractID, reqKey); err != nil {
			return err
		}

		idKey := makeIDListKey(req.URL)
		idList := new(IDList)
		if err := o.getSerializableFromDAO(ic.DAO, idKey, idList); err != nil {
			return err
		}
		if !idList.Remove(resp.ID) {
			return errors.New("response ID wasn't found")
		}

		var err error
		if len(*idList) == 0 {
			err = ic.DAO.DeleteStorageItem(o.ContractID, idKey)
		} else {
			si := &state.StorageItem{Value: idList.Bytes()}
			err = ic.DAO.PutStorageItem(o.ContractID, idKey, si)
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
		o.GAS.mint(ic, nodes[i].GetScriptHash(), &reward[i])
	}
	return nil
}

// Metadata returns contract metadata.
func (o *Oracle) Metadata() *interop.ContractMD {
	return &o.ContractMD
}

// Initialize initializes Oracle contract.
func (o *Oracle) Initialize(ic *interop.Context) error {
	si := &state.StorageItem{Value: NodeList{}.Bytes()}
	if err := ic.DAO.PutStorageItem(o.ContractID, prefixNodeList, si); err != nil {
		return err
	}
	si = &state.StorageItem{Value: make([]byte, 8)} // uint64(0) LE
	return ic.DAO.PutStorageItem(o.ContractID, prefixRequestID, si)
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

// FinishInternal processes oracle response.
func (o *Oracle) FinishInternal(ic *interop.Context) error {
	resp := getResponse(ic.Tx)
	if resp == nil {
		return ErrResponseNotFound
	}
	req, err := o.GetRequestInternal(ic.DAO, resp.ID)
	if err != nil {
		return ErrRequestNotFound
	}

	r := io.NewBinReaderFromBuf(req.UserData)
	userData := stackitem.DecodeBinaryStackItem(r)
	args := stackitem.NewArray([]stackitem.Item{
		stackitem.Make(req.URL),
		stackitem.Make(userData),
		stackitem.Make(resp.Code),
		stackitem.Make(resp.Result),
	})
	ic.VM.Estack().PushVal(args)
	ic.VM.Estack().PushVal(req.CallbackMethod)
	ic.VM.Estack().PushVal(req.CallbackContract.BytesBE())
	return contract.Call(ic)
}

func (o *Oracle) request(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	url, err := stackitem.ToString(args[0])
	if err != nil {
		panic(err)
	}
	filter, err := stackitem.ToString(args[1])
	if err != nil {
		panic(err)
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
	if err := o.RequestInternal(ic, url, filter, cb, userData, gas); err != nil {
		panic(err)
	}
	return stackitem.Null{}
}

// RequestInternal processes oracle request.
func (o *Oracle) RequestInternal(ic *interop.Context, url, filter, cb string, userData stackitem.Item, gas *big.Int) error {
	if len(url) > maxURLLength || len(filter) > maxFilterLength || len(cb) > maxCallbackLength || gas.Uint64() < 1000_0000 {
		return ErrBigArgument
	}

	if !ic.VM.AddGas(gas.Int64()) {
		return ErrNotEnoughGas
	}
	o.GAS.mint(ic, o.Hash, gas)
	si := ic.DAO.GetStorageItem(o.ContractID, prefixRequestID)
	id := binary.LittleEndian.Uint64(si.Value) + 1
	binary.LittleEndian.PutUint64(si.Value, id)
	if err := ic.DAO.PutStorageItem(o.ContractID, prefixRequestID, si); err != nil {
		return err
	}

	// Should be executed from contract.
	_, err := ic.DAO.GetContractState(ic.VM.GetCallingScriptHash())
	if err != nil {
		return err
	}

	w := io.NewBufBinWriter()
	stackitem.EncodeBinaryStackItem(userData, w.BinWriter)
	if w.Err != nil {
		return w.Err
	}
	data := w.Bytes()
	if len(data) > maxUserDataLength {
		return ErrBigArgument
	}

	req := &state.OracleRequest{
		OriginalTxID:     o.getOriginalTxID(ic.DAO, ic.Tx),
		GasForResponse:   gas.Uint64(),
		URL:              url,
		Filter:           &filter,
		CallbackContract: ic.VM.GetCallingScriptHash(),
		CallbackMethod:   cb,
		UserData:         data,
	}
	return o.PutRequestInternal(id, req, ic.DAO)
}

// PutRequestInternal puts oracle request with the specified id to d.
func (o *Oracle) PutRequestInternal(id uint64, req *state.OracleRequest, d dao.DAO) error {
	reqItem := &state.StorageItem{Value: req.Bytes()}
	reqKey := makeRequestKey(id)
	if err := d.PutStorageItem(o.ContractID, reqKey, reqItem); err != nil {
		return err
	}

	// Add request ID to the id list.
	lst := new(IDList)
	key := makeIDListKey(req.URL)
	if err := o.getSerializableFromDAO(d, key, lst); err != nil && !errors.Is(err, storage.ErrKeyNotFound) {
		return err
	}
	*lst = append(*lst, id)
	si := &state.StorageItem{Value: lst.Bytes()}
	return d.PutStorageItem(o.ContractID, key, si)
}

// GetScriptHash returns script hash or oracle nodes.
func (o *Oracle) GetScriptHash() (util.Uint160, error) {
	h := o.Desig.oracleHash.Load()
	if h == nil {
		return util.Uint160{}, storage.ErrKeyNotFound
	}
	return h.(util.Uint160), nil
}

// GetOracleNodes returns public keys of oracle nodes.
func (o *Oracle) GetOracleNodes(d dao.DAO) (keys.PublicKeys, error) {
	return o.Desig.GetDesignatedByRole(d, RoleOracle)
}

// GetRequestInternal returns request by ID and key under which it is stored.
func (o *Oracle) GetRequestInternal(d dao.DAO, id uint64) (*state.OracleRequest, error) {
	key := makeRequestKey(id)
	req := new(state.OracleRequest)
	return req, o.getSerializableFromDAO(d, key, req)
}

// GetIDListInternal returns request by ID and key under which it is stored.
func (o *Oracle) GetIDListInternal(d dao.DAO, url string) (*IDList, error) {
	key := makeIDListKey(url)
	idList := new(IDList)
	return idList, o.getSerializableFromDAO(d, key, idList)
}

func (o *Oracle) verify(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBool(ic.Tx.HasAttribute(transaction.OracleResponseT))
}

func (o *Oracle) getOriginalTxID(d dao.DAO, tx *transaction.Transaction) util.Uint256 {
	for i := range tx.Attributes {
		if tx.Attributes[i].Type == transaction.OracleResponseT {
			id := tx.Attributes[i].Value.(*transaction.OracleResponse).ID
			req, _ := o.GetRequestInternal(d, id)
			return req.OriginalTxID
		}
	}
	return tx.Hash()
}

func makeRequestKey(id uint64) []byte {
	k := make([]byte, 9)
	k[0] = prefixRequest[0]
	binary.LittleEndian.PutUint64(k[1:], id)
	return k
}

func makeIDListKey(url string) []byte {
	return append(prefixIDList, hash.Hash160([]byte(url)).BytesBE()...)
}

func (o *Oracle) getSerializableFromDAO(d dao.DAO, key []byte, item io.Serializable) error {
	return getSerializableFromDAO(o.ContractID, d, key, item)
}
