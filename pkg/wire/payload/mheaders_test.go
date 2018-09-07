package payload

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
	"github.com/CityOfZion/neo-go/pkg/wire/util/address"
	"github.com/stretchr/testify/assert"
)

func TestNewHeaderMessage(t *testing.T) {
	msgHeaders, err := NewHeadersMessage()

	assert.Equal(t, nil, err)
	assert.Equal(t, 0, len(msgHeaders.Headers))

}

func TestAddAndEncodeHeaders(t *testing.T) {

	// uses block10 from mainnet

	msgHeaders, _ := NewHeadersMessage()

	prevH, _ := util.Uint256DecodeString("005fb74a6de169ce5daf59a114405e5b27238b2489690e3b2a60c14ddfc3b326")
	merkleRoot, _ := util.Uint256DecodeString("ca6d58bcb837472c2f77877e68495b83fd5b714dfe0c8230a525f4511a3239f4")
	invocationScript, _ := hex.DecodeString("4036fdd23248880c1c311bcd97df04fe6d740dc1bf340c26915f0466e31e81c039012eca7a760270389e04b58b99820fe49cf8c24c9afc65d696b4d3f406a1e6b5405172a9b461e68dd399c8716de11d31f7dd2ec3be327c636b024562db6ac5df1cffdbee74c994736fd49803234d2baffbc0054f28ba5ec76494a467b4106955bb4084af7746d269241628c667003e9d39288b190ad5cef218ada625cbba8be411bb153828d8d3634e8f586638e2448425bc5b671be69800392ccbdebc945a5099c7406f6a11824105ecad345e525957053e77fbc0119d6b3fa7f854527e816cfce0d95dac66888e07e8990c95103d8e46124aac16f152e088520d7ec8325e3a2456f840e5b77ef0e3c410b347ccaf8a87516d10b88d436563c80712153273993afc320ec49b638225f58de464a1345e62a564b398939f96f6f4b7cf21b583609f85495a")
	verificationScript, _ := hex.DecodeString("552102486fd15702c4490a26703112a5cc1d0923fd697a33406bd5a1c00e0013b09a7021024c7b7fb6c310fccf1ba33b082519d82964ea93868d676662d4a59ad548df0e7d2102aaec38470f6aad0042c6e877cfd8087d2676b0f516fddd362801b9bd3936399e2103b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c2103b8d9d5771d8f513aa0869b9cc8d50986403b78c6da36890638c3d46a5adce04a2102ca0e27697b9c248f6f16e085fd0061e26f44da85b58ee835c110caa5ec3ba5542102df48f60e8f3e01c48ff40b9b7f1310d7a8b2a193188befe1c2e3df740e89509357ae")
	nextCon, _ := util.Uint160DecodeString(address.ToScriptHash("APyEx5f4Zm4oCHwFWiSTaph1fPBxZacYVR"))

	msgHeaders.AddHeader(&BlockBase{
		Version:       0,
		Index:         10,
		PrevHash:      prevH.Reverse(),
		MerkleRoot:    merkleRoot.Reverse(),
		Timestamp:     1476647551,
		ConsensusData: 0xc0f0280216ff14bf,
		NextConsensus: nextCon,
		Witness:       transaction.Witness{invocationScript, verificationScript},
	})

	assert.Equal(t, 1, len(msgHeaders.Headers))

	err := msgHeaders.Headers[0].createHash()
	assert.Equal(t, nil, err)
	// Hash being correct, automatically verifies that the fields are encoded properly
	assert.Equal(t, "f3c4ec44c07eccbda974f1ee34bc6654ab6d3f22cd89c2e5c593a16d6cc7e6e8", msgHeaders.Headers[0].Hash.String())

}

func TestEncodeDecode(t *testing.T) {
	rawBlockHeaders := "010000000026b3c3df4dc1602a3b0e6989248b23275b5e4014a159af5dce69e16d4ab75f00f439321a51f425a530820cfe4d715bfd835b49687e87772f2c4737b8bc586dca7fda03580a000000bf14ff160228f0c059e75d652b5d3827bf04c165bbe9ef95cca4bf5501fd45014036fdd23248880c1c311bcd97df04fe6d740dc1bf340c26915f0466e31e81c039012eca7a760270389e04b58b99820fe49cf8c24c9afc65d696b4d3f406a1e6b5405172a9b461e68dd399c8716de11d31f7dd2ec3be327c636b024562db6ac5df1cffdbee74c994736fd49803234d2baffbc0054f28ba5ec76494a467b4106955bb4084af7746d269241628c667003e9d39288b190ad5cef218ada625cbba8be411bb153828d8d3634e8f586638e2448425bc5b671be69800392ccbdebc945a5099c7406f6a11824105ecad345e525957053e77fbc0119d6b3fa7f854527e816cfce0d95dac66888e07e8990c95103d8e46124aac16f152e088520d7ec8325e3a2456f840e5b77ef0e3c410b347ccaf8a87516d10b88d436563c80712153273993afc320ec49b638225f58de464a1345e62a564b398939f96f6f4b7cf21b583609f85495af1552102486fd15702c4490a26703112a5cc1d0923fd697a33406bd5a1c00e0013b09a7021024c7b7fb6c310fccf1ba33b082519d82964ea93868d676662d4a59ad548df0e7d2102aaec38470f6aad0042c6e877cfd8087d2676b0f516fddd362801b9bd3936399e2103b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c2103b8d9d5771d8f513aa0869b9cc8d50986403b78c6da36890638c3d46a5adce04a2102ca0e27697b9c248f6f16e085fd0061e26f44da85b58ee835c110caa5ec3ba5542102df48f60e8f3e01c48ff40b9b7f1310d7a8b2a193188befe1c2e3df740e89509357ae00"

	var headerMsg HeadersMessage

	rawBlockBytes, _ := hex.DecodeString(rawBlockHeaders)

	r := bytes.NewReader(rawBlockBytes)

	err := headerMsg.DecodePayload(r)

	assert.Equal(t, 1, len(headerMsg.Headers))

	header := headerMsg.Headers[0]
	err = header.createHash()

	assert.Equal(t, "f3c4ec44c07eccbda974f1ee34bc6654ab6d3f22cd89c2e5c593a16d6cc7e6e8", header.Hash.String())

	buf := new(bytes.Buffer)

	err = headerMsg.EncodePayload(buf)

	assert.Equal(t, nil, err)

	assert.Equal(t, hex.EncodeToString(rawBlockBytes), hex.EncodeToString(buf.Bytes()))
}
