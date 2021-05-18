/*
Package nns contains non-divisible non-fungible NEP11-compatible token
implementation. This token is a compatible analogue of C# Neo Name Service
token and is aimed to serve as a domain name service for Neo smart-contracts,
thus it's NeoNameService. This token can be minted with new domain name
registration, the domain name itself is your NFT. Corresponding domain root
must be added by the committee before new domain name can be registered.
*/
package nns

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/crypto"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/management"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/neo"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/std"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/interop/util"
)

// Prefixes used for contract data storage.
const (
	// prefixTotalSupply contains total supply of minted domains.
	prefixTotalSupply byte = 0x00
	// prefixBalance contains map from owner to his balance.
	prefixBalance byte = 0x01
	// prefixAccountToken contains map from (owner + token key) to token ID,
	// where token key = hash160(token ID) and token ID = domain name.
	prefixAccountToken byte = 0x02
	// prefixRegisterPrice contains price for new domain name registration.
	prefixRegisterPrice byte = 0x10
	// prefixRoot contains set of roots (map from root to 0).
	prefixRoot byte = 0x20
	// prefixName contains map from token key to token where token is domain
	// NameState structure.
	prefixName byte = 0x21
	// prefixRecord contains map from (token key + hash160(token name) + record type)
	// to record.
	prefixRecord byte = 0x22
)

// Values constraints.
const (
	// maxRegisterPrice is the maximum price of register method.
	maxRegisterPrice = 1_0000_0000_0000
	// maxRootLength is the maximum domain root length.
	maxRootLength = 16
	// maxDomainNameFragmentLength is the maximum length of the domain name fragment.
	maxDomainNameFragmentLength = 62
	// minDomainNameLength is minimum domain length.
	minDomainNameLength = 3
	// maxDomainNameLength is maximum domain length.
	maxDomainNameLength = 255
	// maxTXTRecordLength is the maximum length of the TXT domain record.
	maxTXTRecordLength = 255
)

// Other constants.
const (
	// defaultRegisterPrice is the default price for new domain registration.
	defaultRegisterPrice = 10_0000_0000
	// millisecondsInYear is amount of milliseconds per year.
	millisecondsInYear = 365 * 24 * 3600 * 1000
)

// Update updates NameService contract.
func Update(nef []byte, manifest string) {
	checkCommittee()
	management.Update(nef, []byte(manifest))
}

// _deploy initializes defaults (total supply and registration price) on contract deploy.
func _deploy(data interface{}, isUpdate bool) {
	if isUpdate {
		return
	}
	ctx := storage.GetContext()
	storage.Put(ctx, []byte{prefixTotalSupply}, 0)
	storage.Put(ctx, []byte{prefixRegisterPrice}, defaultRegisterPrice)
}

// Symbol returns NeoNameService symbol.
func Symbol() string {
	return "NNS"
}

// Decimals returns NeoNameService decimals.
func Decimals() int {
	return 0
}

// TotalSupply returns overall number of domains minted by the NeoNameService contract.
func TotalSupply() int {
	ctx := storage.GetReadOnlyContext()
	return getTotalSupply(ctx)
}

// OwnerOf returns owner of the specified domain.
func OwnerOf(tokenID []byte) interop.Hash160 {
	ctx := storage.GetReadOnlyContext()
	ns := getNameState(ctx, tokenID)
	return ns.Owner
}

// Properties returns domain name and expiration date of the specified domain.
func Properties(tokenID []byte) map[string]interface{} {
	ctx := storage.GetReadOnlyContext()
	ns := getNameState(ctx, tokenID)
	return map[string]interface{}{
		"name":       ns.Name,
		"expiration": ns.Expiration,
	}
}

// BalanceOf returns overall number of domains owned by the specified owner.
func BalanceOf(owner interop.Hash160) int {
	if !isValid(owner) {
		panic(`invalid owner`)
	}
	ctx := storage.GetReadOnlyContext()
	balance := storage.Get(ctx, append([]byte{prefixBalance}, owner...))
	if balance == nil {
		return 0
	}
	return balance.(int)
}

// Tokens returns iterator over a set of all registered domain names.
func Tokens() iterator.Iterator {
	ctx := storage.GetReadOnlyContext()
	return storage.Find(ctx, []byte{prefixName}, storage.ValuesOnly|storage.DeserializeValues|storage.PickField1)
}

// TokensOf returns iterator over minted domains owned by the specified owner.
func TokensOf(owner interop.Hash160) iterator.Iterator {
	if !isValid(owner) {
		panic(`invalid owner`)
	}
	ctx := storage.GetReadOnlyContext()
	return storage.Find(ctx, append([]byte{prefixAccountToken}, owner...), storage.ValuesOnly)
}

// Transfer transfers domain with the specified name to new owner.
func Transfer(to interop.Hash160, tokenID []byte, data interface{}) bool {
	if !isValid(to) {
		panic(`invalid receiver`)
	}
	var (
		tokenKey = getTokenKey(tokenID)
		ctx      = storage.GetContext()
	)
	ns := getNameStateWithKey(ctx, tokenKey)
	from := ns.Owner
	if !runtime.CheckWitness(from) {
		return false
	}
	if !util.Equals(from, to) {
		// update token info
		ns.Owner = to
		ns.Admin = nil
		putNameStateWithKey(ctx, tokenKey, ns)

		// update `from` balance
		updateBalance(ctx, tokenID, from, -1)

		// update `to` balance
		updateBalance(ctx, tokenID, to, +1)
	}
	postTransfer(from, to, tokenID, data)
	return true
}

// AddRoot registers new root.
func AddRoot(root string) {
	checkCommittee()
	if !checkFragment(root, true) {
		panic("invalid root format")
	}
	var (
		ctx     = storage.GetContext()
		rootKey = append([]byte{prefixRoot}, []byte(root)...)
	)
	if storage.Get(ctx, rootKey) != nil {
		panic("root already exists")
	}
	storage.Put(ctx, rootKey, 0)
}

// Roots returns iterator over a set of NameService roots.
func Roots() iterator.Iterator {
	ctx := storage.GetReadOnlyContext()
	return storage.Find(ctx, []byte{prefixRoot}, storage.KeysOnly|storage.RemovePrefix)
}

// SetPrice sets the domain registration price.
func SetPrice(price int) {
	checkCommittee()
	if price < 0 || price > maxRegisterPrice {
		panic("The price is out of range.")
	}
	ctx := storage.GetContext()
	storage.Put(ctx, []byte{prefixRegisterPrice}, price)
}

// GetPrice returns the domain registration price.
func GetPrice() int {
	ctx := storage.GetReadOnlyContext()
	return storage.Get(ctx, []byte{prefixRegisterPrice}).(int)
}

// IsAvailable checks whether provided domain name is available.
func IsAvailable(name string) bool {
	fragments := splitAndCheck(name, false)
	if fragments == nil {
		panic("invalid domain name format")
	}
	ctx := storage.GetReadOnlyContext()
	if storage.Get(ctx, append([]byte{prefixRoot}, []byte(fragments[1])...)) == nil {
		panic("root not found")
	}
	nsBytes := storage.Get(ctx, append([]byte{prefixName}, getTokenKey([]byte(name))...))
	if nsBytes == nil {
		return true
	}
	ns := std.Deserialize(nsBytes.([]byte)).(NameState)
	return runtime.GetTime() >= ns.Expiration
}

// Register registers new domain with the specified owner and name if it's available.
func Register(name string, owner interop.Hash160) bool {
	fragments := splitAndCheck(name, false)
	if fragments == nil {
		panic("invalid domain name format")
	}
	ctx := storage.GetContext()
	if storage.Get(ctx, append([]byte{prefixRoot}, []byte(fragments[1])...)) == nil {
		panic("root not found")
	}

	if !isValid(owner) {
		panic("invalid owner")
	}
	if !runtime.CheckWitness(owner) {
		panic("not witnessed by owner")
	}
	runtime.BurnGas(GetPrice())
	var (
		tokenKey = getTokenKey([]byte(name))
		oldOwner interop.Hash160
	)
	nsBytes := storage.Get(ctx, append([]byte{prefixName}, tokenKey...))
	if nsBytes != nil {
		ns := std.Deserialize(nsBytes.([]byte)).(NameState)
		if runtime.GetTime() < ns.Expiration {
			return false
		}
		oldOwner = ns.Owner
		updateBalance(ctx, []byte(name), oldOwner, -1)
	} else {
		updateTotalSupply(ctx, +1)
	}
	ns := NameState{
		Owner:      owner,
		Name:       name,
		Expiration: runtime.GetTime() + millisecondsInYear,
	}
	putNameStateWithKey(ctx, tokenKey, ns)
	updateBalance(ctx, []byte(name), owner, +1)
	postTransfer(oldOwner, owner, []byte(name), nil)
	return true
}

// Renew increases domain expiration date.
func Renew(name string) int {
	if len(name) > maxDomainNameLength {
		panic("invalid domain name format")
	}
	runtime.BurnGas(GetPrice())
	ctx := storage.GetContext()
	ns := getNameState(ctx, []byte(name))
	ns.Expiration += millisecondsInYear
	putNameState(ctx, ns)
	return ns.Expiration
}

// SetAdmin updates domain admin.
func SetAdmin(name string, admin interop.Hash160) {
	if len(name) > maxDomainNameLength {
		panic("invalid domain name format")
	}
	if admin != nil && !runtime.CheckWitness(admin) {
		panic("not witnessed by admin")
	}
	ctx := storage.GetContext()
	ns := getNameState(ctx, []byte(name))
	if !runtime.CheckWitness(ns.Owner) {
		panic("not witnessed by owner")
	}
	ns.Admin = admin
	putNameState(ctx, ns)
}

// SetRecord adds new record of the specified type to the provided domain.
func SetRecord(name string, typ RecordType, data string) {
	tokenID := []byte(tokenIDFromName(name))
	var ok bool
	switch typ {
	case A:
		ok = checkIPv4(data)
	case CNAME:
		ok = splitAndCheck(data, true) != nil
	case TXT:
		ok = len(data) <= maxTXTRecordLength
	case AAAA:
		ok = checkIPv6(data)
	default:
		panic("unsupported record type")
	}
	if !ok {
		panic("invalid record data")
	}
	ctx := storage.GetContext()
	ns := getNameState(ctx, tokenID)
	ns.checkAdmin()
	putRecord(ctx, tokenID, name, typ, data)
}

// GetRecord returns domain record of the specified type if it exists or an empty
// string if not.
func GetRecord(name string, typ RecordType) string {
	tokenID := []byte(tokenIDFromName(name))
	ctx := storage.GetReadOnlyContext()
	_ = getNameState(ctx, tokenID) // ensure not expired
	return getRecord(ctx, tokenID, name, typ)
}

// DeleteRecord removes domain record with the specified type.
func DeleteRecord(name string, typ RecordType) {
	tokenID := []byte(tokenIDFromName(name))
	ctx := storage.GetContext()
	ns := getNameState(ctx, tokenID)
	ns.checkAdmin()
	recordKey := getRecordKey(tokenID, name, typ)
	storage.Delete(ctx, recordKey)
}

// Resolve resolves given name (not more then three redirects are allowed).
func Resolve(name string, typ RecordType) string {
	ctx := storage.GetReadOnlyContext()
	return resolve(ctx, name, typ, 2)
}

// updateBalance updates account's balance and account's tokens.
func updateBalance(ctx storage.Context, tokenId []byte, acc interop.Hash160, diff int) {
	balanceKey := append([]byte{prefixBalance}, acc...)
	var balance int
	if b := storage.Get(ctx, balanceKey); b != nil {
		balance = b.(int)
	}
	balance += diff
	if balance == 0 {
		storage.Delete(ctx, balanceKey)
	} else {
		storage.Put(ctx, balanceKey, balance)
	}

	tokenKey := getTokenKey(tokenId)
	accountTokenKey := append(append([]byte{prefixAccountToken}, acc...), tokenKey...)
	if diff < 0 {
		storage.Delete(ctx, accountTokenKey)
	} else {
		storage.Put(ctx, accountTokenKey, tokenId)
	}
}

// postTransfer sends Transfer notification to the network and calls onNEP11Payment
// method.
func postTransfer(from, to interop.Hash160, tokenID []byte, data interface{}) {
	runtime.Notify("Transfer", from, to, 1, tokenID)
	if management.GetContract(to) != nil {
		contract.Call(to, "onNEP11Payment", contract.All, from, 1, tokenID, data)
	}
}

// getTotalSupply returns total supply from storage.
func getTotalSupply(ctx storage.Context) int {
	val := storage.Get(ctx, []byte{prefixTotalSupply})
	return val.(int)
}

// updateTotalSupply adds specified diff to the total supply.
func updateTotalSupply(ctx storage.Context, diff int) {
	tsKey := []byte{prefixTotalSupply}
	ts := getTotalSupply(ctx)
	storage.Put(ctx, tsKey, ts+diff)
}

// getTokenKey computes hash160 from the given tokenID.
func getTokenKey(tokenID []byte) []byte {
	return crypto.Ripemd160(tokenID)
}

// getNameState returns domain name state by the specified tokenID.
func getNameState(ctx storage.Context, tokenID []byte) NameState {
	tokenKey := getTokenKey(tokenID)
	return getNameStateWithKey(ctx, tokenKey)
}

// getNameStateWithKey returns domain name state by the specified token key.
func getNameStateWithKey(ctx storage.Context, tokenKey []byte) NameState {
	nameKey := append([]byte{prefixName}, tokenKey...)
	nsBytes := storage.Get(ctx, nameKey)
	if nsBytes == nil {
		panic("token not found")
	}
	ns := std.Deserialize(nsBytes.([]byte)).(NameState)
	ns.ensureNotExpired()
	return ns
}

// putNameState stores domain name state.
func putNameState(ctx storage.Context, ns NameState) {
	tokenKey := getTokenKey([]byte(ns.Name))
	putNameStateWithKey(ctx, tokenKey, ns)
}

// putNameStateWithKey stores domain name state with the specified token key.
func putNameStateWithKey(ctx storage.Context, tokenKey []byte, ns NameState) {
	nameKey := append([]byte{prefixName}, tokenKey...)
	nsBytes := std.Serialize(ns)
	storage.Put(ctx, nameKey, nsBytes)
}

// getRecord returns domain record.
func getRecord(ctx storage.Context, tokenId []byte, name string, typ RecordType) string {
	recordKey := getRecordKey(tokenId, name, typ)
	record := storage.Get(ctx, recordKey)
	return record.(string)
}

// putRecord stores domain record.
func putRecord(ctx storage.Context, tokenId []byte, name string, typ RecordType, record string) {
	recordKey := getRecordKey(tokenId, name, typ)
	storage.Put(ctx, recordKey, record)
}

// getRecordKey returns key used to store domain records.
func getRecordKey(tokenId []byte, name string, typ RecordType) []byte {
	recordKey := append([]byte{prefixRecord}, getTokenKey(tokenId)...)
	recordKey = append(recordKey, getTokenKey([]byte(name))...)
	return append(recordKey, []byte{byte(typ)}...)
}

// isValid returns true if the provided address is a valid Uint160.
func isValid(address interop.Hash160) bool {
	return address != nil && len(address) == 20
}

// checkCommittee panics if the script container is not signed by the committee.
func checkCommittee() {
	committee := neo.GetCommittee()
	if committee == nil {
		panic("failed to get committee")
	}
	l := len(committee)
	committeeMultisig := contract.CreateMultisigAccount(l-(l-1)/2, committee)
	if committeeMultisig == nil || !runtime.CheckWitness(committeeMultisig) {
		panic("not witnessed by committee")
	}
}

// checkFragment validates root or a part of domain name.
func checkFragment(v string, isRoot bool) bool {
	maxLength := maxDomainNameFragmentLength
	if isRoot {
		maxLength = maxRootLength
	}
	if len(v) == 0 || len(v) > maxLength {
		return false
	}
	c := v[0]
	if isRoot {
		if !(c >= 'a' && c <= 'z') {
			return false
		}
	} else {
		if !isAlNum(c) {
			return false
		}
	}
	for i := 1; i < len(v); i++ {
		if !isAlNum(v[i]) {
			return false
		}
	}
	return true
}

// isAlNum checks whether provided char is a lowercase letter or a number.
func isAlNum(c uint8) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
}

// splitAndCheck splits domain name into parts and validates it.
func splitAndCheck(name string, allowMultipleFragments bool) []string {
	l := len(name)
	if l < minDomainNameLength || maxDomainNameLength < l {
		return nil
	}
	fragments := std.StringSplit(name, ".")
	l = len(fragments)
	if l < 2 {
		return nil
	}
	if l > 2 && !allowMultipleFragments {
		return nil
	}
	for i := 0; i < l; i++ {
		if !checkFragment(fragments[i], i == l-1) {
			return nil
		}
	}
	return fragments
}

// checkIPv4 checks record on IPv4 compliance.
func checkIPv4(data string) bool {
	l := len(data)
	if l < 7 || 15 < l {
		return false
	}
	fragments := std.StringSplit(data, ".")
	if len(fragments) != 4 {
		return false
	}
	numbers := make([]int, 4)
	for i, f := range fragments {
		if len(f) == 0 {
			return false
		}
		number := std.Atoi10(f)
		if number < 0 || 255 < number {
			panic("not a byte")
		}
		if number > 0 && f[0] == '0' {
			return false
		}
		if number == 0 && len(f) > 1 {
			return false
		}
		numbers[i] = number
	}
	n0 := numbers[0]
	n1 := numbers[1]
	n3 := numbers[3]
	if n0 == 0 ||
		n0 == 10 ||
		n0 == 127 ||
		n0 >= 224 ||
		(n0 == 169 && n1 == 254) ||
		(n0 == 172 && 16 <= n1 && n1 <= 31) ||
		(n0 == 192 && n1 == 168) ||
		n3 == 0 ||
		n3 == 255 {
		return false
	}
	return true
}

// checkIPv6 checks record on IPv6 compliance.
func checkIPv6(data string) bool {
	l := len(data)
	if l < 2 || 39 < l {
		return false
	}
	fragments := std.StringSplit(data, ":")
	l = len(fragments)
	if l < 3 || 8 < l {
		return false
	}
	var hasEmpty bool
	nums := make([]int, 8)
	for i, f := range fragments {
		if len(f) == 0 {
			if i == 0 {
				nums[i] = 0
			} else if i == l-1 {
				nums[7] = 0
			} else if hasEmpty {
				return false
			} else {
				hasEmpty = true
				endIndex := 9 - l + i
				for j := i; j < endIndex; j++ {
					nums[j] = 0
				}
			}
		} else {
			if len(f) > 4 {
				return false
			}
			n := std.Atoi(f, 16)
			if 65535 < n {
				panic("fragment overflows uint16: " + f)
			}
			idx := i
			if hasEmpty {
				idx = i + 8 - l
			}
			nums[idx] = n
		}
	}

	f0 := nums[0]
	if f0 < 0x2000 || f0 == 0x2002 || f0 == 0x3ffe || f0 > 0x3fff { // IPv6 Global Unicast https://www.iana.org/assignments/ipv6-address-space/ipv6-address-space.xhtml
		return false
	}
	if f0 == 0x2001 {
		f1 := nums[1]
		if f1 < 0x200 || f1 == 0xdb8 {
			return false
		}
	}
	return true
}

// tokenIDFromName returns token ID (domain.root) from provided name.
func tokenIDFromName(name string) string {
	fragments := splitAndCheck(name, true)
	if fragments == nil {
		panic("invalid domain name format")
	}
	l := len(fragments)
	return name[len(name)-(len(fragments[l-1])+len(fragments[l-2])+1):]
}

// resolve resolves provided name using record with the specified type and given
// maximum redirections constraint.
func resolve(ctx storage.Context, name string, typ RecordType, redirect int) string {
	if redirect < 0 {
		panic("invalid redirect")
	}
	records := getRecords(ctx, name)
	cname := ""
	for iterator.Next(records) {
		r := iterator.Value(records).([]string)
		key := []byte(r[0])
		value := r[1]
		rTyp := key[len(key)-1]
		if rTyp == byte(typ) {
			return value
		}
		if rTyp == byte(CNAME) {
			cname = value
		}
	}
	if cname == "" {
		return string([]byte(nil))
	}
	return resolve(ctx, cname, typ, redirect-1)
}

// getRecords returns iterator over the set of records corresponded with the
// specified name.
func getRecords(ctx storage.Context, name string) iterator.Iterator {
	tokenID := []byte(tokenIDFromName(name))
	_ = getNameState(ctx, tokenID)
	recordsKey := append([]byte{prefixRecord}, getTokenKey(tokenID)...)
	recordsKey = append(recordsKey, getTokenKey([]byte(name))...)
	return storage.Find(ctx, recordsKey, storage.None)
}
