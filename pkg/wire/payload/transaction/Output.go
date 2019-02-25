package transaction

import "github.com/CityOfZion/neo-go/pkg/wire/util"

type Output struct {
	// The NEO asset id used in the transaction.
	AssetID util.Uint256

	// Amount of AssetType send or received.
	Amount int64

	// The address of the remittee.
	ScriptHash util.Uint160
}

func NewOutput(assetID util.Uint256, Amount int64, ScriptHash util.Uint160) *Output {
	return &Output{
		assetID,
		Amount,
		ScriptHash,
	}
}

func (o *Output) Encode(bw *util.BinWriter) {
	bw.Write(o.AssetID)
	bw.Write(o.Amount)
	bw.Write(o.ScriptHash)
}

func (o *Output) Decode(br *util.BinReader) {
	br.Read(&o.AssetID)
	br.Read(&o.Amount)
	br.Read(&o.ScriptHash)
}
