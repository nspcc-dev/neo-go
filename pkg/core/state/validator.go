package state

import (
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// MaxValidatorsVoted limits the number of validators that one can vote for.
const MaxValidatorsVoted = 1024

// Validator holds the state of a validator.
type Validator struct {
	PublicKey  *keys.PublicKey
	Registered bool
	Votes      util.Fixed8
}

// ValidatorsCount represents votes with particular number of consensus nodes
// for this number to be changeable by the voting system.
type ValidatorsCount [MaxValidatorsVoted]util.Fixed8

// RegisteredAndHasVotes returns true or false whether Validator is registered and has votes.
func (vs *Validator) RegisteredAndHasVotes() bool {
	return vs.Registered && vs.Votes > util.Fixed8(0)
}

// UnregisteredAndHasNoVotes returns true when Validator is not registered and has no votes.
func (vs *Validator) UnregisteredAndHasNoVotes() bool {
	return !vs.Registered && vs.Votes == 0
}

// EncodeBinary encodes Validator to the given BinWriter.
func (vs *Validator) EncodeBinary(bw *io.BinWriter) {
	vs.PublicKey.EncodeBinary(bw)
	bw.WriteBool(vs.Registered)
	vs.Votes.EncodeBinary(bw)
}

// DecodeBinary decodes Validator from the given BinReader.
func (vs *Validator) DecodeBinary(reader *io.BinReader) {
	vs.PublicKey = &keys.PublicKey{}
	vs.PublicKey.DecodeBinary(reader)
	vs.Registered = reader.ReadBool()
	vs.Votes.DecodeBinary(reader)
}

// EncodeBinary encodes ValidatorCount to the given BinWriter.
func (vc *ValidatorsCount) EncodeBinary(w *io.BinWriter) {
	for i := range vc {
		vc[i].EncodeBinary(w)
	}
}

// DecodeBinary decodes ValidatorCount from the given BinReader.
func (vc *ValidatorsCount) DecodeBinary(r *io.BinReader) {
	for i := range vc {
		vc[i].DecodeBinary(r)
	}
}

// GetWeightedAverage returns an average count of validators that's been voted
// for not counting 1/4 of minimum and maximum numbers.
func (vc *ValidatorsCount) GetWeightedAverage() int {
	const (
		lowerThreshold = 0.25
		upperThreshold = 0.75
	)
	var (
		sumWeight, sumValue, overallSum, slidingSum util.Fixed8
		slidingRatio                                float64
	)

	for i := range vc {
		overallSum += vc[i]
	}

	for i := range vc {
		if slidingRatio >= upperThreshold {
			break
		}
		weight := vc[i]
		slidingSum += weight
		previousRatio := slidingRatio
		slidingRatio = slidingSum.FloatValue() / overallSum.FloatValue()

		if slidingRatio <= lowerThreshold {
			continue
		}

		if previousRatio < lowerThreshold {
			if slidingRatio > upperThreshold {
				weight = util.Fixed8FromFloat((upperThreshold - lowerThreshold) * overallSum.FloatValue())
			} else {
				weight = util.Fixed8FromFloat((slidingRatio - lowerThreshold) * overallSum.FloatValue())
			}
		} else if slidingRatio > upperThreshold {
			weight = util.Fixed8FromFloat((upperThreshold - previousRatio) * overallSum.FloatValue())
		}
		sumWeight += weight
		// Votes with N values get stored with N-1 index, thus +1 here.
		sumValue += util.Fixed8(i+1) * weight
	}
	if sumValue == 0 || sumWeight == 0 {
		return 0
	}
	return int(sumValue / sumWeight)
}
