package transaction

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

//go:generate stringer -type=WitnessAction -linecomment

// WitnessAction represents an action to perform in WitnessRule if
// WitnessCondition matches.
type WitnessAction byte

const (
	// WitnessDeny rejects current witness if condition is met.
	WitnessDeny WitnessAction = 0 // Deny
	// WitnessAllow approves current witness if condition is met.
	WitnessAllow WitnessAction = 1 // Allow
)

// WitnessRule represents a single rule for Rules witness scope.
type WitnessRule struct {
	Action    WitnessAction    `json:"action"`
	Condition WitnessCondition `json:"condition"`
}

type witnessRuleAux struct {
	Action    string          `json:"action"`
	Condition json.RawMessage `json:"condition"`
}

// EncodeBinary implements the Serializable interface.
func (w *WitnessRule) EncodeBinary(bw *io.BinWriter) {
	bw.WriteB(byte(w.Action))
	w.Condition.EncodeBinary(bw)
}

// DecodeBinary implements the Serializable interface.
func (w *WitnessRule) DecodeBinary(br *io.BinReader) {
	w.Action = WitnessAction(br.ReadB())
	if br.Err == nil && w.Action != WitnessDeny && w.Action != WitnessAllow {
		br.Err = errors.New("unknown witness rule action")
		return
	}
	w.Condition = DecodeBinaryCondition(br)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (w *WitnessRule) MarshalJSON() ([]byte, error) {
	cond, err := w.Condition.MarshalJSON()
	if err != nil {
		return nil, err
	}
	aux := &witnessRuleAux{
		Action:    w.Action.String(),
		Condition: cond,
	}
	return json.Marshal(aux)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (w *WitnessRule) UnmarshalJSON(data []byte) error {
	aux := &witnessRuleAux{}
	err := json.Unmarshal(data, aux)
	if err != nil {
		return err
	}
	var action WitnessAction
	switch aux.Action {
	case WitnessDeny.String():
		action = WitnessDeny
	case WitnessAllow.String():
		action = WitnessAllow
	default:
		return errors.New("unknown witness rule action")
	}
	cond, err := UnmarshalConditionJSON(aux.Condition)
	if err != nil {
		return err
	}
	w.Action = action
	w.Condition = cond
	return nil
}

// ToStackItem implements Convertible interface.
func (w *WitnessRule) ToStackItem() stackitem.Item {
	return stackitem.NewArray([]stackitem.Item{
		stackitem.NewBigInteger(big.NewInt(int64(w.Action))),
		w.Condition.ToStackItem(),
	})
}

// Copy creates a deep copy of the WitnessRule.
func (w *WitnessRule) Copy() *WitnessRule {
	return &WitnessRule{
		Action:    w.Action,
		Condition: w.Condition.Copy(),
	}
}
