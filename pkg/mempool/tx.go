package mempool

import (
	"time"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction"
)

// TX is a wrapper struct around a normal tx
// which includes extra information about the TX
// <aligned>
type TX struct {
	ParentTX   *transaction.Transactioner
	Added      time.Time
	Fee        uint64
	Size       uint64
	NumWitness uint8
	Free       bool
}

// Descriptor takes a transaction and puts it into a new TX struct along with metadata
func Descriptor(trans transaction.Transactioner) *TX {

	var desc TX
	desc.ParentTX = &trans
	desc.Fee = getFee(trans.TXOs(), trans.UTXOs())
	desc.Free = desc.Fee != 0
	desc.Added = time.Now()
	desc.Size = uint64(len(trans.Bytes()))

	numWit := len(trans.Witness())
	if numWit > 255 || numWit < 0 { // < 0 should not happen
		numWit = 255
	}
	desc.NumWitness = uint8(numWit)

	return &desc
}

// TODO: need blockchain package complete for fee calculation
// HMM:Could also put the function in the config
func getFee(in []*transaction.Input, out []*transaction.Output) uint64 {
	// query utxo set for inputs, then subtract from out to get fee
	return 0
}
