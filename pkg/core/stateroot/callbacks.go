package stateroot

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
)

// SetSignAndSendCb sets callback for sending signed root.
func (s *Module) SetSignAndSendCallback(f func(*state.MPTRoot) error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.signAndSendCb = f
}

// SetUpdateValidatorsCallback sets callback for sending signed root.
func (s *Module) SetUpdateValidatorsCallback(f func(keys.PublicKeys)) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.updateValidatorsCb = f
}
