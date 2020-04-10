package transaction

//TODO NEO3.0: Update bynary
/*
func TestEncodeDecodeEnrollment(t *testing.T) {
	rawtx := "200002ff8ac54687f36bbc31a91b730cc385da8af0b581f2d59d82b5cfef824fd271f60001d3d3b7028d61fea3b7803fda3d7f0a1f7262d38e5e1c8987b0313e0a94574151000001e72d286979ee6cb1b7e65dfddfb2e384100b8d148e7758de42e4168b71792c60005441d11600000050ac4949596f5b62fef7be4d1c3e494e6048ed4a01414079d78189d591097b17657a62240c93595e8233dc81157ea2cd477813f09a11fd72845e6bd97c5a3dda125985ea3d5feca387e9933649a9a671a69ab3f6301df6232102ff8ac54687f36bbc31a91b730cc385da8af0b581f2d59d82b5cfef824fd271f6ac"
	tx := decodeTransaction(rawtx, t)
	assert.Equal(t, "988832f693785dcbcb8d5a0e9d5d22002adcbfb1eb6bbeebf8c494fff580e147", tx.Hash().StringLE())
	assert.Equal(t, EnrollmentType, tx.Type)
	assert.IsType(t, tx.Data, &EnrollmentTX{})
	assert.Equal(t, 0, int(tx.Version))

	data, err := testserdes.EncodeBinary(tx)
	assert.Equal(t, nil, err)
	assert.Equal(t, rawtx, hex.EncodeToString(data))
}
*/
