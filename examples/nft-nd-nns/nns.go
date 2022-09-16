/*
Package nns contains non-divisible non-fungible NEP-11-compatible token
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
	// prefixRegisterPrice contains list of prices for new domain name registration
	// depending on the domain name length.
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
	// maxDomainNameFragmentLength is the maximum length of the domain name fragment
	maxDomainNameFragmentLength = 63
	// minDomainNameLength is minimum domain length.
	minDomainNameLength = 3
	// maxDomainNameLength is maximum domain length.
	maxDomainNameLength = 255
	// maxTXTRecordLength is the maximum length of the TXT domain record.
	maxTXTRecordLength = 255
	// maxRecordID is the maximum value of record ID (the upper bound for the number
	// of records with the same type).
	maxRecordID = 255
)

// Other constants.
const (
	// defaultRegisterPrice is the default price for new domain registration.
	defaultRegisterPrice = 10_0000_0000
	// millisecondsInSecond is the amount of milliseconds per second.
	millisecondsInSecond = 1000
	// millisecondsInYear is amount of milliseconds per year.
	millisecondsInYear = 365 * 24 * 3600 * millisecondsInSecond
	// millisecondsInTenYears is the amount of milliseconds per ten years.
	millisecondsInTenYears = 10 * millisecondsInYear
)

// RecordState is a type that registered entities are saved to.
type RecordState struct {
	Name string
	Type RecordType
	Data string
	ID   byte
}

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
	storage.Put(ctx, []byte{prefixRegisterPrice}, std.Serialize([]int{
		defaultRegisterPrice, // Prices for all other lengths of domain names.
		-1,                   // Domain names with a length of 1 are not open for registration by default.
		-1,                   // Domain names with a length of 2 are not open for registration by default.
		-1,                   // Domain names with a length of 3 are not open for registration by default.
		-1,                   // Domain names with a length of 4 are not open for registration by default.
	}))
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
		"admin":      ns.Admin,
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
	if !from.Equals(to) {
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

// Roots returns iterator over a set of NameService roots.
func Roots() iterator.Iterator {
	ctx := storage.GetReadOnlyContext()
	return storage.Find(ctx, []byte{prefixRoot}, storage.KeysOnly|storage.RemovePrefix)
}

// SetPrice sets the domain registration price.
func SetPrice(priceList []int) {
	checkCommittee()
	if len(priceList) == 0 {
		panic("price list is empty")
	}
	for i := 0; i < len(priceList); i++ {
		if i == 0 && priceList[i] == -1 {
			panic("default price is out of range")
		}
		if priceList[i] < -1 || priceList[i] > maxRegisterPrice {
			panic("price is out of range")
		}
	}
	ctx := storage.GetContext()
	storage.Put(ctx, []byte{prefixRegisterPrice}, std.Serialize(priceList))
}

// GetPrice returns the domain registration price depending on the domain name length.
func GetPrice(length int) int {
	ctx := storage.GetReadOnlyContext()
	priceList := std.Deserialize(storage.Get(ctx, []byte{prefixRegisterPrice}).([]byte)).([]int)
	if length >= len(priceList) {
		length = 0
	}
	return priceList[length]
}

// IsAvailable checks whether provided domain name is available.
func IsAvailable(name string) bool {
	fragments := splitAndCheck(name, true)
	if fragments == nil {
		panic("invalid domain name format")
	}
	ctx := storage.GetReadOnlyContext()
	l := len(fragments)
	if storage.Get(ctx, append([]byte{prefixRoot}, []byte(fragments[l-1])...)) == nil {
		if l != 1 {
			panic("TLD not found")
		}
		return true
	}
	if GetPrice(len(fragments[0])) < 0 {
		return false
	}
	if !parentExpired(ctx, 0, fragments) {
		return false
	}
	return len(getParentConflictingRecord(ctx, name, fragments)) == 0
}

// getPrentConflictingRecord returns record of '*.name' format if they are presented.
// These records conflict with domain name to be registered.
func getParentConflictingRecord(ctx storage.Context, name string, fragments []string) string {
	parentKey := getTokenKey([]byte(name[len(fragments[0])+1:]))
	parentRecKey := append([]byte{prefixRecord}, parentKey...)
	it := storage.Find(ctx, parentRecKey, storage.ValuesOnly|storage.DeserializeValues)
	suffix := []byte(name)
	for iterator.Next(it) {
		r := iterator.Value(it).(RecordState)
		ind := std.MemorySearchLastIndex([]byte(r.Name), suffix, len(r.Name))
		if ind > 0 && ind+len(suffix) == len(r.Name) {
			return r.Name
		}
	}
	return ""
}

// parentExpired returns true if any domain from fragments doesn't exist or expired.
// first denotes the deepest subdomain to check.
func parentExpired(ctx storage.Context, first int, fragments []string) bool {
	now := runtime.GetTime()
	last := len(fragments) - 1
	name := fragments[last]
	for i := last; i >= first; i-- {
		if i != last {
			name = fragments[i] + "." + name
		}
		nsBytes := storage.Get(ctx, append([]byte{prefixName}, getTokenKey([]byte(name))...))
		if nsBytes == nil {
			return true
		}
		ns := std.Deserialize(nsBytes.([]byte)).(NameState)
		if now >= ns.Expiration {
			return true
		}
	}
	return false
}

// Register registers new domain with the specified owner and name if it's available.
func Register(name string, owner interop.Hash160, email string, refresh, retry, expire, ttl int) bool {
	fragments := splitAndCheck(name, true)
	if fragments == nil {
		panic("invalid domain name format")
	}
	l := len(fragments)
	tldKey := append([]byte{prefixRoot}, []byte(fragments[l-1])...)
	ctx := storage.GetContext()
	tldBytes := storage.Get(ctx, tldKey)
	if l == 1 {
		checkCommittee()
		if tldBytes != nil {
			panic("TLD already exists")
		}
		storage.Put(ctx, tldKey, 0)
	} else {
		if tldBytes == nil {
			panic("TLD not found")
		}
		if parentExpired(ctx, 1, fragments) {
			panic("one of the parent domains is not registered")
		}
		parentKey := getTokenKey([]byte(name[len(fragments[0])+1:]))
		nsBytes := storage.Get(ctx, append([]byte{prefixName}, parentKey...))
		ns := std.Deserialize(nsBytes.([]byte)).(NameState)
		ns.checkAdmin()

		if conflict := getParentConflictingRecord(ctx, name, fragments); len(conflict) != 0 {
			panic("parent domain has conflicting records: " + conflict)
		}
	}

	if !isValid(owner) {
		panic("invalid owner")
	}
	if !runtime.CheckWitness(owner) {
		panic("not witnessed by owner")
	}
	price := GetPrice(len(fragments[0]))
	if price < 0 {
		checkCommittee()
	} else {
		runtime.BurnGas(price)
	}

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
		it := storage.Find(ctx, append([]byte{prefixRecord}, tokenKey...), storage.KeysOnly)
		for iterator.Next(it) {
			storage.Delete(ctx, iterator.Value(it))
		}
	} else {
		updateTotalSupply(ctx, +1)
	}
	ns := NameState{
		Owner:      owner,
		Name:       name,
		Expiration: runtime.GetTime() + expire*millisecondsInSecond,
	}
	putNameStateWithKey(ctx, tokenKey, ns)
	putSoaRecord(ctx, name, email, refresh, retry, expire, ttl)
	updateBalance(ctx, []byte(name), owner, +1)
	postTransfer(oldOwner, owner, []byte(name), nil)
	return true
}

// UpdateSOA updates soa record.
func UpdateSOA(name, email string, refresh, retry, expire, ttl int) {
	if len(name) > maxDomainNameLength {
		panic("invalid domain name format")
	}
	ctx := storage.GetContext()
	ns := getNameState(ctx, []byte(name))
	ns.checkAdmin()
	putSoaRecord(ctx, name, email, refresh, retry, expire, ttl)
}

// putSoaRecord stores SOA domain record.
func putSoaRecord(ctx storage.Context, name, email string, refresh, retry, expire, ttl int) {
	data := name + " " + email + " " +
		std.Itoa(runtime.GetTime(), 10) + " " +
		std.Itoa(refresh, 10) + " " +
		std.Itoa(retry, 10) + " " +
		std.Itoa(expire, 10) + " " +
		std.Itoa(ttl, 10)
	tokenId := []byte(tokenIDFromName(ctx, name))
	putRecord(ctx, tokenId, name, SOA, 0, data)
}

// updateSoaSerial updates serial of the corresponding SOA domain record.
func updateSoaSerial(ctx storage.Context, tokenId []byte) {
	recordKey := getRecordKey(tokenId, string(tokenId), SOA, 0)

	recBytes := storage.Get(ctx, recordKey)
	if recBytes == nil {
		panic("SOA record not found")
	}
	rec := std.Deserialize(recBytes.([]byte)).(RecordState)

	split := std.StringSplitNonEmpty(rec.Data, " ")
	if len(split) != 7 {
		panic("corrupted SOA record format")
	}
	split[2] = std.Itoa(runtime.GetTime(), 10) // update serial
	rec.Data = split[0] + " " + split[1] + " " +
		split[2] + " " + split[3] + " " +
		split[4] + " " + split[5] + " " +
		split[6]

	recBytes = std.Serialize(rec)
	storage.Put(ctx, recordKey, recBytes)
}

// RenewDefault increases domain expiration date up to 1 year.
func RenewDefault(name string) int {
	return Renew(name, 1)
}

// Renew increases domain expiration date up to the specified amount of years.
func Renew(name string, years int64) int {
	if years < 1 || years > 10 {
		panic("invalid renewal period value")
	}
	if len(name) > maxDomainNameLength {
		panic("invalid domain name format")
	}
	fragments := splitAndCheck(name, true)
	if fragments == nil {
		panic("invalid domain name format")
	}
	price := GetPrice(len(fragments[0]))
	if price < 0 {
		checkCommittee()
	} else {
		runtime.BurnGas(price * int(years))
	}

	ctx := storage.GetContext()
	ns := getNameState(ctx, []byte(name))
	ns.Expiration += int(millisecondsInYear * years)
	if ns.Expiration > int(runtime.GetTime())+millisecondsInTenYears {
		panic("10 years of expiration period at max is allowed")
	}
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

// SetRecord updates record of the specified type and ID.
func SetRecord(name string, typ RecordType, id byte, data string) {
	ctx := storage.GetContext()
	tokenID := checkRecord(ctx, name, typ, data)
	recordKey := getRecordKey(tokenID, name, typ, id)
	recBytes := storage.Get(ctx, recordKey)
	if recBytes == nil {
		panic("unknown record")
	}
	putRecord(ctx, tokenID, name, typ, id, data)
	updateSoaSerial(ctx, tokenID)
}

// AddRecord adds new record of the specified type to the provided domain.
func AddRecord(name string, typ RecordType, data string) {
	ctx := storage.GetContext()
	tokenID := checkRecord(ctx, name, typ, data)
	recordsPrefix := getRecordsByTypePrefix(tokenID, name, typ)
	var id byte
	records := storage.Find(ctx, recordsPrefix, storage.ValuesOnly|storage.DeserializeValues)
	for iterator.Next(records) {
		r := iterator.Value(records).(RecordState)
		if r.Name == name && r.Type == typ && r.Data == data {
			panic("record already exists")
		}
		id++
	}
	if id > maxRecordID {
		panic("maximum number of records reached")
	}
	if typ == CNAME && id != 0 {
		panic("multiple CNAME records")
	}
	putRecord(ctx, tokenID, name, typ, id, data)
	updateSoaSerial(ctx, tokenID)
}

// checkRecord performs record validness check and returns token ID.
func checkRecord(ctx storage.Context, name string, typ RecordType, data string) []byte {
	tokenID := []byte(tokenIDFromName(ctx, name))
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
	ns := getNameState(ctx, tokenID)
	ns.checkAdmin()
	return tokenID
}

// GetRecords returns domain records of the specified type if they exist or an empty
// array if not.
func GetRecords(name string, typ RecordType) []string {
	ctx := storage.GetReadOnlyContext()
	tokenID := []byte(tokenIDFromName(ctx, name))
	_ = getNameState(ctx, tokenID) // ensure not expired
	return getRecordsByType(ctx, tokenID, name, typ)
}

// DeleteRecords removes all domain records with the specified type.
func DeleteRecords(name string, typ RecordType) {
	if typ == SOA {
		panic("forbidden to delete SOA record")
	}
	ctx := storage.GetContext()
	tokenID := []byte(tokenIDFromName(ctx, name))
	ns := getNameState(ctx, tokenID)
	ns.checkAdmin()
	recordsPrefix := getRecordsByTypePrefix(tokenID, name, typ)
	records := storage.Find(ctx, recordsPrefix, storage.KeysOnly)
	for iterator.Next(records) {
		key := iterator.Value(records).(string)
		storage.Delete(ctx, key)
	}
	updateSoaSerial(ctx, tokenID)
}

// Resolve resolves given name (not more than three redirects are allowed) to a set
// of domain records.
func Resolve(name string, typ RecordType) []string {
	ctx := storage.GetReadOnlyContext()
	res := []string{}
	return resolve(ctx, res, name, typ, 2)
}

// GetAllRecords returns an Iterator with RecordState items for given name.
func GetAllRecords(name string) iterator.Iterator {
	ctx := storage.GetReadOnlyContext()
	return getAllRecords(ctx, name)
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
	ns := getNameStateWithKey(ctx, tokenKey)
	fragments := std.StringSplit(string(tokenID), ".")
	if parentExpired(ctx, 1, fragments) {
		panic("parent domain has expired")
	}
	return ns
}

// getNameStateWithKey returns domain name state by the specified token key.
func getNameStateWithKey(ctx storage.Context, tokenKey []byte) NameState {
	nameKey := getNameStateKey(tokenKey)
	nsBytes := storage.Get(ctx, nameKey)
	if nsBytes == nil {
		panic("token not found")
	}
	ns := std.Deserialize(nsBytes.([]byte)).(NameState)
	ns.ensureNotExpired()
	return ns
}

// getNameStateKey returns NameState key for the provided token key.
func getNameStateKey(tokenKey []byte) []byte {
	return append([]byte{prefixName}, tokenKey...)
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

// getRecordsByType returns domain records of the specified type or an empty array if no records found.
func getRecordsByType(ctx storage.Context, tokenId []byte, name string, typ RecordType) []string {
	recordsPrefix := getRecordsByTypePrefix(tokenId, name, typ)
	records := storage.Find(ctx, recordsPrefix, storage.ValuesOnly|storage.DeserializeValues)
	res := []string{} // return empty slice if no records was found.
	for iterator.Next(records) {
		r := iterator.Value(records).(RecordState)
		if r.Type == typ {
			res = append(res, r.Data)
		}
	}
	return res
}

// putRecord puts the specified record to the contract storage without any additional checks.
func putRecord(ctx storage.Context, tokenId []byte, name string, typ RecordType, id byte, data string) {
	recordKey := getRecordKey(tokenId, name, typ, id)
	rs := RecordState{
		Name: name,
		Type: typ,
		Data: data,
		ID:   id,
	}
	recBytes := std.Serialize(rs)
	storage.Put(ctx, recordKey, recBytes)
}

// getRecordKey returns key used to store domain record with the specified type and ID.
// This key always have a single corresponding value.
func getRecordKey(tokenId []byte, name string, typ RecordType, id byte) []byte {
	prefix := getRecordsByTypePrefix(tokenId, name, typ)
	return append(prefix, id)
}

// getRecordsByTypePrefix returns prefix used to store domain records with the
// specified type of different IDs.
func getRecordsByTypePrefix(tokenId []byte, name string, typ RecordType) []byte {
	recordKey := getRecordsPrefix(tokenId, name)
	return append(recordKey, []byte{byte(typ)}...)
}

// getRecordsPrefix returns prefix used to store domain records of different types.
func getRecordsPrefix(tokenId []byte, name string) []byte {
	recordKey := append([]byte{prefixRecord}, getTokenKey(tokenId)...)
	return append(recordKey, getTokenKey([]byte(name))...)
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
// 1. Root domain must start with a letter.
// 2. All other fragments must start and end in a letter or a digit.
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
	for i := 1; i < len(v)-1; i++ {
		if v[i] != '-' && !isAlNum(v[i]) {
			return false
		}
	}
	return isAlNum(v[len(v)-1])
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
				if len(fragments[1]) != 0 {
					return false
				}
				nums[i] = 0
			} else if i == l-1 {
				if len(fragments[i-1]) != 0 {
					return false
				}
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
	if l < 8 && !hasEmpty {
		return false
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
func tokenIDFromName(ctx storage.Context, name string) string {
	fragments := splitAndCheck(name, true)
	if fragments == nil {
		panic("invalid domain name format")
	}
	sum := 0
	for i := 0; i < len(fragments)-1; i++ {
		tokenKey := getTokenKey([]byte(name[sum:]))
		nameKey := getNameStateKey(tokenKey)
		nsBytes := storage.Get(ctx, nameKey)
		if nsBytes != nil {
			ns := std.Deserialize(nsBytes.([]byte)).(NameState)
			if runtime.GetTime() < ns.Expiration {
				return name[sum:]
			}
		}
		sum += len(fragments[i]) + 1
	}
	return name
}

// resolve resolves provided name using record with the specified type and given
// maximum redirections constraint.
func resolve(ctx storage.Context, res []string, name string, typ RecordType, redirect int) []string {
	if redirect < 0 {
		panic("invalid redirect")
	}
	if len(name) == 0 {
		panic("invalid name")
	}
	if name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	records := getAllRecords(ctx, name)
	cname := ""
	for iterator.Next(records) {
		r := iterator.Value(records).(RecordState)
		if r.Type == typ {
			res = append(res, r.Data)
		}
		if r.Type == CNAME {
			cname = r.Data
		}
	}
	if cname == "" || typ == CNAME {
		return res
	}

	// TODO: the line below must be removed from the neofs nns:
	// res = append(res, cname)
	// @roman-khimov, it is done in a separate commit in neofs-contracts repo, is it OK?
	return resolve(ctx, res, cname, typ, redirect-1)
}

// getAllRecords returns iterator over the set of records corresponded with the
// specified name. Records returned are of different types and/or different IDs.
// No keys are returned.
func getAllRecords(ctx storage.Context, name string) iterator.Iterator {
	tokenID := []byte(tokenIDFromName(ctx, name))
	_ = getNameState(ctx, tokenID) // ensure not expired.
	recordsPrefix := getRecordsPrefix(tokenID, name)
	return storage.Find(ctx, recordsPrefix, storage.ValuesOnly|storage.DeserializeValues)
}
