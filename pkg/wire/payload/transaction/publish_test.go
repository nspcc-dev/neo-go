package transaction

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodePublish(t *testing.T) {
	///transaction taken from neo-python; can be found on testnet 5467a1fc8723ceffa8e5ee59399b02eea1df6fbaa53768c6704b90b960d223fa
	// taken from neo-python;
	rawtx := "d000fd3f01746b4c04000000004c04000000004c040000000061681e416e745368617265732e426c6f636b636861696e2e476574486569676874681d416e745368617265732e426c6f636b636861696e2e476574426c6f636b744c0400000000948c6c766b947275744c0402000000936c766b9479744c0400000000948c6c766b9479681d416e745368617265732e4865616465722e47657454696d657374616d70a0744c0401000000948c6c766b947275744c0401000000948c6c766b9479641b004c0400000000744c0402000000948c6c766b947275623000744c0401000000936c766b9479744c0400000000936c766b9479ac744c0402000000948c6c766b947275620300744c0402000000948c6c766b947961748c6c766b946d748c6c766b946d748c6c766b946d746c768c6b946d746c768c6b946d746c768c6b946d6c75660302050001044c6f636b0c312e302d70726576696577310a4572696b205a68616e67126572696b40616e747368617265732e6f7267234c6f636b20796f75722061737365747320756e74696c20612074696d657374616d702e00014e23ac4c4851f93407d4c59e1673171f39859db9e7cac72540cd3cc1ae0cca87000001e72d286979ee6cb1b7e65dfddfb2e384100b8d148e7758de42e4168b71792c6000ebcaaa0d00000067f97110a66136d38badc7b9f88eab013027ce49014140c298da9f06d5687a0bb87ea3bba188b7dcc91b9667ea5cb71f6fdefe388f42611df29be9b2d6288655b9f2188f46796886afc3b37d8b817599365d9e161ecfb62321034b44ed9c8a88fb2497b6b57206cc08edd42c5614bd1fee790e5b795dee0f4e11ac"
	rawtxBytes, _ := hex.DecodeString(rawtx)

	publ := NewPublish(30)

	r := bytes.NewReader(rawtxBytes)
	err := publ.Decode(r)
	assert.Equal(t, nil, err)

	assert.Equal(t, types.Publish, publ.Type)

	buf := new(bytes.Buffer)
	err = publ.Encode(buf)

	assert.Equal(t, nil, err)
	assert.Equal(t, rawtx, hex.EncodeToString(buf.Bytes()))
	assert.Equal(t, "5467a1fc8723ceffa8e5ee59399b02eea1df6fbaa53768c6704b90b960d223fa", publ.Hash.String())

}

func TestEncodeDecodePublish2(t *testing.T) {
	// https://github.com/CityOfZion/neo-python/blob/master/neo/Core/TX/test_transactions.py#L154

	rawtx := "d000a9746b7400936c766b94797451936c766b9479a1633a007400936c766b94797451936c766b94797452936c766b9479617c6554009561746c768c6b946d746c768c6b946d746c768c6b946d6c75667400936c766b94797451936c766b9479617c6525007452936c766b94799561746c768c6b946d746c768c6b946d746c768c6b946d6c7566746b7400936c766b94797451936c766b94799361746c768c6b946d746c768c6b946d6c756600ff09e5919ce5919ce5919c04302e3031037777770377777704656565660001fb9b53e0a87295a94973cd395d64c068c705d662e3965682b2cb36bf67acf7e5000001e72d286979ee6cb1b7e65dfddfb2e384100b8d148e7758de42e4168b71792c60001edc0c1700000050ac4949596f5b62fef7be4d1c3e494e6048ed4a0141402725b8f7e5ada56e5c5e85177cdda9dd6cf738a7f35861fb3413c4e05017125acae5d978cd9e89bda7ab13eb87ba960023cb44d085b9d2b06a88e47cefd6e224232102ff8ac54687f36bbc31a91b730cc385da8af0b581f2d59d82b5cfef824fd271f6ac"
	rawtxBytes, _ := hex.DecodeString(rawtx)

	publ := NewPublish(30)

	r := bytes.NewReader(rawtxBytes)
	err := publ.Decode(r)
	assert.Equal(t, nil, err)

	assert.Equal(t, 0, int(publ.Version))
	assert.Equal(t, types.Publish, publ.Type)

	//@todo: add the following assert once we have the Size calculation in place
	//assert.Equal(t, 402, publ.Size())

	assert.Equal(t, 0, len(publ.Attributes))

	assert.Equal(t, 1, len(publ.Inputs))
	assert.Equal(t, "e5f7ac67bf36cbb2825696e362d605c768c0645d39cd7349a99572a8e0539bfb", publ.Inputs[0].PrevHash.String())

	assert.Equal(t, 1, len(publ.Outputs))
	assert.Equal(t, "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7", publ.Outputs[0].AssetID.String())

	assert.Equal(t, 1, len(publ.Witnesses))
	assert.Equal(t, "402725b8f7e5ada56e5c5e85177cdda9dd6cf738a7f35861fb3413c4e05017125acae5d978cd9e89bda7ab13eb87ba960023cb44d085b9d2b06a88e47cefd6e224", hex.EncodeToString(publ.Witnesses[0].InvocationScript))
	assert.Equal(t, "2102ff8ac54687f36bbc31a91b730cc385da8af0b581f2d59d82b5cfef824fd271f6ac", hex.EncodeToString(publ.Witnesses[0].VerificationScript))

	buf := new(bytes.Buffer)
	err = publ.Encode(buf)

	assert.Equal(t, nil, err)
	assert.Equal(t, rawtx, hex.EncodeToString(buf.Bytes()))
	assert.Equal(t, "514157940a3e31b087891c5e8ed362721f0a7f3dda3f80b7a3fe618d02b7d3d3", publ.Hash.String())

}
