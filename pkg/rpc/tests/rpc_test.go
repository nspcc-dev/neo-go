package rpc

import (
	"testing"
	"context"

//	"github.com/CityOfZion/neo-go/pkg/rpc"
	. "github.com/onsi/gomega"
	"gopkg.in/h2non/gock.v1"
)

const (
	rpcEndpoint     = "http://127.0.0.1:30332"
	neoScanEndpoint = "http://127.0.0.1:4001"
	neoScanPath     = "/api/main_net/v1/get_balance/"
	wif             = "KxDgvEKzgSBPPfuVfw67oPQBSjidEiqTHURKSDL1R7yGaGYAeYnr"
	gasAssetId      = 0x602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7
	address         = "AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y"
	balanceResponse = map[string]string{
		"balance": {
			"unspent": {
				"value": 12093.99869986,
				"txid":  "260f77e0897ed6eec1de62eccf6aefc4e3f07494822471fbdb7450d100fa1a7b",
				"n":     0,
			},
			"asset":  "GAS",
			"amount": 12093.99869986,
		},
		"address": address,
	}
	rawTx = "8000012023ba2703c53263e8d6e522dc32203339dcd8eee9017b1afa00d15074dbfb7124829474f0e3c4ef6acfec62dec1eed67e89e0770f26000002e72d286979ee6cb1b7e65dfddfb2e384100b8d148e7758de42e4168b71792c601f82d9951901000023ba2703c53263e8d6e522dc32203339dcd8eee9e72d286979ee6cb1b7e65dfddfb2e384100b8d148e7758de42e4168b71792c6001000000000000003b3e217936205e1a107bb93ea62fe58e933d746401414044540f2d25c28d97bfa89da054c95e078f8225a80cad6258cd887c6b3234d367c0696c856b17db0ea4f5646aa59d1d473f9bc5f948362023514794a4c53bef9b2321031a6c6fbbdf02ca351745fa86b9ba5a9452d785ac4f7fc2b7548ca2a46c4fcf4aac"
    requestBody = map[string]string{
    	"jsonrpc": "2.0",
    	"method": "sendrawtransaction",
    	"params": rawTx,
    	"id": "1",
	}
    responseBody = map[string]string{
    	"jsonrpc": "2.0",
    	"id": "1",
    	"error": "null",
    	"result": "true",
	}
)

func TestSendToAddress(t *testing.T) {
	defer gock.Off()

	// test tools setup
	gock.New(neoScanEndpoint).
		Get(neoScanPath + address).
		Reply(200).
		JSON(balanceResponse)

	gock.New(rpcEndpoint).Post("/").
		MatchType("json").JSON(requestBody).
		Reply(200).JSON(responseBody)

	g := NewGomegaWithT(t)

	// code setup
	ctx := context.Background()
	opts := rpc.ClientOptions{}
	client, err := rpc.NewClient(ctx, rpcEndpoint, opts)
	client.SetWIF(wif)
	nsServer := rpc.NeoScanServer{
		URL:  neoScanEndpoint,
		Path: neoScanPath,
	}
	client.SetBalancer(nsServer)

	resp, err := client.SendToAddress(gasAssetId, address, 1)
	g.Expect(resp.Result).To(BeTrue())
}

