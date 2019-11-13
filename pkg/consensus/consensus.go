package consensus

import "github.com/CityOfZion/neo-go/pkg/util"

// cacheMaxCapacity is the default cache capacity taken
// from C# implementation https://github.com/neo-project/neo/blob/master/neo/Ledger/Blockchain.cs#L64
const cacheMaxCapacity = 100

// Service represents consensus instance.
type Service interface {
	OnPayload(p *Payload)
	GetPayload(h util.Uint256) *Payload
}

type service struct {
	cache *relayCache
}

// NewService returns new consensus.Service instance.
func NewService() Service {
	return &service{
		cache: newFIFOCache(cacheMaxCapacity),
	}
}

// OnPayload handles Payload receive.
func (s *service) OnPayload(p *Payload) {
	s.cache.Add(p)
}

// GetPayload returns payload stored in cache.
func (s *service) GetPayload(h util.Uint256) *Payload {
	return s.cache.Get(h)
}
