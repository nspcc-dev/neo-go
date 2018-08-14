package chainparams

// For now we will just use this to store the
// peers, to test peer data
// Once complete, it will store the genesis params for testnet and mainnet

var mainnetSeedList = []string{
	"seed1.neo.org:10333",  // NOP
	"seed2.neo.org:10333",  // YEP
	"seed3.neo.org:10333",  // YEP
	"seed4.neo.org:10333",  // NOP
	"seed5.neo.org:10333",  // YEP
	"13.59.52.94:10333",    // NOP
	"18.220.214.143:10333", // NOP
	"13.58.198.112:10333",  // NOP
	"13.59.14.206:10333",   // NOP
	"18.216.9.7:10333",     // NOP
}

//MainnetSeedList is a string slice containing the initial seeds from protocol.mainnet
// That are replying
var MainnetSeedList = []string{
	"seed2.neo.org:10333",
	"seed3.neo.org:10333",
	"seed5.neo.org:10333",
}

var testNetSeedList = []string{

	"18.218.97.227:20333",
	"18.219.30.120:20333",
	"18.219.13.91:20333",
	"13.59.116.121:20333",
	"18.218.255.178:20333",
}
