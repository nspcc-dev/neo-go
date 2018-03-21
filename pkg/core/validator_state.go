package core

import (
	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// Validators is a mapping between public keys and ValidatorState.
type Validators map[*crypto.PublicKey]*ValidatorState

// ValidatorState holds the state of a validator.
type ValidatorState struct {
	PublicKey  *crypto.PublicKey
	Registered bool
	Votes      util.Fixed8
}
