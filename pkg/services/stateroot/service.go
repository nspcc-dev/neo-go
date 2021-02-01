package stateroot

import (
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
)

type (
	// Service represents state root service.
	Service interface {
		blockchainer.StateRoot
		OnPayload(p *payload.Extensible) error
	}

	service struct {
		blockchainer.StateRoot
	}
)

const (
	// Category is message category for extensible payloads.
	Category = "StateService"
)

// New returns new state root service instance using underlying module.
func New(mod blockchainer.StateRoot) (Service, error) {
	return &service{
		StateRoot: mod,
	}, nil
}

// OnPayload implements Service interface.
func (s *service) OnPayload(ep *payload.Extensible) error {
	m := new(Message)
	r := io.NewBinReaderFromBuf(ep.Data)
	m.DecodeBinary(r)
	if r.Err != nil {
		return r.Err
	}
	switch m.Type {
	case RootT:
		sr := m.Payload.(*state.MPTRoot)
		if sr.Index == 0 {
			return nil
		}
		return s.AddStateRoot(sr)
	}
	return nil
}
