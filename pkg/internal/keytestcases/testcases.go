package keytestcases

// Ktype represents key testcase values (different encodings of the key).
type Ktype struct {
	Address,
	PrivateKey,
	PublicKey,
	Wif,
	Passphrase,
	EncryptedWif string
	Invalid bool
}

// Arr contains a set of known keys in Ktype format.
var Arr = []Ktype{
	{
		Address:      "NMPAXGtMfZ8s8rcfP9JhrYrNeZHG4xSVmd",
		PrivateKey:   "7d128a6d096f0c14c3a25a2b0c41cf79661bfcb4a8cc95aaaea28bde4d732344",
		PublicKey:    "02028a99826edc0c97d18e22b6932373d908d323aa7f92656a77ec26e8861699ef",
		Wif:          "L1QqQJnpBwbsPGAuutuzPTac8piqvbR1HRjrY5qHup48TBCBFe4g",
		Passphrase:   "city of zion",
		EncryptedWif: "6PYWdzMKGbfxHbfb2JqZJ5Yr1y6jjjuSPLjvgS4byvDkgz2NdiBgeJwBFc",
	},
	{
		Address:      "NRC6oteucWYXq7aASD6YWe5rNeXAw1ehye",
		PrivateKey:   "9ab7e154840daca3a2efadaf0df93cd3a5b51768c632f5433f86909d9b994a69",
		PublicKey:    "031d8e1630ce640966967bc6d95223d21f44304133003140c3b52004dc981349c9",
		Wif:          "L2QTooFoDFyRFTxmtiVHt5CfsXfVnexdbENGDkkrrgTTryiLsPMG",
		Passphrase:   "我的密码",
		EncryptedWif: "6PYLxXgqE8MaQJr3NR96rfFBxzdNLytb3CfxeDXyijSEG2M1vF9d4CoRAg",
	},
	{
		Address:      "NTFAwXLGoiWwSMP5vJyZp8K4cBFwrzUs8m",
		PrivateKey:   "3edee7036b8fd9cef91de47386b191dd76db2888a553e7736bb02808932a915b",
		PublicKey:    "02232ce8d2e2063dce0451131851d47421bfc4fc1da4db116fca5302c0756462fa",
		Wif:          "KyKvWLZsNwBJx5j9nurHYRwhYfdQUu9tTEDsLCUHDbYBL8cHxMiG",
		Passphrase:   "MyL33tP@33w0rd",
		EncryptedWif: "6PYRoabFnWARA3ZWwfJ4efQ4uuuB9WdVrA1LFbMkZLtXZ2DJg3bzjiK59s",
	},
	{
		Address:      "xdf4UGKevVrMR1j3UkPsuoYKSC4ocoAkKx",
		PrivateKey:   "zzdee7036b8fd9cef91de47386b191dd76db2888a553e7736bb02808932a915b",
		PublicKey:    "zz232ce8d2e2063dce0451131851d47421bfc4fc1da4db116fca5302c0756462fa",
		Wif:          "zzKvWLZsNwBJx5j9nurHYRwhYfdQUu9tTEDsLCUHDbYBL8cHxMiG",
		Passphrase:   "zzL33tP@33w0rd",
		EncryptedWif: "6PYRoabFnWARA3ZWwfJ4efQ4uuuB9WdVrA1LFbMkZLtXZ2DJg3bzjiK59s",
		Invalid:      true,
	},
}
