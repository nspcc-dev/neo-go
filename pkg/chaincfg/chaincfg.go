package chaincfg

import (
	"bytes"
	"encoding/hex"

	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
)

// Params are the parameters needed to setup the network
type Params struct {
	GenesisBlock payload.Block
}

//NetParams returns the parameters for the chosen network magic
func NetParams(magic protocol.Magic) (Params, error) {
	switch magic {
	case protocol.MainNet:
		return mainnet()
	//	todo: there have to be different scenarios for different networks
	case protocol.PrivNet:
		return mainnet()
	default:
		return mainnet()
	}
}

//Mainnet returns the parameters needed for mainnet
func mainnet() (Params, error) {
	rawHex := "000000000000000000000000000000000000000000000000000000000000000000000000f41bc036e39b0d6b0579c851c6fde83af802fa4e57bec0bc3365eae3abf43f8065fc8857000000001dac2b7c0000000059e75d652b5d3827bf04c165bbe9ef95cca4bf55010001510400001dac2b7c00000000400000455b7b226c616e67223a227a682d434e222c226e616d65223a22e5b08fe89a81e882a1227d2c7b226c616e67223a22656e222c226e616d65223a22416e745368617265227d5d0000c16ff28623000000da1745e9b549bd0bfa1a569971c77eba30cd5a4b00000000400001445b7b226c616e67223a227a682d434e222c226e616d65223a22e5b08fe89a81e5b881227d2c7b226c616e67223a22656e222c226e616d65223a22416e74436f696e227d5d0000c16ff286230008009f7fd096d37ed2c0e3f7f0cfc924beef4ffceb680000000001000000019b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc50000c16ff28623005fa99d93303775fe50ca119c327759313eccfa1c01000151"
	rawBytes, err := hex.DecodeString(rawHex)
	if err != nil {
		return Params{}, err
	}
	reader := bytes.NewReader(rawBytes)

	block := payload.Block{}
	err = block.Decode(reader)
	if err != nil {
		return Params{}, err
	}

	return Params{
		GenesisBlock: block,
	}, nil
}
