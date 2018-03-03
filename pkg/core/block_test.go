package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
)

func TestGenisis(t *testing.T) {
	var (
		rawBlock = "000000000000000000000000000000000000000000000000000000000000000000000000845c34e7c1aed302b1718e914da0c42bf47c476ac4d89671f278d8ab6d27aa3d65fc8857000000001dac2b7c00000000be48d3a3f5d10013ab9ffee489706078714f1ea2010001510400001dac2b7c00000000400000455b7b226c616e67223a227a682d434e222c226e616d65223a22e5b08fe89a81e882a1227d2c7b226c616e67223a22656e222c226e616d65223a22416e745368617265227d5d0000c16ff28623000000da1745e9b549bd0bfa1a569971c77eba30cd5a4b00000000400001445b7b226c616e67223a227a682d434e222c226e616d65223a22e5b08fe89a81e5b881227d2c7b226c616e67223a22656e222c226e616d65223a22416e74436f696e227d5d0000c16ff286230008009f7fd096d37ed2c0e3f7f0cfc924beef4ffceb680000000001000000019b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc50000c16ff2862300be48d3a3f5d10013ab9ffee489706078714f1ea201000151"
		//rawBlockHash = "996e37358dc369912041f966f8c5d8d3a8255ba5dcbd3447f8a82b55db869099"
	)

	rawBlockBytes, err := hex.DecodeString(rawBlock)
	if err != nil {
		t.Fatal(err)
	}

	block := &Block{}
	if err := block.DecodeBinary(bytes.NewReader(rawBlockBytes)); err != nil {
		t.Fatal(err)
	}

	hash, err := block.Hash()
	if err != nil {
		t.Fatal(err)
	}

	t.Log(hash)
}

func TestDecodeBlock(t *testing.T) {
	var (
		rawBlock              = "00000000b7def681f0080262aa293071c53b41fc3146b196067243700b68acd059734fd19543108bf9ddc738cbee2ed1160f153aa0d057f062de0aa3cbb64ba88735c23d43667e59543f050095df82b02e324c5ff3812db982f3b0089a21a278988efeec6a027b2501fd450140113ac66657c2f544e8ad13905fcb2ebaadfef9502cbefb07960fbe56df098814c223dcdd3d0efa0b43a9459e654d948516dcbd8b370f50fbecfb8b411d48051a408500ce85591e516525db24065411f6a88f43de90fa9c167c2e6f5af43bc84e65e5a4bb174bc83a19b6965ff10f476b1b151ae15439a985f33916abc6822b0bb140f4aae522ffaea229987a10d01beec826c3b9a189fe02aa82680581b78f3df0ea4d3f93ca8ea35ffc90f15f7db9017f92fafd9380d9ba3237973cf4313cf626fc40e30e50e3588bd047b39f478b59323868cd50c7ab54355d8245bf0f1988d37528f9bbfc68110cf917debbdbf1f4bdd02cdcccdc3269fdf18a6c727ee54b6934d840e43918dd1ec6123550ec37a513e72b34b2c2a3baa510dec3037cbef2fa9f6ed1e7ccd1f3f6e19d4ce2c0919af55249a970c2685217f75a5589cf9e54dff8449af155210209e7fd41dfb5c2f8dc72eb30358ac100ea8c72da18847befe06eade68cebfcb9210327da12b5c40200e9f65569476bbff2218da4f32548ff43b6387ec1416a231ee821034ff5ceeac41acf22cd5ed2da17a6df4dd8358fcb2bfb1a43208ad0feaab2746b21026ce35b29147ad09e4afe4ec4a7319095f08198fa8babbe3c56e970b143528d2221038dddc06ce687677a53d54f096d2591ba2302068cf123c1f2d75c2dddc542557921039dafd8571a641058ccc832c5e2111ea39b09c0bde36050914384f7a48bce9bf92102d02b1873a0863cd042cc717da31cea0d7cf9db32b74d4c72c01b0011503e2e2257ae01000095df82b000000000"
		rawBlockHash          = "922ba0c0d06afbeec4c50b0541a29153feaa46c5d7304e7bf7f40870d9f3aeb0"
		rawBlockPrevHash      = "d14f7359d0ac680b7043720696b14631fc413bc5713029aa620208f081f6deb7"
		rawBlockIndex         = 343892
		rawBlockTimestamp     = 1501455939
		rawBlockConsensusData = 6866918707944415125
	)

	rawBlockBytes, err := hex.DecodeString(rawBlock)
	if err != nil {
		t.Fatal(err)
	}

	block := &Block{}
	if err := block.DecodeBinary(bytes.NewReader(rawBlockBytes)); err != nil {
		t.Fatal(err)
	}
	if block.Index != uint32(rawBlockIndex) {
		t.Fatalf("expected the index to the block to be %d got %d", rawBlockIndex, block.Index)
	}
	if block.Timestamp != uint32(rawBlockTimestamp) {
		t.Fatalf("expected timestamp to be %d got %d", rawBlockTimestamp, block.Timestamp)
	}
	if block.ConsensusData != uint64(rawBlockConsensusData) {
		t.Fatalf("expected consensus data to be %d got %d", rawBlockConsensusData, block.ConsensusData)
	}
	if block.PrevHash.String() != rawBlockPrevHash {
		t.Fatalf("expected prev block hash to be %s got %s", rawBlockPrevHash, block.PrevHash)
	}
	hash, err := block.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if hash.String() != rawBlockHash {
		t.Fatalf("expected hash of the block to be %s got %s", rawBlockHash, hash)
	}
}

func newBlockBase() BlockBase {
	return BlockBase{
		Version:       0,
		PrevHash:      sha256.Sum256([]byte("a")),
		MerkleRoot:    sha256.Sum256([]byte("b")),
		Timestamp:     999,
		Index:         1,
		ConsensusData: 1111,
		NextConsensus: util.Uint160{},
		Script: &transaction.Witness{
			VerificationScript: []byte{0x0},
			InvocationScript:   []byte{0x1},
		},
	}
}

func TestHashBlockEqualsHashHeader(t *testing.T) {
	base := newBlockBase()
	b := &Block{BlockBase: base}
	head := &Header{BlockBase: base}

	bhash, _ := b.Hash()
	headhash, _ := head.Hash()
	if bhash != headhash {
		t.Fatalf("expected both hashes to be equal %s and %s", bhash, headhash)
	}
}

func TestBlockVerify(t *testing.T) {
	block := &Block{
		BlockBase: newBlockBase(),
		Transactions: []*transaction.Transaction{
			{Type: transaction.MinerTX},
			{Type: transaction.IssueTX},
		},
	}

	if !block.Verify(false) {
		t.Fatal("block should be verified")
	}

	block.Transactions = []*transaction.Transaction{
		{Type: transaction.IssueTX},
		{Type: transaction.MinerTX},
	}

	if block.Verify(false) {
		t.Fatal("block should not by verified")
	}

	block.Transactions = []*transaction.Transaction{
		{Type: transaction.MinerTX},
		{Type: transaction.MinerTX},
	}

	if block.Verify(false) {
		t.Fatal("block should not by verified")
	}
}
