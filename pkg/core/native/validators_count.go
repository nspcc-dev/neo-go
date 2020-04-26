package native

import (
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

// MaxValidatorsVoted limits the number of validators that one can vote for.
const MaxValidatorsVoted = 1024

// ValidatorsCount represents votes with particular number of consensus nodes
// for this number to be changeable by the voting system.
type ValidatorsCount [MaxValidatorsVoted]big.Int

// ValidatorsCountFromBytes converts serialized ValidatorsCount to structure.
func ValidatorsCountFromBytes(b []byte) (*ValidatorsCount, error) {
	vc := new(ValidatorsCount)
	if len(b) == 0 {
		return vc, nil
	}
	r := io.NewBinReaderFromBuf(b)
	vc.DecodeBinary(r)

	if r.Err != nil {
		return nil, r.Err
	}
	return vc, nil
}

// Bytes returns serialized ValidatorsCount.
func (vc *ValidatorsCount) Bytes() []byte {
	w := io.NewBufBinWriter()
	vc.EncodeBinary(w.BinWriter)
	if w.Err != nil {
		panic(w.Err)
	}
	return w.Bytes()
}

// EncodeBinary implements io.Serializable interface.
func (vc *ValidatorsCount) EncodeBinary(w *io.BinWriter) {
	for i := range vc {
		w.WriteVarBytes(emit.IntToBytes(&vc[i]))
	}
}

// DecodeBinary implements io.Serializable interface.
func (vc *ValidatorsCount) DecodeBinary(r *io.BinReader) {
	for i := range vc {
		buf := r.ReadVarBytes()
		if r.Err != nil {
			return
		}
		vc[i] = *emit.BytesToInt(buf)
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
		sumWeight, sumValue, overallSum, slidingSum int64
		slidingRatio                                float64
	)

	for i := range vc {
		overallSum += vc[i].Int64()
	}

	for i := range vc {
		if slidingRatio >= upperThreshold {
			break
		}
		weight := vc[i].Int64()
		slidingSum += weight
		previousRatio := slidingRatio
		slidingRatio = float64(slidingSum) / float64(overallSum)

		if slidingRatio <= lowerThreshold {
			continue
		}

		if previousRatio < lowerThreshold {
			if slidingRatio > upperThreshold {
				weight = int64((upperThreshold - lowerThreshold) * float64(overallSum))
			} else {
				weight = int64((slidingRatio - lowerThreshold) * float64(overallSum))
			}
		} else if slidingRatio > upperThreshold {
			weight = int64((upperThreshold - previousRatio) * float64(overallSum))
		}
		sumWeight += weight
		// Votes with N values get stored with N-1 index, thus +1 here.
		sumValue += (int64(i) + 1) * weight
	}
	if sumValue == 0 || sumWeight == 0 {
		return 0
	}
	return int(sumValue / sumWeight)
}
