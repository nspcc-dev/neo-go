package addrmgr

import (
	"fmt"
	"sync"
	"time"

	"github.com/CityOfZion/neo-go/pkg/peer"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
)

const (
	maxTries = 5 // Try to connect five times, if we cannot then we say it is bad
	// We will keep bad addresses, so that we do not attempt to connect to them in the future.
	// nodes at the moment seem to send a large percentage of bad nodes.

	maxFailures = 12 // This will be incremented when we connect to a node and for whatever reason, they keep disconnecting.
	// The number is high because there could be a period of time, where a node is behaving unrepsonsively and
	// we do not want to immmediately put it inside of the bad bucket.

	// This is the maximum amount of addresses that the addrmgr will hold
	maxAllowedAddrs = 2000
)

type addrStats struct {
	tries       uint8
	failures    uint8
	permanent   bool // permanent = inbound, temp = outbound
	lastTried   time.Time
	lastSuccess time.Time
}

type Addrmgr struct {
	addrmtx   sync.RWMutex
	goodAddrs map[*payload.Net_addr]addrStats
	newAddrs  map[*payload.Net_addr]struct{}
	badAddrs  map[*payload.Net_addr]addrStats
	knownList map[string]*payload.Net_addr // this contains all of the addresses in badAddrs, newAddrs and goodAddrs
}

func New() *Addrmgr {

	return &Addrmgr{
		sync.RWMutex{},
		make(map[*payload.Net_addr]addrStats, 100),
		make(map[*payload.Net_addr]struct{}, 100),
		make(map[*payload.Net_addr]addrStats, 100),
		make(map[string]*payload.Net_addr, 100),
	}
}

//AddAddrs will add new add new addresses into the newaddr list
// This is safe for concurrent access.
func (a *Addrmgr) AddAddrs(newAddrs []*payload.Net_addr) {

	newAddrs = removeDuplicates(newAddrs)

	var nas []*payload.Net_addr
	for _, addr := range newAddrs {
		a.addrmtx.Lock()

		if _, ok := a.knownList[addr.IPPort()]; !ok { // filter unique addresses
			nas = append(nas, addr)
		}
		a.addrmtx.Unlock()
	}

	for _, addr := range nas {
		a.addrmtx.Lock()
		a.newAddrs[addr] = struct{}{}
		a.knownList[addr.IPPort()] = addr
		a.addrmtx.Unlock()
	}
}

// Good returns the good addresses.
// A good address is:
// - Known
// - has been successfully connected to in the last week
// - Note: that while users are launching full nodes from their laptops, the one week marker will need to be modified.
func (a *Addrmgr) Good() []payload.Net_addr {

	var goodAddrs []payload.Net_addr

	var oneWeekAgo = time.Now().Add(((time.Hour * 24) * 7) * -1)

	a.addrmtx.RLock()
	// TODO: sort addresses, permanent ones go first
	for addr, stat := range a.goodAddrs {
		if stat.lastTried.Before(oneWeekAgo) {
			continue
		}
		goodAddrs = append(goodAddrs, *addr)

	}
	a.addrmtx.RUnlock()

	return goodAddrs
}

// Unconnected Addrs are addresses in the newAddr List
func (a *Addrmgr) Unconnected() []payload.Net_addr {

	var unconnAddrs []payload.Net_addr

	a.addrmtx.RLock()
	for addr := range a.newAddrs {
		unconnAddrs = append(unconnAddrs, *addr)
	}
	a.addrmtx.RUnlock()
	return unconnAddrs
}

// Bad Addrs are addresses in the badAddr List
// They are put there if we have tried to connect
// to them in the past and it failed.
func (a *Addrmgr) Bad() []payload.Net_addr {

	var badAddrs []payload.Net_addr

	a.addrmtx.RLock()
	for addr := range a.badAddrs {
		badAddrs = append(badAddrs, *addr)
	}
	a.addrmtx.RUnlock()
	return badAddrs
}

// FetchMoreAddresses will return true if the number of good + NewAddrs are less than 100
// This number is kept low because at the moment, there are not a lot of Good Addresses
// Tests have shown that at most there are < 100
func (a *Addrmgr) FetchMoreAddresses() bool {

	var numOfKnownAddrs int

	a.addrmtx.RLock()
	numOfKnownAddrs = len(a.knownList)
	a.addrmtx.RUnlock()

	return numOfKnownAddrs < maxAllowedAddrs
}

// ConnectionComplete will be called by the server when we have done the handshake
// and Not when we have successfully Connected with net.Conn
// It is to tell the AddrMgr that we have connected to a peer
// This peer should already be known to the AddrMgr because
// We send it to the Connmgr
func (a *Addrmgr) ConnectionComplete(addressport string, inbound bool) {
	a.addrmtx.Lock()
	defer a.addrmtx.Unlock()

	// if addrmgr does not know this, then we just return
	if _, ok := a.knownList[addressport]; !ok {
		fmt.Println("Connected to an unknown address:port ", addressport)
		return
	}

	na := a.knownList[addressport]

	// move it from newAddrs List to GoodAddr List
	stats := a.goodAddrs[na]
	stats.lastSuccess = time.Now()
	stats.lastTried = time.Now()
	stats.permanent = inbound
	stats.tries++
	a.goodAddrs[na] = stats

	// remove it from new and bad, if it was there
	delete(a.newAddrs, na)
	delete(a.badAddrs, na)

	// Unfortunately, Will have a lot of bad nodes because of people connecting to nodes
	// from their laptop. TODO Sort function in good will mitigate.

}

// Failed will be called by ConnMgr
// It is used to tell the AddrMgr that they had tried connecting an address an have failed
// This is concurrent safe
func (a *Addrmgr) Failed(addressport string) {
	a.addrmtx.Lock()
	defer a.addrmtx.Unlock()

	// if addrmgr does not know this, then we just return
	if _, ok := a.knownList[addressport]; !ok {
		fmt.Println("Connected to an unknown address:port ", addressport)
		return
	}

	na := a.knownList[addressport]

	// HMM: logic here could be simpler if we make it one list instead

	if stats, ok := a.badAddrs[na]; ok {
		stats.lastTried = time.Now()
		stats.failures++
		stats.tries++
		if float32(stats.failures)/float32(stats.tries) > 0.8 && stats.tries > 5 {
			delete(a.badAddrs, na)
			return
		}
		a.badAddrs[na] = stats
		return
	}

	if stats, ok := a.goodAddrs[na]; ok {
		fmt.Println("We have a good addr", na.IPPort())
		stats.lastTried = time.Now()
		stats.failures++
		stats.tries++
		if float32(stats.failures)/float32(stats.tries) > 0.5 && stats.tries > 10 {
			delete(a.goodAddrs, na)
			a.badAddrs[na] = stats
			return
		}
		a.goodAddrs[na] = stats
		return
	}

	if _, ok := a.newAddrs[na]; ok {
		delete(a.newAddrs, na)
		a.badAddrs[na] = addrStats{}
	}

}

//OnAddr is the responder for the Config file
// when a OnAddr is received by a peer
func (a *Addrmgr) OnAddr(p *peer.Peer, msg *payload.AddrMessage) {
	a.AddAddrs(msg.AddrList)
}

// OnGetAddr Is called when a peer sends a request for the addressList
// We will give them the best addresses we have from good.
func (a *Addrmgr) OnGetAddr(p *peer.Peer, msg *payload.GetAddrMessage) {
	a.addrmtx.RLock()
	defer a.addrmtx.RUnlock()
	// Push most recent peers to peer
	addrMsg, err := payload.NewAddrMessage()
	if err != nil {
		return
	}
	for _, add := range a.Good() {
		addrMsg.AddNetAddr(&add)
	}

	p.Write(addrMsg)
}

// https://www.dotnetperls.com/duplicates-go
func removeDuplicates(elements []*payload.Net_addr) []*payload.Net_addr {

	encountered := map[string]bool{}
	result := []*payload.Net_addr{}

	for _, element := range elements {
		if encountered[element.IPPort()] == true {
			// Do not add duplicate.
		} else {
			// Record this element as an encountered element.
			encountered[element.IPPort()] = true
			// Append to result slice.
			result = append(result, element)
		}
	}
	// Return the new slice.
	return result
}
