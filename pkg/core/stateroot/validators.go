package stateroot

import (
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
)

// UpdateStateValidators updates list of state validator keys.
func (s *Module) UpdateStateValidators(height uint32, pubs keys.PublicKeys) {
	script, _ := smartcontract.CreateDefaultMultiSigRedeemScript(pubs)
	h := hash.Hash160(script)

	s.mtx.Lock()
	kc := s.getKeyCacheForHeight(height)
	if kc.validatorsHash != h {
		s.keys = append(s.keys, keyCache{
			height:           height,
			validatorsKeys:   pubs,
			validatorsHash:   h,
			validatorsScript: script,
		})
	}
	s.mtx.Unlock()
}

func (s *Module) getKeyCacheForHeight(h uint32) keyCache {
	index := sort.Search(len(s.keys), func(i int) bool {
		return s.keys[i].height >= h
	})
	if index == len(s.keys) {
		return keyCache{}
	}
	return s.keys[index]
}
