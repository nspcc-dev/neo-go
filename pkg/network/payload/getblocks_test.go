package payload

// TODO: Currently the hashstop is not encoded, therefore this test will fail.
// Need to figure some stuff how to handle this properly.
// - anthdm 04/02/2018

// func TestGetBlocksEncodeDecode(t *testing.T) {
// 	hash, _ := util.Uint256DecodeFromString("d42561e3d30e15be6400b6df2f328e02d2bf6354c41dce433bc57687c82144bf")

// 	start := []util.Uint256{
// 		hash,
// 		sha256.Sum256([]byte("a")),
// 		sha256.Sum256([]byte("b")),
// 		sha256.Sum256([]byte("c")),
// 	}
// 	stop := sha256.Sum256([]byte("d"))

// 	p := NewGetBlocks(start, stop)
// 	buf := new(bytes.Buffer)
// 	if err := p.EncodeBinary(buf); err != nil {
// 		t.Fatal(err)
// 	}

// 	pDecode := &GetBlocks{}
// 	if err := pDecode.DecodeBinary(buf); err != nil {
// 		t.Fatal(err)
// 	}

// 	if !reflect.DeepEqual(p, pDecode) {
// 		t.Fatalf("expecting both getblocks payloads to be equal %v and %v", p, pDecode)
// 	}
// }

// TODO: Currently the hashstop is not encoded, therefore this test will fail.
// Need to figure some stuff how to handle this properly.
// - anthdm 04/02/2018
//
// func TestGetBlocksWithEmptyHashStop(t *testing.T) {
// 	start := []util.Uint256{
// 		sha256.Sum256([]byte("a")),
// 	}
// 	stop := util.Uint256{}

// 	buf := new(bytes.Buffer)
// 	p := NewGetBlocks(start, stop)
// 	if err := p.EncodeBinary(buf); err != nil {
// 		t.Fatal(err)
// 	}

// 	pDecode := &GetBlocks{}
// 	if err := pDecode.DecodeBinary(buf); err != nil {
// 		t.Fatal(err)
// 	}

// 	if !reflect.DeepEqual(p, pDecode) {
// 		t.Fatalf("expecting both getblocks payloads to be equal %v and %v", p, pDecode)
// 	}
// }
