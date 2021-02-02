package stateroot

import (
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
)

// UpdateStateValidators updates list of state validator keys.
func (s *Module) UpdateStateValidators(height uint32, pubs keys.PublicKeys) {
	script, _ := smartcontract.CreateDefaultMultiSigRedeemScript(pubs)
	h := hash.Hash160(script)

	s.mtx.Lock()
	if s.updateValidatorsCb != nil {
		s.updateValidatorsCb(pubs)
	}
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
	for i := len(s.keys) - 1; i >= 0; i-- {
		if s.keys[i].height <= h && (i+1 == len(s.keys) || s.keys[i+1].height < h) {
			return s.keys[i]
		}
	}
	return keyCache{}
}

// GetStateValidators returns current state validators.
func (s *Module) GetStateValidators(height uint32) keys.PublicKeys {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.getKeyCacheForHeight(height).validatorsKeys.Copy()
}
