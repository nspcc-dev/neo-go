package result

import (
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/stretchr/testify/require"
)

func testProofWithKey() *ProofWithKey {
	return &ProofWithKey{
		Key: random.Bytes(10),
		Proof: [][]byte{
			random.Bytes(12),
			random.Bytes(0),
			random.Bytes(34),
		},
	}
}

func TestGetProof_MarshalJSON(t *testing.T) {
	t.Run("Good", func(t *testing.T) {
		p := &GetProof{
			Result:  *testProofWithKey(),
			Success: true,
		}
		testserdes.MarshalUnmarshalJSON(t, p, new(GetProof))
	})
	t.Run("Compatibility", func(t *testing.T) {
		js := []byte(`{
      		"proof" : "25ddeb9aa1bfc353c9c54e21dffb470f65d9c22a0662616c616e63654f70000000000000000708fd12020020666eaa8a6e75d43a97d76e72b605c7e05189f0c57ec19d84acdb75810f18239d202c83028ce3d7abcf4e4f95d05fbfdfa5e18bde3a8fbb65a57559d6b5ea09425c2090c40d440744a848e3b407a00e4efb692a957245a1efc9cb8496cb05fd328ee620dd2652bf25dfc3ad5fee7b200ccf3e3ae50772ff8ed58907e4dab8e7d4b2489720d8a5d5ed75b5b0f256d0a2cf5c220b4ddae2a228ef0fc0212b689f3811dfa94620342cc0d73fabd2440ed2cc735a9608391a510e1981b321a9f4258682706adc9620ced036e52f39387b9c58ade7bf8c3ca8959b64d8031d36d9b1c62f3f1c51c7cb2031072c7c801b5c1614dae441383a65344acd238f13db28ff0a39c0626e597f002062552d64c616d8b2a6a93d22936055110c0065728aa2b4fbf4d76b108390b474203322d3c93c741674a307cf6455e77c02ceeda307d4ec23fd809a2a420b4243f82052ab92a9cedc6716ad4c66a8a3e423b195b05bdebde456f992bff48f2561e99720e6379995e7053823b8ba8fb8af9623cf48e89f60c989598445df5e711db42a6f20192894ed637e86561ff6a4b8dea4539dee8bddb2fb20bf4ae3499852985c88b120e0005edd09f2335aa6b59ff4723e1262b2192adaa5e3e56f79e662f07041f04c2033577f3e2c5bb0e58746980a07cdfad2f872e2b9a10bcc27b7c678c85576df8420f0f04180d15b6eaa0c43e62380084c75ad773d790700a7120c6c4da1fc51693000fd720100209648e8f10a5ff4c209009b9a09697babbe1b2150d0948c1970a560282a1bfa4720988af8f34859dd8309bffea0b1dff9c8cef0b9b0d6a1852d40786627729ae7be00206ebf4f1b7861bca041cbb8feca75158511ca43a1810d17e1e3017468e8cef0de20cac93064090a7da09f8202c17d1e6cbb9a16eb43afcb032e80719cbf05b3446d2019b76a10b91fb99ec08814e8108e5490b879fb09a190cb2c129dfd98335bd5de000020b1da1198bacacf2adc0d863929d77c285ce3a26e736203d0c0a69a1312255fb2207ee8aa092f49348bd89f9c4bf004b0bee2241a2d0acfe7b3ce08e414b04a5717205b0dda71eac8a4e4cdc6a7b939748c0a78abb54f2547a780e6df67b25530330f000020fc358fb9d1e0d36461e015ac8e35f97072a9f9e750a3c25722a2b1a858fcb82d203c52c9fac6d4694b351390158334a9166bc3478ceb9bea2b0b244915f918239e20d526344a24ff19ee6a9f5c5beb833f4eb6d51191590350e26fa50b138493473f005200000000000000000000002077c404fec0a4265568951dbd096572787d109fab105213f4f292a5f53ce72fca00000020b8d1c7a386eaba83ce83ee0700d4ca9b86e75d147d670ea05123e438231d895000004801250b090a0a010b0f0c0305030c090c05040e02010d0f0f0b0407000f06050d090c02020a0006202af2097cf9d3f42e49f6b3c3dd254e7cbdab3485b029721cbbbf1ad0455a810852000000000000002055170506f4b18bc573a909b51cb21bdd5d303ec511f6cdfb1c6a1ab8d8a1dad020ee774c1b9fe1d8ea8d05823837d959da48af74f384d52f06c42c9d146c5258e300000000000000000072000000204457a6fe530ee953ad1f9caf63daf7f86719c9986df2d0b6917021eb379800f00020406bfc79da4ba6f37452a679d13cca252585d34f7e94a480b047bad9427f233e00000000201ce15a2373d28e0dc5f2000cf308f155d06f72070a29e5af1528c8f05f29d248000000000000004301200601060c0601060e06030605040f0700000000000000000000000000000000072091b83866bbd7450115b462e8d48601af3c3e9a35e7018d2b98a23e107c15c200090307000410a328e800",
      		"success" : true
   		}`)

		var p GetProof
		require.NoError(t, json.Unmarshal(js, &p))
		require.Equal(t, 8, len(p.Result.Proof))
		for i := range p.Result.Proof { // smoke test that every chunk is correctly encoded node
			r := io.NewBinReaderFromBuf(p.Result.Proof[i])
			n := mpt.DecodeNodeWithType(r)
			require.NoError(t, r.Err)
			require.NotNil(t, n)
		}
	})
}

func TestProofWithKey_EncodeString(t *testing.T) {
	expected := testProofWithKey()
	var actual ProofWithKey
	require.NoError(t, actual.FromString(expected.String()))
	require.Equal(t, expected, &actual)
}

func TestVerifyProof_MarshalJSON(t *testing.T) {
	t.Run("Good", func(t *testing.T) {
		vp := &VerifyProof{random.Bytes(100)}
		testserdes.MarshalUnmarshalJSON(t, vp, new(VerifyProof))
	})
	t.Run("NoValue", func(t *testing.T) {
		vp := new(VerifyProof)
		testserdes.MarshalUnmarshalJSON(t, vp, &VerifyProof{[]byte{1, 2, 3}})
	})
}
