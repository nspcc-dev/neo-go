package state

import (
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// Validator holds the state of a validator.
type Validator struct {
	PublicKey  *keys.PublicKey
	Registered bool
	Votes      util.Fixed8
}

// RegisteredAndHasVotes returns true or false whether Validator is registered and has votes.
func (vs *Validator) RegisteredAndHasVotes() bool {
	return vs.Registered && vs.Votes > util.Fixed8(0)
}

// EncodeBinary encodes Validator to the given BinWriter.
func (vs *Validator) EncodeBinary(bw *io.BinWriter) {
	vs.PublicKey.EncodeBinary(bw)
	bw.WriteLE(vs.Registered)
	bw.WriteLE(vs.Votes)
}

// DecodeBinary decodes Validator from the given BinReader.
func (vs *Validator) DecodeBinary(reader *io.BinReader) {
	vs.PublicKey = &keys.PublicKey{}
	vs.PublicKey.DecodeBinary(reader)
	reader.ReadLE(&vs.Registered)
	reader.ReadLE(&vs.Votes)
}

// GetValidatorsWeightedAverage applies weighted filter based on votes for validator and returns number of validators.
// Get back to it with further investigation in https://github.com/nspcc-dev/neo-go/issues/512.
func GetValidatorsWeightedAverage(validators []*Validator) int {
	return int(weightedAverage(applyWeightedFilter(validators)))
}

// applyWeightedFilter is an implementation of the filter for validators votes.
// C# reference https://github.com/neo-project/neo/blob/41caff115c28d6c7665b2a7ac72967e7ce82e921/neo/Helper.cs#L273
func applyWeightedFilter(validators []*Validator) map[*Validator]float64 {
	var validatorsWithVotes []*Validator
	var amount float64

	weightedVotes := make(map[*Validator]float64)
	start := 0.25
	end := 0.75
	sum := float64(0)
	current := float64(0)

	for _, validator := range validators {
		if validator.Votes > util.Fixed8(0) {
			validatorsWithVotes = append(validatorsWithVotes, validator)
			amount += validator.Votes.FloatValue()
		}
	}

	for _, validator := range validatorsWithVotes {
		if current >= end {
			break
		}
		weight := validator.Votes.FloatValue()
		sum += weight
		old := current
		current = sum / amount

		if current <= start {
			continue
		}

		if old < start {
			if current > end {
				weight = (end - start) * amount
			} else {
				weight = (current - start) * amount
			}
		} else if current > end {
			weight = (end - old) * amount
		}
		weightedVotes[validator] = weight
	}
	return weightedVotes
}

func weightedAverage(weightedVotes map[*Validator]float64) float64 {
	sumWeight := float64(0)
	sumValue := float64(0)
	for vState, weight := range weightedVotes {
		sumWeight += weight
		sumValue += vState.Votes.FloatValue() * weight
	}
	if sumValue == 0 || sumWeight == 0 {
		return 0
	}
	return sumValue / sumWeight
}
