package client

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rpcClientTestCase struct {
	name           string
	invoke         func(c *Client) (interface{}, error)
	serverResponse string
	result         func(c *Client) interface{}
	check          func(t *testing.T, c *Client, result interface{})
}

const hexB1 = "00000000e862e7907fc987cd58ddb3abb754aeb8812c9377c45e737a036fe88a622c3b8f301f2e84a86b207270830e7929530ccb841a3df7379fe6f0ac8865b33316839501cdd0847201000001000000abec5362f11e75b6e02e407bb98d63675d14384101fd08010c40ab634ce91590e77b246cee8b204e8a270268ee1ef32434cece73f425a7dbc90f1bed1dbe914edcaa2653167ad170ae10e16a9b2c6b7e0af1f711fb848fbb1b7f0c40232de6ad07ee3846bafa96302d37602349501a556df575e7df0743e45b076d6a0c6c6dd4cad3898f9e8848dd054abd303b229fd12984042f241f0e668f39a0fb0c408b4af43057df189a9d471010b5150bab442040403147c5e502bda38cde3ff8bce803f01245e07e2bfb95d57349c55dcc27e3710b82f2735d0f40eb4342908e330c40cda66f743d4ed8d856f5376953f9169581c668a9370245aef16202ebef9bb3f7f81234be62ec287d701ad7d8bf5042648019af9fe5baa0a8e05d279bfdb1d4c994130c2102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e0c2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd620c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699140b413073b3bb030057040000000000000002000000abec5362f11e75b6e02e407bb98d63675d14384100000000000000003e5f0d0000000000b00400000001abec5362f11e75b6e02e407bb98d63675d14384101590218ddf5050c14316e851039019d39dfc2c37d6c3fee19fd5809870c14abec5362f11e75b6e02e407bb98d63675d14384113c00c087472616e736665720c14897720d8cd76f4f00abfa37c0edd889c208fde9b41627d5b523801fd08010c402d96d8fde4ba266f89bc71ef117361967e0d11ed84cd60942a27bc99994dc58adf36a0e74ce976aac657a87a3c19c38e8ca450c67420046b81d98c60fd8feb040c40b3c15d5d23e0403a36cf559caee2979ca6ef00fe255df0e5c3daac4da051016b41eba42668934cd3308359451bafdd5419d059179fd40859684a3b91388bf9d80c407ac048cf8540b091955a374a0f36dae560c92c0134886507a589edf58b9dfbb4e3dbd5450be34e269d2e5454eb14eb7d6280d6101b4529410f829d37634849be0c403bba4113a687ff8507c1753f8519557531cf9df51ecc20deeb2c2b003ec5a1f7588cdd50b99e40b4f8039bb56c5df7ec9e7d6ea4b02fe23792510da21c7557f394130c2102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e0c2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd620c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699140b413073b3bb0003000000abec5362f11e75b6e02e407bb98d63675d1438410000000000000000de6e0d0000000000b00400000001abec5362f11e75b6e02e407bb98d63675d143841015d0300e87648170000000c14316e851039019d39dfc2c37d6c3fee19fd5809870c14abec5362f11e75b6e02e407bb98d63675d14384113c00c087472616e736665720c143b7d3711c6f0ccf9b1dca903d1bfa1d896f1238c41627d5b523801fd08010c4063fb12aa9f3fb83f6324ea9c7ec11fa9e995b51140f480409d68cf4d625e598b0632d6610602984bfa2f5e5ea9bcc62a0e6d818dd271b38530c0d1b8a71b4e0c0c4013e091eac6f304668d647c5c032fd1020597ea5204545e21c38655a6343d58492118f1231ede91af848af7e1d987d1a8816966f5fc1a7821c6c6f62734267bde0c40daadd04a7a4141d96c58de2d373e672ca071e2b82138ef52df016ac522710385db2ac73743d2fe73061fa5d6cb0ff73a7ec7f0667e4c8bff6aa0d5783128d36e0c40dab85cd87d3f92be9532292bdc6f420b0ecbf2f877c70c6a9921ee0fc900dfc53998cf020a51fa9af3d0608f6a2b9048cea3c0b586485802bbd278b261eee8a494130c2102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e0c2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd620c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699140b413073b3bb"

const hexTxMoveNeo = "0002000000abec5362f11e75b6e02e407bb98d63675d14384100000000000000003e5f0d0000000000b00400000001abec5362f11e75b6e02e407bb98d63675d14384101590218ddf5050c14316e851039019d39dfc2c37d6c3fee19fd5809870c14abec5362f11e75b6e02e407bb98d63675d14384113c00c087472616e736665720c14897720d8cd76f4f00abfa37c0edd889c208fde9b41627d5b523801fd08010c402d96d8fde4ba266f89bc71ef117361967e0d11ed84cd60942a27bc99994dc58adf36a0e74ce976aac657a87a3c19c38e8ca450c67420046b81d98c60fd8feb040c40b3c15d5d23e0403a36cf559caee2979ca6ef00fe255df0e5c3daac4da051016b41eba42668934cd3308359451bafdd5419d059179fd40859684a3b91388bf9d80c407ac048cf8540b091955a374a0f36dae560c92c0134886507a589edf58b9dfbb4e3dbd5450be34e269d2e5454eb14eb7d6280d6101b4529410f829d37634849be0c403bba4113a687ff8507c1753f8519557531cf9df51ecc20deeb2c2b003ec5a1f7588cdd50b99e40b4f8039bb56c5df7ec9e7d6ea4b02fe23792510da21c7557f394130c2102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e0c2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd620c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699140b413073b3bb"

const b1Verbose = `{"id":5,"jsonrpc":"2.0","result":{"size":1681,"nextblockhash":"0xf2afe371a27c9dbac4f4a8ad8eba750898b7c04aa298e64fe9e488e947976045","confirmations":6,"hash":"0xbd178d8d4a28ec082c034f817ce2423221281a31e7e00014dbf732c4053033d2","version":0,"previousblockhash":"0x8f3b2c628ae86f037a735ec477932c81b8ae54b7abb3dd58cd87c97f90e762e8","merkleroot":"0x95831633b36588acf0e69f37f73d1a84cb0c5329790e837072206ba8842e1f30","time":1591366176001,"index":1,"nextconsensus":"AXSvJVzydxXuL9da4GVwK25zdesCrVKkHL","witnesses":[{"invocation":"DECrY0zpFZDneyRs7osgToonAmjuHvMkNM7Oc/Qlp9vJDxvtHb6RTtyqJlMWetFwrhDhapssa34K8fcR+4SPuxt/DEAjLeatB+44Rrr6ljAtN2AjSVAaVW31deffB0PkWwdtagxsbdTK04mPnohI3QVKvTA7Ip/RKYQELyQfDmaPOaD7DECLSvQwV98Ymp1HEBC1FQurRCBAQDFHxeUCvaOM3j/4vOgD8BJF4H4r+5XVc0nFXcwn43ELgvJzXQ9A60NCkI4zDEDNpm90PU7Y2Fb1N2lT+RaVgcZoqTcCRa7xYgLr75uz9/gSNL5i7Ch9cBrX2L9QQmSAGa+f5bqgqOBdJ5v9sdTJ","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBMHOzuw=="}],"consensus_data":{"primary":0,"nonce":"0000000000000457"},"tx":[{"txid":"0x8af9ccb8e7e0f0a73e77b78dc52750e77c50f78b09ecc2f0669c0b459cc7dd89","size":575,"version":0,"nonce":2,"sender":"AXSvJVzydxXuL9da4GVwK25zdesCrVKkHL","sys_fee":"0","net_fee":"0.0087635","valid_until_block":1200,"attributes":[],"cosigners":[{"account":"0x4138145d67638db97b402ee0b6751ef16253ecab","scopes":1}],"script":"0218ddf5050c14316e851039019d39dfc2c37d6c3fee19fd5809870c14abec5362f11e75b6e02e407bb98d63675d14384113c00c087472616e736665720c14897720d8cd76f4f00abfa37c0edd889c208fde9b41627d5b5238","scripts":[{"invocation":"DEAtltj95Lomb4m8ce8Rc2GWfg0R7YTNYJQqJ7yZmU3Fit82oOdM6XaqxleoejwZw46MpFDGdCAEa4HZjGD9j+sEDECzwV1dI+BAOjbPVZyu4pecpu8A/iVd8OXD2qxNoFEBa0HrpCZok0zTMINZRRuv3VQZ0FkXn9QIWWhKO5E4i/nYDEB6wEjPhUCwkZVaN0oPNtrlYMksATSIZQelie31i537tOPb1UUL404mnS5UVOsU631igNYQG0UpQQ+CnTdjSEm+DEA7ukETpof/hQfBdT+FGVV1Mc+d9R7MIN7rLCsAPsWh91iM3VC5nkC0+AObtWxd9+yefW6ksC/iN5JRDaIcdVfz","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBMHOzuw=="}]},{"txid":"0xe7cff9e4820e53232dae619a3e6f57a9430dc240b5ed7b5c0ea2cfee3e90c985","size":579,"version":0,"nonce":3,"sender":"AXSvJVzydxXuL9da4GVwK25zdesCrVKkHL","sys_fee":"0","net_fee":"0.0088035","valid_until_block":1200,"attributes":[],"cosigners":[{"account":"0x4138145d67638db97b402ee0b6751ef16253ecab","scopes":1}],"script":"0300e87648170000000c14316e851039019d39dfc2c37d6c3fee19fd5809870c14abec5362f11e75b6e02e407bb98d63675d14384113c00c087472616e736665720c143b7d3711c6f0ccf9b1dca903d1bfa1d896f1238c41627d5b5238","scripts":[{"invocation":"DEBj+xKqnz+4P2Mk6px+wR+p6ZW1EUD0gECdaM9NYl5ZiwYy1mEGAphL+i9eXqm8xioObYGN0nGzhTDA0binG04MDEAT4JHqxvMEZo1kfFwDL9ECBZfqUgRUXiHDhlWmND1YSSEY8SMe3pGvhIr34dmH0aiBaWb1/Bp4IcbG9ic0JnveDEDardBKekFB2WxY3i03PmcsoHHiuCE471LfAWrFInEDhdsqxzdD0v5zBh+l1ssP9zp+x/BmfkyL/2qg1XgxKNNuDEDauFzYfT+SvpUyKSvcb0ILDsvy+HfHDGqZIe4PyQDfxTmYzwIKUfqa89Bgj2orkEjOo8C1hkhYArvSeLJh7uik","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBMHOzuw=="}]}]}}`

const hexHeader1 = "00000000e862e7907fc987cd58ddb3abb754aeb8812c9377c45e737a036fe88a622c3b8f301f2e84a86b207270830e7929530ccb841a3df7379fe6f0ac8865b33316839501cdd0847201000001000000abec5362f11e75b6e02e407bb98d63675d14384101fd08010c40ab634ce91590e77b246cee8b204e8a270268ee1ef32434cece73f425a7dbc90f1bed1dbe914edcaa2653167ad170ae10e16a9b2c6b7e0af1f711fb848fbb1b7f0c40232de6ad07ee3846bafa96302d37602349501a556df575e7df0743e45b076d6a0c6c6dd4cad3898f9e8848dd054abd303b229fd12984042f241f0e668f39a0fb0c408b4af43057df189a9d471010b5150bab442040403147c5e502bda38cde3ff8bce803f01245e07e2bfb95d57349c55dcc27e3710b82f2735d0f40eb4342908e330c40cda66f743d4ed8d856f5376953f9169581c668a9370245aef16202ebef9bb3f7f81234be62ec287d701ad7d8bf5042648019af9fe5baa0a8e05d279bfdb1d4c994130c2102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e0c2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd620c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699140b413073b3bb00"

const header1Verbose = `{"id":5,"jsonrpc":"2.0","result":{"hash":"0xbd178d8d4a28ec082c034f817ce2423221281a31e7e00014dbf732c4053033d2","size":518,"version":0,"previousblockhash":"0x8f3b2c628ae86f037a735ec477932c81b8ae54b7abb3dd58cd87c97f90e762e8","merkleroot":"0x95831633b36588acf0e69f37f73d1a84cb0c5329790e837072206ba8842e1f30","time":1591366176001,"index":1,"nextconsensus":"AXSvJVzydxXuL9da4GVwK25zdesCrVKkHL","witnesses":[{"invocation":"DECrY0zpFZDneyRs7osgToonAmjuHvMkNM7Oc/Qlp9vJDxvtHb6RTtyqJlMWetFwrhDhapssa34K8fcR+4SPuxt/DEAjLeatB+44Rrr6ljAtN2AjSVAaVW31deffB0PkWwdtagxsbdTK04mPnohI3QVKvTA7Ip/RKYQELyQfDmaPOaD7DECLSvQwV98Ymp1HEBC1FQurRCBAQDFHxeUCvaOM3j/4vOgD8BJF4H4r+5XVc0nFXcwn43ELgvJzXQ9A60NCkI4zDEDNpm90PU7Y2Fb1N2lT+RaVgcZoqTcCRa7xYgLr75uz9/gSNL5i7Ch9cBrX2L9QQmSAGa+f5bqgqOBdJ5v9sdTJ","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBMHOzuw=="}],"confirmations":6,"nextblockhash":"0xf2afe371a27c9dbac4f4a8ad8eba750898b7c04aa298e64fe9e488e947976045"}}`

const txMoveNeoVerbose = `{"id":5,"jsonrpc":"2.0","result":{"blockhash":"0xbd178d8d4a28ec082c034f817ce2423221281a31e7e00014dbf732c4053033d2","confirmations":6,"blocktime":1591366176001,"txid":"0x8af9ccb8e7e0f0a73e77b78dc52750e77c50f78b09ecc2f0669c0b459cc7dd89","size":575,"version":0,"nonce":2,"sender":"AXSvJVzydxXuL9da4GVwK25zdesCrVKkHL","sys_fee":"0","net_fee":"0.0087635","valid_until_block":1200,"attributes":[],"cosigners":[{"account":"0x4138145d67638db97b402ee0b6751ef16253ecab","scopes":1}],"script":"0218ddf5050c14316e851039019d39dfc2c37d6c3fee19fd5809870c14abec5362f11e75b6e02e407bb98d63675d14384113c00c087472616e736665720c14897720d8cd76f4f00abfa37c0edd889c208fde9b41627d5b5238","scripts":[{"invocation":"DEAtltj95Lomb4m8ce8Rc2GWfg0R7YTNYJQqJ7yZmU3Fit82oOdM6XaqxleoejwZw46MpFDGdCAEa4HZjGD9j+sEDECzwV1dI+BAOjbPVZyu4pecpu8A/iVd8OXD2qxNoFEBa0HrpCZok0zTMINZRRuv3VQZ0FkXn9QIWWhKO5E4i/nYDEB6wEjPhUCwkZVaN0oPNtrlYMksATSIZQelie31i537tOPb1UUL404mnS5UVOsU631igNYQG0UpQQ+CnTdjSEm+DEA7ukETpof/hQfBdT+FGVV1Mc+d9R7MIN7rLCsAPsWh91iM3VC5nkC0+AObtWxd9+yefW6ksC/iN5JRDaIcdVfz","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBMHOzuw=="}]}}`

// getResultBlock1 returns data for block number 1 which is used by several tests.
func getResultBlock1() *result.Block {
	binB, err := hex.DecodeString(hexB1)
	if err != nil {
		panic(err)
	}
	b := new(block.Block)
	err = testserdes.DecodeBinary(binB, b)
	if err != nil {
		panic(err)
	}
	b2Hash, err := util.Uint256DecodeStringLE("f2afe371a27c9dbac4f4a8ad8eba750898b7c04aa298e64fe9e488e947976045")
	if err != nil {
		panic(err)
	}
	return &result.Block{
		Block: b,
		BlockMetadata: result.BlockMetadata{
			Size:          1681,
			NextBlockHash: &b2Hash,
			Confirmations: 6,
		},
	}
}

func getTxMoveNeo() *result.TransactionOutputRaw {
	b1 := getResultBlock1()
	txBin, err := hex.DecodeString(hexTxMoveNeo)
	if err != nil {
		panic(err)
	}
	tx := new(transaction.Transaction)
	err = testserdes.DecodeBinary(txBin, tx)
	if err != nil {
		panic(err)
	}
	return &result.TransactionOutputRaw{
		Transaction: tx,
		TransactionMetadata: result.TransactionMetadata{
			Timestamp:     b1.Timestamp,
			Blockhash:     b1.Block.Hash(),
			Confirmations: int(b1.Confirmations),
		},
	}
}

// rpcClientTestCases contains `serverResponse` json data fetched from examples
// published in official C# JSON-RPC API v2.10.3 reference
// (see https://docs.neo.org/docs/en-us/reference/rpc/latest-version/api.html)
var rpcClientTestCases = map[string][]rpcClientTestCase{
	"getapplicationlog": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetApplicationLog(util.Uint256{})
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"txid":"0x17145a039fca704fcdbeb46e6b210af98a1a9e5b9768e46ffc38f71c79ac2521","executions":[{"trigger":"Application","contract":"0xb9fa3b421eb749d5dd585fe1c1133b311a14bcb1","vmstate":"HALT","gas_consumed":"1","stack":[{"type":"Integer","value":1}],"notifications":[]}]}}`,
			result: func(c *Client) interface{} {
				txHash, err := util.Uint256DecodeStringLE("17145a039fca704fcdbeb46e6b210af98a1a9e5b9768e46ffc38f71c79ac2521")
				if err != nil {
					panic(err)
				}
				scriptHash, err := util.Uint160DecodeStringLE("b9fa3b421eb749d5dd585fe1c1133b311a14bcb1")
				if err != nil {
					panic(err)
				}
				return &result.ApplicationLog{
					TxHash: txHash,
					Executions: []result.Execution{
						{
							Trigger:     "Application",
							ScriptHash:  scriptHash,
							VMState:     "HALT",
							GasConsumed: util.Fixed8FromInt64(1),
							Stack:       []smartcontract.Parameter{{Type: smartcontract.IntegerType, Value: int64(1)}},
							Events:      []result.NotificationEvent{},
						},
					},
				}
			},
		},
	},
	"getbestblockhash": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBestBlockHash()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"0x773dd2dae4a9c9275290f89b56e67d7363ea4826dfd4fc13cc01cf73a44b0d0e"}`,
			result: func(c *Client) interface{} {
				result, err := util.Uint256DecodeStringLE("773dd2dae4a9c9275290f89b56e67d7363ea4826dfd4fc13cc01cf73a44b0d0e")
				if err != nil {
					panic(err)
				}
				return result
			},
		},
	},
	"getblock": {
		{
			name: "byIndex_positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"` + hexB1 + `"}`,
			result: func(c *Client) interface{} {
				b := getResultBlock1()
				return b.Block
			},
		},
		{
			name: "byIndex_verbose_positive",
			invoke: func(c *Client) (i interface{}, err error) {
				return c.GetBlockByIndexVerbose(1)
			},
			serverResponse: b1Verbose,
			result: func(c *Client) interface{} {
				return getResultBlock1()
			},
		},
		{
			name: "byHash_positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("d151651e86680a7ecbc87babf3346a42e7bc9974414ce192c9c22ac4f2e9d043")
				if err != nil {
					panic(err)
				}
				return c.GetBlockByHash(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"` + hexB1 + `"}`,
			result: func(c *Client) interface{} {
				b := getResultBlock1()
				return b.Block
			},
		},
		{
			name: "byHash_verbose_positive",
			invoke: func(c *Client) (i interface{}, err error) {
				hash, err := util.Uint256DecodeStringLE("86fe1061140b2ea791b0739fb9732abc6e5e47de4927228a1ac41de3d93eb7cb")
				if err != nil {
					panic(err)
				}
				return c.GetBlockByHashVerbose(hash)
			},
			serverResponse: b1Verbose,
			result: func(c *Client) interface{} {
				return getResultBlock1()
			},
		},
	},
	"getblockcount": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockCount()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":991991}`,
			result: func(c *Client) interface{} {
				return uint32(991991)
			},
		},
	},
	"getblockhash": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHash(1)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"0x4c1e879872344349067c3b1a30781eeb4f9040d3795db7922f513f6f9660b9b2"}`,
			result: func(c *Client) interface{} {
				hash, err := util.Uint256DecodeStringLE("4c1e879872344349067c3b1a30781eeb4f9040d3795db7922f513f6f9660b9b2")
				if err != nil {
					panic(err)
				}
				return hash
			},
		},
	},
	"getblockheader": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("68e4bd688b852e807eef13a0ff7da7b02223e359a35153667e88f9cb4a3b0801")
				if err != nil {
					panic(err)
				}
				return c.GetBlockHeader(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"` + hexHeader1 + `"}`,
			result: func(c *Client) interface{} {
				b := getResultBlock1()
				return b.Header()
			},
		},
		{
			name: "verbose_positive",
			invoke: func(c *Client) (i interface{}, err error) {
				hash, err := util.Uint256DecodeStringLE("cbb73ed9e31dc41a8a222749de475e6ebc2a73b99f73b091a72e0b146110fe86")
				if err != nil {
					panic(err)
				}
				return c.GetBlockHeaderVerbose(hash)
			},
			serverResponse: header1Verbose,
			result: func(c *Client) interface{} {
				b := getResultBlock1()
				return &result.Header{
					Hash:          b.Hash(),
					Size:          518,
					Version:       b.Version,
					NextBlockHash: b.NextBlockHash,
					PrevBlockHash: b.PrevHash,
					MerkleRoot:    b.MerkleRoot,
					Timestamp:     b.Timestamp,
					Index:         b.Index,
					NextConsensus: address.Uint160ToString(b.NextConsensus),
					Witnesses:     []transaction.Witness{b.Script},
					Confirmations: 6,
				}
			},
		},
	},
	"getblocksysfee": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockSysFee(1)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"195500"}`,
			result: func(c *Client) interface{} {
				return util.Fixed8FromInt64(195500)
			},
		},
	},
	"getconnectioncount": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetConnectionCount()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":10}`,
			result: func(c *Client) interface{} {
				return 10
			},
		},
	},
	"getcontractstate": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("dc675afc61a7c0f7b3d2682bf6e1d8ed865a0e5f")
				if err != nil {
					panic(err)
				}
				return c.GetContractState(hash)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"version":0,"hash":"0xdc675afc61a7c0f7b3d2682bf6e1d8ed865a0e5f","script":"5fc56b6c766b00527ac46c766b51527ac46107576f6f6c6f6e676c766b52527ac403574e476c766b53527ac4006c766b54527ac4210354ae498221046c666efebbaee9bd0eb4823469c98e748494a92a71f346b1a6616c766b55527ac46c766b00c3066465706c6f79876c766b56527ac46c766b56c36416006c766b55c36165f2026c766b57527ac462d8016c766b55c36165d801616c766b00c30b746f74616c537570706c79876c766b58527ac46c766b58c36440006168164e656f2e53746f726167652e476574436f6e7465787406737570706c79617c680f4e656f2e53746f726167652e4765746c766b57527ac46270016c766b00c3046e616d65876c766b59527ac46c766b59c36412006c766b52c36c766b57527ac46247016c766b00c30673796d626f6c876c766b5a527ac46c766b5ac36412006c766b53c36c766b57527ac4621c016c766b00c308646563696d616c73876c766b5b527ac46c766b5bc36412006c766b54c36c766b57527ac462ef006c766b00c30962616c616e63654f66876c766b5c527ac46c766b5cc36440006168164e656f2e53746f726167652e476574436f6e746578746c766b51c351c3617c680f4e656f2e53746f726167652e4765746c766b57527ac46293006c766b51c300c36168184e656f2e52756e74696d652e436865636b5769746e657373009c6c766b5d527ac46c766b5dc3640e00006c766b57527ac46255006c766b00c3087472616e73666572876c766b5e527ac46c766b5ec3642c006c766b51c300c36c766b51c351c36c766b51c352c36165d40361527265c9016c766b57527ac4620e00006c766b57527ac46203006c766b57c3616c756653c56b6c766b00527ac4616168164e656f2e53746f726167652e476574436f6e746578746c766b00c3617c680f4e656f2e53746f726167652e4765746165700351936c766b51527ac46168164e656f2e53746f726167652e476574436f6e746578746c766b00c36c766b51c361651103615272680f4e656f2e53746f726167652e507574616168164e656f2e53746f726167652e476574436f6e7465787406737570706c79617c680f4e656f2e53746f726167652e4765746165f40251936c766b52527ac46168164e656f2e53746f726167652e476574436f6e7465787406737570706c796c766b52c361659302615272680f4e656f2e53746f726167652e50757461616c756653c56b6c766b00527ac461516c766b51527ac46168164e656f2e53746f726167652e476574436f6e746578746c766b00c36c766b51c361654002615272680f4e656f2e53746f726167652e507574616168164e656f2e53746f726167652e476574436f6e7465787406737570706c796c766b51c361650202615272680f4e656f2e53746f726167652e50757461516c766b52527ac46203006c766b52c3616c756659c56b6c766b00527ac46c766b51527ac46c766b52527ac4616168164e656f2e53746f726167652e476574436f6e746578746c766b00c3617c680f4e656f2e53746f726167652e4765746c766b53527ac46168164e656f2e53746f726167652e476574436f6e746578746c766b51c3617c680f4e656f2e53746f726167652e4765746c766b54527ac46c766b53c3616576016c766b52c3946c766b55527ac46c766b54c3616560016c766b52c3936c766b56527ac46c766b55c300a2640d006c766b52c300a2620400006c766b57527ac46c766b57c364ec00616168164e656f2e53746f726167652e476574436f6e746578746c766b00c36c766b55c36165d800615272680f4e656f2e53746f726167652e507574616168164e656f2e53746f726167652e476574436f6e746578746c766b51c36c766b56c361659c00615272680f4e656f2e53746f726167652e5075746155c57600135472616e73666572205375636365737366756cc476516c766b00c3c476526c766b51c3c476536c766b52c3c476546168184e656f2e426c6f636b636861696e2e476574486569676874c46168124e656f2e52756e74696d652e4e6f7469667961516c766b58527ac4620e00006c766b58527ac46203006c766b58c3616c756653c56b6c766b00527ac4616c766b00c36c766b51527ac46c766b51c36c766b52527ac46203006c766b52c3616c756653c56b6c766b00527ac461516c766b00c36a527a527ac46c766b51c36c766b52527ac46203006c766b52c3616c7566","parameters":["ByteArray"],"returntype":"ByteArray","name":"Woolong","code_version":"0.9.2","author":"lllwvlvwlll","email":"lllwvlvwlll@gmail.com","description":"GO NEO!!!","properties":{"storage":true,"dynamic_invoke":false}}}`,
			result: func(c *Client) interface{} {
				hash, err := util.Uint160DecodeStringLE("dc675afc61a7c0f7b3d2682bf6e1d8ed865a0e5f")
				if err != nil {
					panic(err)
				}
				script, err := hex.DecodeString("5fc56b6c766b00527ac46c766b51527ac46107576f6f6c6f6e676c766b52527ac403574e476c766b53527ac4006c766b54527ac4210354ae498221046c666efebbaee9bd0eb4823469c98e748494a92a71f346b1a6616c766b55527ac46c766b00c3066465706c6f79876c766b56527ac46c766b56c36416006c766b55c36165f2026c766b57527ac462d8016c766b55c36165d801616c766b00c30b746f74616c537570706c79876c766b58527ac46c766b58c36440006168164e656f2e53746f726167652e476574436f6e7465787406737570706c79617c680f4e656f2e53746f726167652e4765746c766b57527ac46270016c766b00c3046e616d65876c766b59527ac46c766b59c36412006c766b52c36c766b57527ac46247016c766b00c30673796d626f6c876c766b5a527ac46c766b5ac36412006c766b53c36c766b57527ac4621c016c766b00c308646563696d616c73876c766b5b527ac46c766b5bc36412006c766b54c36c766b57527ac462ef006c766b00c30962616c616e63654f66876c766b5c527ac46c766b5cc36440006168164e656f2e53746f726167652e476574436f6e746578746c766b51c351c3617c680f4e656f2e53746f726167652e4765746c766b57527ac46293006c766b51c300c36168184e656f2e52756e74696d652e436865636b5769746e657373009c6c766b5d527ac46c766b5dc3640e00006c766b57527ac46255006c766b00c3087472616e73666572876c766b5e527ac46c766b5ec3642c006c766b51c300c36c766b51c351c36c766b51c352c36165d40361527265c9016c766b57527ac4620e00006c766b57527ac46203006c766b57c3616c756653c56b6c766b00527ac4616168164e656f2e53746f726167652e476574436f6e746578746c766b00c3617c680f4e656f2e53746f726167652e4765746165700351936c766b51527ac46168164e656f2e53746f726167652e476574436f6e746578746c766b00c36c766b51c361651103615272680f4e656f2e53746f726167652e507574616168164e656f2e53746f726167652e476574436f6e7465787406737570706c79617c680f4e656f2e53746f726167652e4765746165f40251936c766b52527ac46168164e656f2e53746f726167652e476574436f6e7465787406737570706c796c766b52c361659302615272680f4e656f2e53746f726167652e50757461616c756653c56b6c766b00527ac461516c766b51527ac46168164e656f2e53746f726167652e476574436f6e746578746c766b00c36c766b51c361654002615272680f4e656f2e53746f726167652e507574616168164e656f2e53746f726167652e476574436f6e7465787406737570706c796c766b51c361650202615272680f4e656f2e53746f726167652e50757461516c766b52527ac46203006c766b52c3616c756659c56b6c766b00527ac46c766b51527ac46c766b52527ac4616168164e656f2e53746f726167652e476574436f6e746578746c766b00c3617c680f4e656f2e53746f726167652e4765746c766b53527ac46168164e656f2e53746f726167652e476574436f6e746578746c766b51c3617c680f4e656f2e53746f726167652e4765746c766b54527ac46c766b53c3616576016c766b52c3946c766b55527ac46c766b54c3616560016c766b52c3936c766b56527ac46c766b55c300a2640d006c766b52c300a2620400006c766b57527ac46c766b57c364ec00616168164e656f2e53746f726167652e476574436f6e746578746c766b00c36c766b55c36165d800615272680f4e656f2e53746f726167652e507574616168164e656f2e53746f726167652e476574436f6e746578746c766b51c36c766b56c361659c00615272680f4e656f2e53746f726167652e5075746155c57600135472616e73666572205375636365737366756cc476516c766b00c3c476526c766b51c3c476536c766b52c3c476546168184e656f2e426c6f636b636861696e2e476574486569676874c46168124e656f2e52756e74696d652e4e6f7469667961516c766b58527ac4620e00006c766b58527ac46203006c766b58c3616c756653c56b6c766b00527ac4616c766b00c36c766b51527ac46c766b51c36c766b52527ac46203006c766b52c3616c756653c56b6c766b00527ac461516c766b00c36a527a527ac46c766b51c36c766b52527ac46203006c766b52c3616c7566")
				if err != nil {
					panic(err)
				}
				return &result.ContractState{
					Version:     0,
					ScriptHash:  hash,
					Script:      script,
					ParamList:   []smartcontract.ParamType{smartcontract.ByteArrayType},
					ReturnType:  smartcontract.ByteArrayType,
					Name:        "Woolong",
					CodeVersion: "0.9.2",
					Author:      "lllwvlvwlll",
					Email:       "lllwvlvwlll@gmail.com",
					Description: "GO NEO!!!",
					Properties: result.Properties{
						HasStorage:       true,
						HasDynamicInvoke: false,
						IsPayable:        false,
					},
				}
			},
		},
	},
	"getnep5balances": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("1aada0032aba1ef6d1f07bbd8bec1d85f5380fb3")
				if err != nil {
					panic(err)
				}
				return c.GetNEP5Balances(hash)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"balance":[{"asset_hash":"a48b6e1291ba24211ad11bb90ae2a10bf1fcd5a8","amount":"50000000000","last_updated_block":251604}],"address":"AY6eqWjsUFCzsVELG7yG72XDukKvC34p2w"}}`,
			result: func(c *Client) interface{} {
				hash, err := util.Uint160DecodeStringLE("a48b6e1291ba24211ad11bb90ae2a10bf1fcd5a8")
				if err != nil {
					panic(err)
				}
				return &result.NEP5Balances{
					Balances: []result.NEP5Balance{{
						Asset:       hash,
						Amount:      "50000000000",
						LastUpdated: 251604,
					}},
					Address: "AY6eqWjsUFCzsVELG7yG72XDukKvC34p2w",
				}
			},
		},
	},
	"getnep5transfers": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP5Transfers("AbHgdBaWEnHkCiLtDZXjhvhaAK2cwFh5pF")
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"sent":[],"received":[{"timestamp":1555651816,"asset_hash":"600c4f5200db36177e3e8a09e9f18e2fc7d12a0f","transfer_address":"AYwgBNMepiv5ocGcyNT4mA8zPLTQ8pDBis","amount":"1000000","block_index":436036,"transfer_notify_index":0,"tx_hash":"df7683ece554ecfb85cf41492c5f143215dd43ef9ec61181a28f922da06aba58"}],"address":"AbHgdBaWEnHkCiLtDZXjhvhaAK2cwFh5pF"}}`,
			result: func(c *Client) interface{} {
				assetHash, err := util.Uint160DecodeStringLE("600c4f5200db36177e3e8a09e9f18e2fc7d12a0f")
				if err != nil {
					panic(err)
				}
				txHash, err := util.Uint256DecodeStringLE("df7683ece554ecfb85cf41492c5f143215dd43ef9ec61181a28f922da06aba58")
				if err != nil {
					panic(err)
				}
				return &result.NEP5Transfers{
					Sent: []result.NEP5Transfer{},
					Received: []result.NEP5Transfer{
						{
							Timestamp:   1555651816,
							Asset:       assetHash,
							Address:     "AYwgBNMepiv5ocGcyNT4mA8zPLTQ8pDBis",
							Amount:      "1000000",
							Index:       436036,
							NotifyIndex: 0,
							TxHash:      txHash,
						},
					},
					Address: "AbHgdBaWEnHkCiLtDZXjhvhaAK2cwFh5pF",
				}
			},
		},
	},
	"getpeers": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetPeers()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"unconnected":[{"address":"172.200.0.1","port":"20333"}],"connected":[{"address":"127.0.0.1","port":"20335"}],"bad":[{"address":"172.200.0.254","port":"20332"}]}}`,
			result: func(c *Client) interface{} {
				return &result.GetPeers{
					Unconnected: result.Peers{
						{
							Address: "172.200.0.1",
							Port:    "20333",
						},
					},
					Connected: result.Peers{
						{
							Address: "127.0.0.1",
							Port:    "20335",
						},
					},
					Bad: result.Peers{
						{
							Address: "172.200.0.254",
							Port:    "20332",
						},
					},
				}
			},
		},
	},
	"getrawmempool": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawMemPool()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":["0x9786cce0dddb524c40ddbdd5e31a41ed1f6b5c8a683c122f627ca4a007a7cf4e"]}`,
			result: func(c *Client) interface{} {
				hash, err := util.Uint256DecodeStringLE("9786cce0dddb524c40ddbdd5e31a41ed1f6b5c8a683c122f627ca4a007a7cf4e")
				if err != nil {
					panic(err)
				}
				return []util.Uint256{hash}
			},
		},
	},
	"getrawtransaction": {
		{
			name: "positive",
			invoke: func(c *Client) (i interface{}, err error) {
				hash, err := util.Uint256DecodeStringLE("ca23bd5df3249836849309ca2afe972bfd288b0a7ae61302c8fd545daa8bffd6")
				if err != nil {
					panic(err)
				}
				return c.GetRawTransaction(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"` + hexTxMoveNeo + `"}`,
			result: func(c *Client) interface{} {
				tx := getTxMoveNeo()
				return tx.Transaction
			},
		},
		{
			name: "verbose_positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("8185b0db7ed77190b93ac8bd44896822cd8f3cfcf702b3f50131e0efd200ef96")
				if err != nil {
					panic(err)
				}
				return c.GetRawTransactionVerbose(hash)
			},
			serverResponse: txMoveNeoVerbose,
			result: func(c *Client) interface{} {
				return getTxMoveNeo()
			},
		},
	},
	"getstorage": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("03febccf81ac85e3d795bc5cbd4e84e907812aa3")
				if err != nil {
					panic(err)
				}
				key, err := hex.DecodeString("5065746572")
				if err != nil {
					panic(err)
				}
				return c.GetStorage(hash, key)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"4c696e"}`,
			result: func(c *Client) interface{} {
				value, err := hex.DecodeString("4c696e")
				if err != nil {
					panic(err)
				}
				return value
			},
		},
	},
	"gettransactionheight": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("cb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2")
				if err != nil {
					panic(err)
				}
				return c.GetTransactionHeight(hash)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":1}`,
			result: func(c *Client) interface{} {
				return uint32(1)
			},
		},
	},
	"getunclaimedgas": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetUnclaimedGas("AGofsxAUDwt52KjaB664GYsqVAkULYvKNt")
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"897299680935"}`,
			result: func(c *Client) interface{} {
				return util.Fixed8(897299680935)
			},
		},
	},
	"getvalidators": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetValidators()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":[{"publickey":"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2","votes":"0","active":true},{"publickey":"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e","votes":"0","active":true},{"publickey":"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699","votes":"0","active":true},{"publickey":"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62","votes":"0","active":true}]}`,
			result:         func(c *Client) interface{} { return []result.Validator{} },
			check: func(t *testing.T, c *Client, uns interface{}) {
				res, ok := uns.([]result.Validator)
				require.True(t, ok)
				assert.Equal(t, 4, len(res))
			},
		},
	},
	"getversion": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetVersion()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"tcp_port":20332,"nonce":2153672787,"useragent":"/NEO-GO:0.73.1-pre-273-ge381358/"}}`,
			result: func(c *Client) interface{} {
				return &result.Version{
					Port:      uint16(20332),
					Nonce:     2153672787,
					UserAgent: "/NEO-GO:0.73.1-pre-273-ge381358/",
				}
			},
		},
	},
	"invokefunction": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("91b83e96f2a7c4fdf0c1688441ec61986c7cae26")
				if err != nil {
					panic(err)
				}
				return c.InvokeFunction("af7c7328eee5a275a3bcaee2bf0cf662b5e739be", "balanceOf", []smartcontract.Parameter{
					{
						Type:  smartcontract.Hash160Type,
						Value: hash,
					},
				})
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"script":"1426ae7c6c9861ec418468c1f0fdc4a7f2963eb89151c10962616c616e63654f6667be39e7b562f60cbfe2aebca375a2e5ee28737caf","state":"HALT","gas_consumed":"0.311","stack":[{"type":"ByteArray","value":"JivsCEQy"}],"tx":"d101361426ae7c6c9861ec418468c1f0fdc4a7f2963eb89151c10962616c616e63654f6667be39e7b562f60cbfe2aebca375a2e5ee28737caf000000000000000000000000"}}`,
			result: func(c *Client) interface{} {
				bytes, err := hex.DecodeString("262bec084432")
				if err != nil {
					panic(err)
				}
				return &result.Invoke{
					State:       "HALT",
					GasConsumed: "0.311",
					Script:      "1426ae7c6c9861ec418468c1f0fdc4a7f2963eb89151c10962616c616e63654f6667be39e7b562f60cbfe2aebca375a2e5ee28737caf",
					Stack: []smartcontract.Parameter{
						{
							Type:  smartcontract.ByteArrayType,
							Value: bytes,
						},
					},
				}
			},
		},
	},
	"invokescript": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeScript("00046e616d656724058e5e1b6008847cd662728549088a9ee82191")
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"script":"00046e616d656724058e5e1b6008847cd662728549088a9ee82191","state":"HALT","gas_consumed":"0.161","stack":[{"type":"ByteArray","value":"TkVQNSBHQVM="}],"tx":"d1011b00046e616d656724058e5e1b6008847cd662728549088a9ee82191000000000000000000000000"}}`,
			result: func(c *Client) interface{} {
				bytes, err := hex.DecodeString("4e45503520474153")
				if err != nil {
					panic(err)
				}
				return &result.Invoke{
					State:       "HALT",
					GasConsumed: "0.161",
					Script:      "00046e616d656724058e5e1b6008847cd662728549088a9ee82191",
					Stack: []smartcontract.Parameter{
						{
							Type:  smartcontract.ByteArrayType,
							Value: bytes,
						},
					},
				}
			},
		},
	},
	"sendrawtransaction": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.SendRawTransaction(transaction.New([]byte{byte(opcode.PUSH1)}, 0))
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":true}`,
			result: func(c *Client) interface{} {
				// no error expected
				return nil
			},
		},
	},
	"submitblock": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.SubmitBlock(block.Block{
					Base:         block.Base{},
					Transactions: nil,
					Trimmed:      false,
				})
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":true}`,
			result: func(c *Client) interface{} {
				// no error expected
				return nil
			},
		},
	},
	"validateaddress": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.ValidateAddress("AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i")
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"address":"AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i","isvalid":true}}`,
			result: func(c *Client) interface{} {
				// no error expected
				return nil
			},
		},
	},
}

type rpcClientErrorCase struct {
	name   string
	invoke func(c *Client) (interface{}, error)
}

var rpcClientErrorCases = map[string][]rpcClientErrorCase{
	`{"jsonrpc":"2.0","id":1,"result":"not-a-hex-string"}`: {
		{
			name: "getblock_not_a_hex_response",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
		},
		{
			name: "getblockheader_not_a_hex_response",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetBlockHeader(hash)
			},
		},
		{
			name: "getrawtransaction_not_a_hex_response",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetRawTransaction(hash)
			},
		},
		{
			name: "getstorage_not_a_hex_response",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("03febccf81ac85e3d795bc5cbd4e84e907812aa3")
				if err != nil {
					panic(err)
				}
				key, err := hex.DecodeString("5065746572")
				if err != nil {
					panic(err)
				}
				return c.GetStorage(hash, key)
			},
		},
	},
	`{"jsonrpc":"2.0","id":1,"result":"01"}`: {
		{
			name: "getblock_decodebin_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
		},
		{
			name: "getheader_decodebin_err",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetBlockHeader(hash)
			},
		},
		{
			name: "getrawtransaction_decodebin_err",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetRawTransaction(hash)
			},
		},
	},
	`{"jsonrpc":"2.0","id":1,"result":false}`: {
		{
			name: "sendrawtransaction_bad_server_answer",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.SendRawTransaction(transaction.New([]byte{byte(opcode.PUSH1)}, 0))
			},
		},
		{
			name: "submitblock_bad_server_answer",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.SubmitBlock(block.Block{
					Base:         block.Base{},
					Transactions: nil,
					Trimmed:      false,
				})
			},
		},
		{
			name: "validateaddress_bad_server_answer",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.ValidateAddress("AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i")
			},
		},
	},
	`{"id":1,"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params"}}`: {
		{
			name: "getapplicationlog_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetApplicationLog(util.Uint256{})
			},
		},
		{
			name: "getbestblockhash_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBestBlockHash()
			},
		},
		{
			name: "getblock_byindex_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
		},
		{
			name: "getblock_byindex_verbose_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndexVerbose(1)
			},
		},
		{
			name: "getblock_byhash_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByHash(util.Uint256{})
			},
		},
		{
			name: "getblock_byhash_verbose_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByHashVerbose(util.Uint256{})
			},
		},
		{
			name: "getblockhash_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHash(0)
			},
		},
		{
			name: "getblockheader_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHeader(util.Uint256{})
			},
		},
		{
			name: "getblockheader_verbose_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHeaderVerbose(util.Uint256{})
			},
		},
		{
			name: "getblocksysfee_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockSysFee(1)
			},
		},
		{
			name: "getconnectioncount_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetConnectionCount()
			},
		},
		{
			name: "getcontractstate_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetContractState(util.Uint160{})
			},
		},
		{
			name: "getnep5balances_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP5Balances(util.Uint160{})
			},
		},
		{
			name: "getnep5transfers_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP5Transfers("")
			},
		},
		{
			name: "getrawtransaction_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawTransaction(util.Uint256{})
			},
		},
		{
			name: "getrawtransaction_verbose_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawTransactionVerbose(util.Uint256{})
			},
		},
		{
			name: "getstorage_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetStorage(util.Uint160{}, []byte{})
			},
		},
		{
			name: "gettransactionheight_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetTransactionHeight(util.Uint256{})
			},
		},
		{
			name: "getunclaimedgas_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetUnclaimedGas("")
			},
		},
		{
			name: "invokefunction_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeFunction("", "", []smartcontract.Parameter{})
			},
		},
		{
			name: "invokescript_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeScript("")
			},
		},
		{
			name: "sendrawtransaction_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.SendRawTransaction(&transaction.Transaction{})
			},
		},
		{
			name: "submitblock_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.SubmitBlock(block.Block{})
			},
		},
		{
			name: "validateaddress_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.ValidateAddress("")
			},
		},
	},
	`{}`: {
		{
			name: "getapplicationlog_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetApplicationLog(util.Uint256{})
			},
		},
		{
			name: "getbestblockhash_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBestBlockHash()
			},
		},
		{
			name: "getblock_byindex_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
		},
		{
			name: "getblock_byindex_verbose_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndexVerbose(1)
			},
		},
		{
			name: "getblock_byhash_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByHash(util.Uint256{})
			},
		},
		{
			name: "getblock_byhash_verbose_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByHashVerbose(util.Uint256{})
			},
		},
		{
			name: "getblockcount_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockCount()
			},
		},
		{
			name: "getblockhash_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHash(1)
			},
		},
		{
			name: "getblockheader_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHeader(util.Uint256{})
			},
		},
		{
			name: "getblockheader_verbose_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHeaderVerbose(util.Uint256{})
			},
		},
		{
			name: "getblocksysfee_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockSysFee(1)
			},
		},
		{
			name: "getconnectioncount_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetConnectionCount()
			},
		},
		{
			name: "getcontractstate_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetContractState(util.Uint160{})
			},
		},
		{
			name: "getnep5balances_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP5Balances(util.Uint160{})
			},
		},
		{
			name: "getnep5transfers_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP5Transfers("")
			},
		},
		{
			name: "getpeers_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetPeers()
			},
		},
		{
			name: "getrawmempool_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawMemPool()
			},
		},
		{
			name: "getrawtransaction_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawTransaction(util.Uint256{})
			},
		},
		{
			name: "getrawtransaction_verbose_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawTransactionVerbose(util.Uint256{})
			},
		},
		{
			name: "getstorage_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetStorage(util.Uint160{}, []byte{})
			},
		},
		{
			name: "gettransactionheight_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetTransactionHeight(util.Uint256{})
			},
		},
		{
			name: "getunclaimedgas_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetUnclaimedGas("")
			},
		},
		{
			name: "getvalidators_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetValidators()
			},
		},
		{
			name: "getversion_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetVersion()
			},
		},
		{
			name: "invokefunction_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeFunction("", "", []smartcontract.Parameter{})
			},
		},
		{
			name: "invokescript_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeScript("")
			},
		},
		{
			name: "sendrawtransaction_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.SendRawTransaction(transaction.New([]byte{byte(opcode.PUSH1)}, 0))
			},
		},
		{
			name: "submitblock_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.SubmitBlock(block.Block{
					Base:         block.Base{},
					Transactions: nil,
					Trimmed:      false,
				})
			},
		},
		{
			name: "validateaddress_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.ValidateAddress("")
			},
		},
	},
}

func TestRPCClients(t *testing.T) {
	t.Run("Client", func(t *testing.T) {
		testRPCClient(t, func(ctx context.Context, endpoint string, opts Options) (*Client, error) {
			return New(ctx, endpoint, opts)
		})
	})
	t.Run("WSClient", func(t *testing.T) {
		testRPCClient(t, func(ctx context.Context, endpoint string, opts Options) (*Client, error) {
			wsc, err := NewWS(ctx, httpURLtoWS(endpoint), opts)
			require.NoError(t, err)
			return &wsc.Client, nil
		})
	})
}

func testRPCClient(t *testing.T, newClient func(context.Context, string, Options) (*Client, error)) {
	for method, testBatch := range rpcClientTestCases {
		t.Run(method, func(t *testing.T) {
			for _, testCase := range testBatch {
				t.Run(testCase.name, func(t *testing.T) {
					srv := initTestServer(t, testCase.serverResponse)
					defer srv.Close()

					endpoint := srv.URL
					opts := Options{}
					c, err := newClient(context.TODO(), endpoint, opts)
					if err != nil {
						t.Fatal(err)
					}

					actual, err := testCase.invoke(c)
					assert.NoError(t, err)

					expected := testCase.result(c)
					if testCase.check == nil {
						assert.Equal(t, expected, actual)
					} else {
						testCase.check(t, c, actual)
					}
				})
			}
		})
	}
	for serverResponse, testBatch := range rpcClientErrorCases {
		srv := initTestServer(t, serverResponse)
		defer srv.Close()

		endpoint := srv.URL
		opts := Options{}
		c, err := newClient(context.TODO(), endpoint, opts)
		if err != nil {
			t.Fatal(err)
		}

		for _, testCase := range testBatch {
			t.Run(testCase.name, func(t *testing.T) {
				_, err := testCase.invoke(c)
				assert.Error(t, err)
			})
		}
	}
}

func httpURLtoWS(url string) string {
	return "ws" + strings.TrimPrefix(url, "http") + "/ws"
}

func initTestServer(t *testing.T, resp string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/ws" && req.Method == "GET" {
			var upgrader = websocket.Upgrader{}
			ws, err := upgrader.Upgrade(w, req, nil)
			require.NoError(t, err)
			for {
				ws.SetReadDeadline(time.Now().Add(2 * time.Second))
				_, _, err = ws.ReadMessage()
				if err != nil {
					break
				}
				ws.SetWriteDeadline(time.Now().Add(2 * time.Second))
				err = ws.WriteMessage(1, []byte(resp))
				if err != nil {
					break
				}
			}
			ws.Close()
			return
		}
		requestHandler(t, w, resp)
	}))

	return srv
}

func requestHandler(t *testing.T, w http.ResponseWriter, resp string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, err := w.Write([]byte(resp))

	if err != nil {
		t.Fatalf("Error writing response: %s", err.Error())
	}
}

func TestCalculateValidUntilBlock(t *testing.T) {
	var (
		getBlockCountCalled int
		getValidatorsCalled int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r := request.NewIn()
		err := r.DecodeData(req.Body)
		if err != nil {
			t.Fatalf("Cannot decode request body: %s", req.Body)
		}
		var response string
		switch r.Method {
		case "getblockcount":
			getBlockCountCalled++
			response = `{"jsonrpc":"2.0","id":1,"result":50}`
		case "getvalidators":
			getValidatorsCalled++
			response = `{"id":1,"jsonrpc":"2.0","result":[{"publickey":"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2","votes":"0","active":true},{"publickey":"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e","votes":"0","active":true},{"publickey":"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699","votes":"0","active":true},{"publickey":"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62","votes":"0","active":true}]}`
		default:
			t.Fatalf("Bad request method: %s", r.Method)
		}
		requestHandler(t, w, response)
	}))
	defer srv.Close()

	endpoint := srv.URL
	opts := Options{}
	c, err := New(context.TODO(), endpoint, opts)
	if err != nil {
		t.Fatal(err)
	}

	validUntilBlock, err := c.CalculateValidUntilBlock()
	assert.NoError(t, err)
	assert.Equal(t, uint32(54), validUntilBlock)
	assert.Equal(t, 1, getBlockCountCalled)
	assert.Equal(t, 1, getValidatorsCalled)

	// check, whether caching is working
	validUntilBlock, err = c.CalculateValidUntilBlock()
	assert.NoError(t, err)
	assert.Equal(t, uint32(54), validUntilBlock)
	assert.Equal(t, 2, getBlockCountCalled)
	assert.Equal(t, 1, getValidatorsCalled)
}
