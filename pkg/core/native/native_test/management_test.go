package native_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	ojson "github.com/nspcc-dev/go-ordered-json"
	"github.com/nspcc-dev/neo-go/internal/contracts"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/chaindump"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativehashes"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/stretchr/testify/require"
)

var (
	// defaultCSS holds serialized native contract states built for genesis block (with UpdateCounter 0)
	// under assumption that all hardforks are disabled.
	defaultCSS = map[string]string{
		nativenames.Management:  `{"id":-1,"hash":"0xfffdc93764dbaddd97c48f252a53ea4643faa3fd","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=","checksum":1094259016},"manifest":{"name":"ContractManagement","abi":{"methods":[{"name":"deploy","offset":0,"parameters":[{"name":"nefFile","type":"ByteArray"},{"name":"manifest","type":"ByteArray"}],"returntype":"Array","safe":false},{"name":"deploy","offset":7,"parameters":[{"name":"nefFile","type":"ByteArray"},{"name":"manifest","type":"ByteArray"},{"name":"data","type":"Any"}],"returntype":"Array","safe":false},{"name":"destroy","offset":14,"parameters":[],"returntype":"Void","safe":false},{"name":"getContract","offset":21,"parameters":[{"name":"hash","type":"Hash160"}],"returntype":"Array","safe":true},{"name":"getContractById","offset":28,"parameters":[{"name":"id","type":"Integer"}],"returntype":"Array","safe":true},{"name":"getContractHashes","offset":35,"parameters":[],"returntype":"InteropInterface","safe":true},{"name":"getMinimumDeploymentFee","offset":42,"parameters":[],"returntype":"Integer","safe":true},{"name":"hasMethod","offset":49,"parameters":[{"name":"hash","type":"Hash160"},{"name":"method","type":"String"},{"name":"pcount","type":"Integer"}],"returntype":"Boolean","safe":true},{"name":"setMinimumDeploymentFee","offset":56,"parameters":[{"name":"value","type":"Integer"}],"returntype":"Void","safe":false},{"name":"update","offset":63,"parameters":[{"name":"nefFile","type":"ByteArray"},{"name":"manifest","type":"ByteArray"}],"returntype":"Void","safe":false},{"name":"update","offset":70,"parameters":[{"name":"nefFile","type":"ByteArray"},{"name":"manifest","type":"ByteArray"},{"name":"data","type":"Any"}],"returntype":"Void","safe":false}],"events":[{"name":"Deploy","parameters":[{"name":"Hash","type":"Hash160"}]},{"name":"Update","parameters":[{"name":"Hash","type":"Hash160"}]},{"name":"Destroy","parameters":[{"name":"Hash","type":"Hash160"}]}]},"features":{},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":[],"trusts":[],"extra":null},"updatecounter":0}`,
		nativenames.StdLib:      `{"id":-2,"hash":"0xacce6fd80d44e1796aa0c2c625e9e4e0ce39efc0","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dA","checksum":1991619121},"manifest":{"name":"StdLib","abi":{"methods":[{"name":"atoi","offset":0,"parameters":[{"name":"value","type":"String"}],"returntype":"Integer","safe":true},{"name":"atoi","offset":7,"parameters":[{"name":"value","type":"String"},{"name":"base","type":"Integer"}],"returntype":"Integer","safe":true},{"name":"base58CheckDecode","offset":14,"parameters":[{"name":"s","type":"String"}],"returntype":"ByteArray","safe":true},{"name":"base58CheckEncode","offset":21,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"String","safe":true},{"name":"base58Decode","offset":28,"parameters":[{"name":"s","type":"String"}],"returntype":"ByteArray","safe":true},{"name":"base58Encode","offset":35,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"String","safe":true},{"name":"base64Decode","offset":42,"parameters":[{"name":"s","type":"String"}],"returntype":"ByteArray","safe":true},{"name":"base64Encode","offset":49,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"String","safe":true},{"name":"deserialize","offset":56,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"Any","safe":true},{"name":"itoa","offset":63,"parameters":[{"name":"value","type":"Integer"}],"returntype":"String","safe":true},{"name":"itoa","offset":70,"parameters":[{"name":"value","type":"Integer"},{"name":"base","type":"Integer"}],"returntype":"String","safe":true},{"name":"jsonDeserialize","offset":77,"parameters":[{"name":"json","type":"ByteArray"}],"returntype":"Any","safe":true},{"name":"jsonSerialize","offset":84,"parameters":[{"name":"item","type":"Any"}],"returntype":"ByteArray","safe":true},{"name":"memoryCompare","offset":91,"parameters":[{"name":"str1","type":"ByteArray"},{"name":"str2","type":"ByteArray"}],"returntype":"Integer","safe":true},{"name":"memorySearch","offset":98,"parameters":[{"name":"mem","type":"ByteArray"},{"name":"value","type":"ByteArray"}],"returntype":"Integer","safe":true},{"name":"memorySearch","offset":105,"parameters":[{"name":"mem","type":"ByteArray"},{"name":"value","type":"ByteArray"},{"name":"start","type":"Integer"}],"returntype":"Integer","safe":true},{"name":"memorySearch","offset":112,"parameters":[{"name":"mem","type":"ByteArray"},{"name":"value","type":"ByteArray"},{"name":"start","type":"Integer"},{"name":"backward","type":"Boolean"}],"returntype":"Integer","safe":true},{"name":"serialize","offset":119,"parameters":[{"name":"item","type":"Any"}],"returntype":"ByteArray","safe":true},{"name":"strLen","offset":126,"parameters":[{"name":"str","type":"String"}],"returntype":"Integer","safe":true},{"name":"stringSplit","offset":133,"parameters":[{"name":"str","type":"String"},{"name":"separator","type":"String"}],"returntype":"Array","safe":true},{"name":"stringSplit","offset":140,"parameters":[{"name":"str","type":"String"},{"name":"separator","type":"String"},{"name":"removeEmptyEntries","type":"Boolean"}],"returntype":"Array","safe":true}],"events":[]},"features":{},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":[],"trusts":[],"extra":null},"updatecounter":0}`,
		nativenames.CryptoLib:   `{"id":-3,"hash":"0x726cb6e0cd8628a1350a611384688911ab75f51b","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQA==","checksum":2135988409},"manifest":{"name":"CryptoLib","abi":{"methods":[{"name":"bls12381Add","offset":0,"parameters":[{"name":"x","type":"InteropInterface"},{"name":"y","type":"InteropInterface"}],"returntype":"InteropInterface","safe":true},{"name":"bls12381Deserialize","offset":7,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"InteropInterface","safe":true},{"name":"bls12381Equal","offset":14,"parameters":[{"name":"x","type":"InteropInterface"},{"name":"y","type":"InteropInterface"}],"returntype":"Boolean","safe":true},{"name":"bls12381Mul","offset":21,"parameters":[{"name":"x","type":"InteropInterface"},{"name":"mul","type":"ByteArray"},{"name":"neg","type":"Boolean"}],"returntype":"InteropInterface","safe":true},{"name":"bls12381Pairing","offset":28,"parameters":[{"name":"g1","type":"InteropInterface"},{"name":"g2","type":"InteropInterface"}],"returntype":"InteropInterface","safe":true},{"name":"bls12381Serialize","offset":35,"parameters":[{"name":"g","type":"InteropInterface"}],"returntype":"ByteArray","safe":true},{"name":"murmur32","offset":42,"parameters":[{"name":"data","type":"ByteArray"},{"name":"seed","type":"Integer"}],"returntype":"ByteArray","safe":true},{"name":"ripemd160","offset":49,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"ByteArray","safe":true},{"name":"sha256","offset":56,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"ByteArray","safe":true},{"name":"verifyWithECDsa","offset":63,"parameters":[{"name":"message","type":"ByteArray"},{"name":"pubkey","type":"ByteArray"},{"name":"signature","type":"ByteArray"},{"name":"curve","type":"Integer"}],"returntype":"Boolean","safe":true}],"events":[]},"features":{},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":[],"trusts":[],"extra":null},"updatecounter":0}`,
		nativenames.Ledger:      `{"id":-4,"hash":"0xda65b600f7124ce6c79950c1772a36403104f2be","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=","checksum":1110259869},"manifest":{"name":"LedgerContract","abi":{"methods":[{"name":"currentHash","offset":0,"parameters":[],"returntype":"Hash256","safe":true},{"name":"currentIndex","offset":7,"parameters":[],"returntype":"Integer","safe":true},{"name":"getBlock","offset":14,"parameters":[{"name":"indexOrHash","type":"ByteArray"}],"returntype":"Array","safe":true},{"name":"getTransaction","offset":21,"parameters":[{"name":"hash","type":"Hash256"}],"returntype":"Array","safe":true},{"name":"getTransactionFromBlock","offset":28,"parameters":[{"name":"blockIndexOrHash","type":"ByteArray"},{"name":"txIndex","type":"Integer"}],"returntype":"Array","safe":true},{"name":"getTransactionHeight","offset":35,"parameters":[{"name":"hash","type":"Hash256"}],"returntype":"Integer","safe":true},{"name":"getTransactionSigners","offset":42,"parameters":[{"name":"hash","type":"Hash256"}],"returntype":"Array","safe":true},{"name":"getTransactionVMState","offset":49,"parameters":[{"name":"hash","type":"Hash256"}],"returntype":"Integer","safe":true}],"events":[]},"features":{},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":[],"trusts":[],"extra":null},"updatecounter":0}`,
		nativenames.Neo:         `{"id":-5,"hash":"0xef4073a0f2b305a38ec4050e4d3d28bc40ea63f5","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQA==","checksum":65467259},"manifest":{"name":"NeoToken","abi":{"methods":[{"name":"balanceOf","offset":0,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Integer","safe":true},{"name":"decimals","offset":7,"parameters":[],"returntype":"Integer","safe":true},{"name":"getAccountState","offset":14,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Array","safe":true},{"name":"getAllCandidates","offset":21,"parameters":[],"returntype":"InteropInterface","safe":true},{"name":"getCandidateVote","offset":28,"parameters":[{"name":"pubKey","type":"PublicKey"}],"returntype":"Integer","safe":true},{"name":"getCandidates","offset":35,"parameters":[],"returntype":"Array","safe":true},{"name":"getCommittee","offset":42,"parameters":[],"returntype":"Array","safe":true},{"name":"getGasPerBlock","offset":49,"parameters":[],"returntype":"Integer","safe":true},{"name":"getNextBlockValidators","offset":56,"parameters":[],"returntype":"Array","safe":true},{"name":"getRegisterPrice","offset":63,"parameters":[],"returntype":"Integer","safe":true},{"name":"registerCandidate","offset":70,"parameters":[{"name":"pubkey","type":"PublicKey"}],"returntype":"Boolean","safe":false},{"name":"setGasPerBlock","offset":77,"parameters":[{"name":"gasPerBlock","type":"Integer"}],"returntype":"Void","safe":false},{"name":"setRegisterPrice","offset":84,"parameters":[{"name":"registerPrice","type":"Integer"}],"returntype":"Void","safe":false},{"name":"symbol","offset":91,"parameters":[],"returntype":"String","safe":true},{"name":"totalSupply","offset":98,"parameters":[],"returntype":"Integer","safe":true},{"name":"transfer","offset":105,"parameters":[{"name":"from","type":"Hash160"},{"name":"to","type":"Hash160"},{"name":"amount","type":"Integer"},{"name":"data","type":"Any"}],"returntype":"Boolean","safe":false},{"name":"unclaimedGas","offset":112,"parameters":[{"name":"account","type":"Hash160"},{"name":"end","type":"Integer"}],"returntype":"Integer","safe":true},{"name":"unregisterCandidate","offset":119,"parameters":[{"name":"pubkey","type":"PublicKey"}],"returntype":"Boolean","safe":false},{"name":"vote","offset":126,"parameters":[{"name":"account","type":"Hash160"},{"name":"voteTo","type":"PublicKey"}],"returntype":"Boolean","safe":false}],"events":[{"name":"Transfer","parameters":[{"name":"from","type":"Hash160"},{"name":"to","type":"Hash160"},{"name":"amount","type":"Integer"}]},{"name":"CandidateStateChanged","parameters":[{"name":"pubkey","type":"PublicKey"},{"name":"registered","type":"Boolean"},{"name":"votes","type":"Integer"}]},{"name":"Vote","parameters":[{"name":"account","type":"Hash160"},{"name":"from","type":"PublicKey"},{"name":"to","type":"PublicKey"},{"name":"amount","type":"Integer"}]}]},"features":{},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":["NEP-17"],"trusts":[],"extra":null},"updatecounter":0}`,
		nativenames.Gas:         `{"id":-6,"hash":"0xd2a4cff31913016155e38e474a2c06d08be276cf","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=","checksum":2663858513},"manifest":{"name":"GasToken","abi":{"methods":[{"name":"balanceOf","offset":0,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Integer","safe":true},{"name":"decimals","offset":7,"parameters":[],"returntype":"Integer","safe":true},{"name":"symbol","offset":14,"parameters":[],"returntype":"String","safe":true},{"name":"totalSupply","offset":21,"parameters":[],"returntype":"Integer","safe":true},{"name":"transfer","offset":28,"parameters":[{"name":"from","type":"Hash160"},{"name":"to","type":"Hash160"},{"name":"amount","type":"Integer"},{"name":"data","type":"Any"}],"returntype":"Boolean","safe":false}],"events":[{"name":"Transfer","parameters":[{"name":"from","type":"Hash160"},{"name":"to","type":"Hash160"},{"name":"amount","type":"Integer"}]}]},"features":{},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":["NEP-17"],"trusts":[],"extra":null},"updatecounter":0}`,
		nativenames.Policy:      `{"id":-7,"hash":"0xcc5e4edd9f5f8dba8bb65734541df7a1c081c67b","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=","checksum":1094259016},"manifest":{"name":"PolicyContract","abi":{"methods":[{"name":"blockAccount","offset":0,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Boolean","safe":false},{"name":"getAttributeFee","offset":7,"parameters":[{"name":"attributeType","type":"Integer"}],"returntype":"Integer","safe":true},{"name":"getExecFeeFactor","offset":14,"parameters":[],"returntype":"Integer","safe":true},{"name":"getFeePerByte","offset":21,"parameters":[],"returntype":"Integer","safe":true},{"name":"getStoragePrice","offset":28,"parameters":[],"returntype":"Integer","safe":true},{"name":"isBlocked","offset":35,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Boolean","safe":true},{"name":"setAttributeFee","offset":42,"parameters":[{"name":"attributeType","type":"Integer"},{"name":"value","type":"Integer"}],"returntype":"Void","safe":false},{"name":"setExecFeeFactor","offset":49,"parameters":[{"name":"value","type":"Integer"}],"returntype":"Void","safe":false},{"name":"setFeePerByte","offset":56,"parameters":[{"name":"value","type":"Integer"}],"returntype":"Void","safe":false},{"name":"setStoragePrice","offset":63,"parameters":[{"name":"value","type":"Integer"}],"returntype":"Void","safe":false},{"name":"unblockAccount","offset":70,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Boolean","safe":false}],"events":[]},"features":{},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":[],"trusts":[],"extra":null},"updatecounter":0}`,
		nativenames.Designation: `{"id":-8,"hash":"0x49cf4e5378ffcd4dec034fd98a174c5491e395e2","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0A=","checksum":983638438},"manifest":{"name":"RoleManagement","abi":{"methods":[{"name":"designateAsRole","offset":0,"parameters":[{"name":"role","type":"Integer"},{"name":"nodes","type":"Array"}],"returntype":"Void","safe":false},{"name":"getDesignatedByRole","offset":7,"parameters":[{"name":"role","type":"Integer"},{"name":"index","type":"Integer"}],"returntype":"Array","safe":true}],"events":[{"name":"Designation","parameters":[{"name":"Role","type":"Integer"},{"name":"BlockIndex","type":"Integer"}]}]},"features":{},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":[],"trusts":[],"extra":null},"updatecounter":0}`,
		nativenames.Oracle:      `{"id":-9,"hash":"0xfe924b7cfe89ddd271abaf7210a80a7e11178758","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=","checksum":2663858513},"manifest":{"name":"OracleContract","abi":{"methods":[{"name":"finish","offset":0,"parameters":[],"returntype":"Void","safe":false},{"name":"getPrice","offset":7,"parameters":[],"returntype":"Integer","safe":true},{"name":"request","offset":14,"parameters":[{"name":"url","type":"String"},{"name":"filter","type":"String"},{"name":"callback","type":"String"},{"name":"userData","type":"Any"},{"name":"gasForResponse","type":"Integer"}],"returntype":"Void","safe":false},{"name":"setPrice","offset":21,"parameters":[{"name":"price","type":"Integer"}],"returntype":"Void","safe":false},{"name":"verify","offset":28,"parameters":[],"returntype":"Boolean","safe":true}],"events":[{"name":"OracleRequest","parameters":[{"name":"Id","type":"Integer"},{"name":"RequestContract","type":"Hash160"},{"name":"Url","type":"String"},{"name":"Filter","type":"String"}]},{"name":"OracleResponse","parameters":[{"name":"Id","type":"Integer"},{"name":"OriginalTx","type":"Hash256"}]}]},"features":{},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":[],"trusts":[],"extra":null},"updatecounter":0}`,
		nativenames.Notary:      `{"id":-10,"hash":"0xc1e14f19c3e60d0b9244d06dd7ba9b113135ec3b","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=","checksum":1110259869},"manifest":{"name":"Notary","abi":{"methods":[{"name":"balanceOf","offset":0,"parameters":[{"name":"addr","type":"Hash160"}],"returntype":"Integer","safe":true},{"name":"expirationOf","offset":7,"parameters":[{"name":"addr","type":"Hash160"}],"returntype":"Integer","safe":true},{"name":"getMaxNotValidBeforeDelta","offset":14,"parameters":[],"returntype":"Integer","safe":true},{"name":"lockDepositUntil","offset":21,"parameters":[{"name":"address","type":"Hash160"},{"name":"till","type":"Integer"}],"returntype":"Boolean","safe":false},{"name":"onNEP17Payment","offset":28,"parameters":[{"name":"from","type":"Hash160"},{"name":"amount","type":"Integer"},{"name":"data","type":"Any"}],"returntype":"Void","safe":false},{"name":"setMaxNotValidBeforeDelta","offset":35,"parameters":[{"name":"value","type":"Integer"}],"returntype":"Void","safe":false},{"name":"verify","offset":42,"parameters":[{"name":"signature","type":"Signature"}],"returntype":"Boolean","safe":true},{"name":"withdraw","offset":49,"parameters":[{"name":"from","type":"Hash160"},{"name":"to","type":"Hash160"}],"returntype":"Boolean","safe":false}],"events":[]},"features":{},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":["NEP-27"],"trusts":[],"extra":null},"updatecounter":0}`,
	}
	// cockatriceCSS holds serialized native contract states built for genesis block (with UpdateCounter 0)
	// under assumption that hardforks from Aspidochelone to Cockatrice (included) are enabled.
	cockatriceCSS = map[string]string{
		nativenames.CryptoLib: `{"id":-3,"hash":"0x726cb6e0cd8628a1350a611384688911ab75f51b","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=","checksum":1094259016},"manifest":{"name":"CryptoLib","abi":{"methods":[{"name":"bls12381Add","offset":0,"parameters":[{"name":"x","type":"InteropInterface"},{"name":"y","type":"InteropInterface"}],"returntype":"InteropInterface","safe":true},{"name":"bls12381Deserialize","offset":7,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"InteropInterface","safe":true},{"name":"bls12381Equal","offset":14,"parameters":[{"name":"x","type":"InteropInterface"},{"name":"y","type":"InteropInterface"}],"returntype":"Boolean","safe":true},{"name":"bls12381Mul","offset":21,"parameters":[{"name":"x","type":"InteropInterface"},{"name":"mul","type":"ByteArray"},{"name":"neg","type":"Boolean"}],"returntype":"InteropInterface","safe":true},{"name":"bls12381Pairing","offset":28,"parameters":[{"name":"g1","type":"InteropInterface"},{"name":"g2","type":"InteropInterface"}],"returntype":"InteropInterface","safe":true},{"name":"bls12381Serialize","offset":35,"parameters":[{"name":"g","type":"InteropInterface"}],"returntype":"ByteArray","safe":true},{"name":"keccak256","offset":42,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"ByteArray","safe":true},{"name":"murmur32","offset":49,"parameters":[{"name":"data","type":"ByteArray"},{"name":"seed","type":"Integer"}],"returntype":"ByteArray","safe":true},{"name":"ripemd160","offset":56,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"ByteArray","safe":true},{"name":"sha256","offset":63,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"ByteArray","safe":true},{"name":"verifyWithECDsa","offset":70,"parameters":[{"name":"message","type":"ByteArray"},{"name":"pubkey","type":"ByteArray"},{"name":"signature","type":"ByteArray"},{"name":"curveHash","type":"Integer"}],"returntype":"Boolean","safe":true}],"events":[]},"features":{},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":[],"trusts":[],"extra":null},"updatecounter":0}`,
		nativenames.Neo:       `{"id":-5,"hash":"0xef4073a0f2b305a38ec4050e4d3d28bc40ea63f5","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=","checksum":1325686241},"manifest":{"name":"NeoToken","abi":{"methods":[{"name":"balanceOf","offset":0,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Integer","safe":true},{"name":"decimals","offset":7,"parameters":[],"returntype":"Integer","safe":true},{"name":"getAccountState","offset":14,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Array","safe":true},{"name":"getAllCandidates","offset":21,"parameters":[],"returntype":"InteropInterface","safe":true},{"name":"getCandidateVote","offset":28,"parameters":[{"name":"pubKey","type":"PublicKey"}],"returntype":"Integer","safe":true},{"name":"getCandidates","offset":35,"parameters":[],"returntype":"Array","safe":true},{"name":"getCommittee","offset":42,"parameters":[],"returntype":"Array","safe":true},{"name":"getCommitteeAddress","offset":49,"parameters":[],"returntype":"Hash160","safe":true},{"name":"getGasPerBlock","offset":56,"parameters":[],"returntype":"Integer","safe":true},{"name":"getNextBlockValidators","offset":63,"parameters":[],"returntype":"Array","safe":true},{"name":"getRegisterPrice","offset":70,"parameters":[],"returntype":"Integer","safe":true},{"name":"registerCandidate","offset":77,"parameters":[{"name":"pubkey","type":"PublicKey"}],"returntype":"Boolean","safe":false},{"name":"setGasPerBlock","offset":84,"parameters":[{"name":"gasPerBlock","type":"Integer"}],"returntype":"Void","safe":false},{"name":"setRegisterPrice","offset":91,"parameters":[{"name":"registerPrice","type":"Integer"}],"returntype":"Void","safe":false},{"name":"symbol","offset":98,"parameters":[],"returntype":"String","safe":true},{"name":"totalSupply","offset":105,"parameters":[],"returntype":"Integer","safe":true},{"name":"transfer","offset":112,"parameters":[{"name":"from","type":"Hash160"},{"name":"to","type":"Hash160"},{"name":"amount","type":"Integer"},{"name":"data","type":"Any"}],"returntype":"Boolean","safe":false},{"name":"unclaimedGas","offset":119,"parameters":[{"name":"account","type":"Hash160"},{"name":"end","type":"Integer"}],"returntype":"Integer","safe":true},{"name":"unregisterCandidate","offset":126,"parameters":[{"name":"pubkey","type":"PublicKey"}],"returntype":"Boolean","safe":false},{"name":"vote","offset":133,"parameters":[{"name":"account","type":"Hash160"},{"name":"voteTo","type":"PublicKey"}],"returntype":"Boolean","safe":false}],"events":[{"name":"Transfer","parameters":[{"name":"from","type":"Hash160"},{"name":"to","type":"Hash160"},{"name":"amount","type":"Integer"}]},{"name":"CandidateStateChanged","parameters":[{"name":"pubkey","type":"PublicKey"},{"name":"registered","type":"Boolean"},{"name":"votes","type":"Integer"}]},{"name":"Vote","parameters":[{"name":"account","type":"Hash160"},{"name":"from","type":"PublicKey"},{"name":"to","type":"PublicKey"},{"name":"amount","type":"Integer"}]},{"name":"CommitteeChanged","parameters":[{"name":"old","type":"Array"},{"name":"new","type":"Array"}]}]},"features":{},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":["NEP-17"],"trusts":[],"extra":null},"updatecounter":0}`,
	}
	// echidnaCSS holds serialized native contract states built for genesis block (with UpdateCounter 0)
	// under assumption that hardforks from Aspidochelone to Echidna (included) are enabled.
	echidnaCSS = map[string]string{
		nativenames.Management:  `{"id":-1,"hash":"0xfffdc93764dbaddd97c48f252a53ea4643faa3fd","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dA","checksum":3581846399},"manifest":{"name":"ContractManagement","abi":{"methods":[{"name":"deploy","offset":0,"parameters":[{"name":"nefFile","type":"ByteArray"},{"name":"manifest","type":"ByteArray"}],"returntype":"Array","safe":false},{"name":"deploy","offset":7,"parameters":[{"name":"nefFile","type":"ByteArray"},{"name":"manifest","type":"ByteArray"},{"name":"data","type":"Any"}],"returntype":"Array","safe":false},{"name":"destroy","offset":14,"parameters":[],"returntype":"Void","safe":false},{"name":"getContract","offset":21,"parameters":[{"name":"hash","type":"Hash160"}],"returntype":"Array","safe":true},{"name":"getContractById","offset":28,"parameters":[{"name":"id","type":"Integer"}],"returntype":"Array","safe":true},{"name":"getContractHashes","offset":35,"parameters":[],"returntype":"InteropInterface","safe":true},{"name":"getMinimumDeploymentFee","offset":42,"parameters":[],"returntype":"Integer","safe":true},{"name":"hasMethod","offset":49,"parameters":[{"name":"hash","type":"Hash160"},{"name":"method","type":"String"},{"name":"pcount","type":"Integer"}],"returntype":"Boolean","safe":true},{"name":"isContract","offset":56,"parameters":[{"name":"hash","type":"Hash160"}],"returntype":"Boolean","safe":true},{"name":"setMinimumDeploymentFee","offset":63,"parameters":[{"name":"value","type":"Integer"}],"returntype":"Void","safe":false},{"name":"update","offset":70,"parameters":[{"name":"nefFile","type":"ByteArray"},{"name":"manifest","type":"ByteArray"}],"returntype":"Void","safe":false},{"name":"update","offset":77,"parameters":[{"name":"nefFile","type":"ByteArray"},{"name":"manifest","type":"ByteArray"},{"name":"data","type":"Any"}],"returntype":"Void","safe":false}],"events":[{"name":"Deploy","parameters":[{"name":"Hash","type":"Hash160"}]},{"name":"Update","parameters":[{"name":"Hash","type":"Hash160"}]},{"name":"Destroy","parameters":[{"name":"Hash","type":"Hash160"}]}]},"features":{},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":[],"trusts":[],"extra":null},"updatecounter":0}`,
		nativenames.StdLib:      `{"id":-2,"hash":"0xacce6fd80d44e1796aa0c2c625e9e4e0ce39efc0","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=","checksum":2681632925},"manifest":{"name":"StdLib","abi":{"methods":[{"name":"atoi","offset":0,"parameters":[{"name":"value","type":"String"}],"returntype":"Integer","safe":true},{"name":"atoi","offset":7,"parameters":[{"name":"value","type":"String"},{"name":"base","type":"Integer"}],"returntype":"Integer","safe":true},{"name":"base58CheckDecode","offset":14,"parameters":[{"name":"s","type":"String"}],"returntype":"ByteArray","safe":true},{"name":"base58CheckEncode","offset":21,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"String","safe":true},{"name":"base58Decode","offset":28,"parameters":[{"name":"s","type":"String"}],"returntype":"ByteArray","safe":true},{"name":"base58Encode","offset":35,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"String","safe":true},{"name":"base64Decode","offset":42,"parameters":[{"name":"s","type":"String"}],"returntype":"ByteArray","safe":true},{"name":"base64Encode","offset":49,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"String","safe":true},{"name":"base64UrlDecode","offset":56,"parameters":[{"name":"s","type":"String"}],"returntype":"String","safe":true},{"name":"base64UrlEncode","offset":63,"parameters":[{"name":"data","type":"String"}],"returntype":"String","safe":true},{"name":"deserialize","offset":70,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"Any","safe":true},{"name":"itoa","offset":77,"parameters":[{"name":"value","type":"Integer"}],"returntype":"String","safe":true},{"name":"itoa","offset":84,"parameters":[{"name":"value","type":"Integer"},{"name":"base","type":"Integer"}],"returntype":"String","safe":true},{"name":"jsonDeserialize","offset":91,"parameters":[{"name":"json","type":"ByteArray"}],"returntype":"Any","safe":true},{"name":"jsonSerialize","offset":98,"parameters":[{"name":"item","type":"Any"}],"returntype":"ByteArray","safe":true},{"name":"memoryCompare","offset":105,"parameters":[{"name":"str1","type":"ByteArray"},{"name":"str2","type":"ByteArray"}],"returntype":"Integer","safe":true},{"name":"memorySearch","offset":112,"parameters":[{"name":"mem","type":"ByteArray"},{"name":"value","type":"ByteArray"}],"returntype":"Integer","safe":true},{"name":"memorySearch","offset":119,"parameters":[{"name":"mem","type":"ByteArray"},{"name":"value","type":"ByteArray"},{"name":"start","type":"Integer"}],"returntype":"Integer","safe":true},{"name":"memorySearch","offset":126,"parameters":[{"name":"mem","type":"ByteArray"},{"name":"value","type":"ByteArray"},{"name":"start","type":"Integer"},{"name":"backward","type":"Boolean"}],"returntype":"Integer","safe":true},{"name":"serialize","offset":133,"parameters":[{"name":"item","type":"Any"}],"returntype":"ByteArray","safe":true},{"name":"strLen","offset":140,"parameters":[{"name":"str","type":"String"}],"returntype":"Integer","safe":true},{"name":"stringSplit","offset":147,"parameters":[{"name":"str","type":"String"},{"name":"separator","type":"String"}],"returntype":"Array","safe":true},{"name":"stringSplit","offset":154,"parameters":[{"name":"str","type":"String"},{"name":"separator","type":"String"},{"name":"removeEmptyEntries","type":"Boolean"}],"returntype":"Array","safe":true}],"events":[]},"features":{},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":[],"trusts":[],"extra":null},"updatecounter":0}`,
		nativenames.CryptoLib:   `{"id":-3,"hash":"0x726cb6e0cd8628a1350a611384688911ab75f51b","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQA==","checksum":174904780},"manifest":{"name":"CryptoLib","abi":{"methods":[{"name":"bls12381Add","offset":0,"parameters":[{"name":"x","type":"InteropInterface"},{"name":"y","type":"InteropInterface"}],"returntype":"InteropInterface","safe":true},{"name":"bls12381Deserialize","offset":7,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"InteropInterface","safe":true},{"name":"bls12381Equal","offset":14,"parameters":[{"name":"x","type":"InteropInterface"},{"name":"y","type":"InteropInterface"}],"returntype":"Boolean","safe":true},{"name":"bls12381Mul","offset":21,"parameters":[{"name":"x","type":"InteropInterface"},{"name":"mul","type":"ByteArray"},{"name":"neg","type":"Boolean"}],"returntype":"InteropInterface","safe":true},{"name":"bls12381Pairing","offset":28,"parameters":[{"name":"g1","type":"InteropInterface"},{"name":"g2","type":"InteropInterface"}],"returntype":"InteropInterface","safe":true},{"name":"bls12381Serialize","offset":35,"parameters":[{"name":"g","type":"InteropInterface"}],"returntype":"ByteArray","safe":true},{"name":"keccak256","offset":42,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"ByteArray","safe":true},{"name":"murmur32","offset":49,"parameters":[{"name":"data","type":"ByteArray"},{"name":"seed","type":"Integer"}],"returntype":"ByteArray","safe":true},{"name":"recoverSecp256K1","offset":56,"parameters":[{"name":"messageHash","type":"ByteArray"},{"name":"signature","type":"ByteArray"}],"returntype":"ByteArray","safe":true},{"name":"ripemd160","offset":63,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"ByteArray","safe":true},{"name":"sha256","offset":70,"parameters":[{"name":"data","type":"ByteArray"}],"returntype":"ByteArray","safe":true},{"name":"verifyWithECDsa","offset":77,"parameters":[{"name":"message","type":"ByteArray"},{"name":"pubkey","type":"ByteArray"},{"name":"signature","type":"ByteArray"},{"name":"curveHash","type":"Integer"}],"returntype":"Boolean","safe":true},{"name":"verifyWithEd25519","offset":84,"parameters":[{"name":"message","type":"ByteArray"},{"name":"pubkey","type":"ByteArray"},{"name":"signature","type":"ByteArray"}],"returntype":"Boolean","safe":true}],"events":[]},"features":{},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":[],"trusts":[],"extra":null},"updatecounter":0}`,
		nativenames.Neo:         `{"id":-5,"hash":"0xef4073a0f2b305a38ec4050e4d3d28bc40ea63f5","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dA","checksum":1991619121},"manifest":{"name":"NeoToken","abi":{"methods":[{"name":"balanceOf","offset":0,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Integer","safe":true},{"name":"decimals","offset":7,"parameters":[],"returntype":"Integer","safe":true},{"name":"getAccountState","offset":14,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Array","safe":true},{"name":"getAllCandidates","offset":21,"parameters":[],"returntype":"InteropInterface","safe":true},{"name":"getCandidateVote","offset":28,"parameters":[{"name":"pubKey","type":"PublicKey"}],"returntype":"Integer","safe":true},{"name":"getCandidates","offset":35,"parameters":[],"returntype":"Array","safe":true},{"name":"getCommittee","offset":42,"parameters":[],"returntype":"Array","safe":true},{"name":"getCommitteeAddress","offset":49,"parameters":[],"returntype":"Hash160","safe":true},{"name":"getGasPerBlock","offset":56,"parameters":[],"returntype":"Integer","safe":true},{"name":"getNextBlockValidators","offset":63,"parameters":[],"returntype":"Array","safe":true},{"name":"getRegisterPrice","offset":70,"parameters":[],"returntype":"Integer","safe":true},{"name":"onNEP17Payment","offset":77,"parameters":[{"name":"from","type":"Hash160"},{"name":"amount","type":"Integer"},{"name":"data","type":"Any"}],"returntype":"Void","safe":false},{"name":"registerCandidate","offset":84,"parameters":[{"name":"pubkey","type":"PublicKey"}],"returntype":"Boolean","safe":false},{"name":"setGasPerBlock","offset":91,"parameters":[{"name":"gasPerBlock","type":"Integer"}],"returntype":"Void","safe":false},{"name":"setRegisterPrice","offset":98,"parameters":[{"name":"registerPrice","type":"Integer"}],"returntype":"Void","safe":false},{"name":"symbol","offset":105,"parameters":[],"returntype":"String","safe":true},{"name":"totalSupply","offset":112,"parameters":[],"returntype":"Integer","safe":true},{"name":"transfer","offset":119,"parameters":[{"name":"from","type":"Hash160"},{"name":"to","type":"Hash160"},{"name":"amount","type":"Integer"},{"name":"data","type":"Any"}],"returntype":"Boolean","safe":false},{"name":"unclaimedGas","offset":126,"parameters":[{"name":"account","type":"Hash160"},{"name":"end","type":"Integer"}],"returntype":"Integer","safe":true},{"name":"unregisterCandidate","offset":133,"parameters":[{"name":"pubkey","type":"PublicKey"}],"returntype":"Boolean","safe":false},{"name":"vote","offset":140,"parameters":[{"name":"account","type":"Hash160"},{"name":"voteTo","type":"PublicKey"}],"returntype":"Boolean","safe":false}],"events":[{"name":"Transfer","parameters":[{"name":"from","type":"Hash160"},{"name":"to","type":"Hash160"},{"name":"amount","type":"Integer"}]},{"name":"CandidateStateChanged","parameters":[{"name":"pubkey","type":"PublicKey"},{"name":"registered","type":"Boolean"},{"name":"votes","type":"Integer"}]},{"name":"Vote","parameters":[{"name":"account","type":"Hash160"},{"name":"from","type":"PublicKey"},{"name":"to","type":"PublicKey"},{"name":"amount","type":"Integer"}]},{"name":"CommitteeChanged","parameters":[{"name":"old","type":"Array"},{"name":"new","type":"Array"}]}]},"features":{},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":["NEP-17","NEP-27"],"trusts":[],"extra":null},"updatecounter":0}`,
		nativenames.Policy:      `{"id":-7,"hash":"0xcc5e4edd9f5f8dba8bb65734541df7a1c081c67b","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=","checksum":588003825},"manifest":{"name":"PolicyContract","abi":{"methods":[{"name":"blockAccount","offset":0,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Boolean","safe":false},{"name":"getAttributeFee","offset":7,"parameters":[{"name":"attributeType","type":"Integer"}],"returntype":"Integer","safe":true},{"name":"getExecFeeFactor","offset":14,"parameters":[],"returntype":"Integer","safe":true},{"name":"getFeePerByte","offset":21,"parameters":[],"returntype":"Integer","safe":true},{"name":"getMaxTraceableBlocks","offset":28,"parameters":[],"returntype":"Integer","safe":true},{"name":"getMaxValidUntilBlockIncrement","offset":35,"parameters":[],"returntype":"Integer","safe":true},{"name":"getMillisecondsPerBlock","offset":42,"parameters":[],"returntype":"Integer","safe":true},{"name":"getStoragePrice","offset":49,"parameters":[],"returntype":"Integer","safe":true},{"name":"isBlocked","offset":56,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Boolean","safe":true},{"name":"setAttributeFee","offset":63,"parameters":[{"name":"attributeType","type":"Integer"},{"name":"value","type":"Integer"}],"returntype":"Void","safe":false},{"name":"setExecFeeFactor","offset":70,"parameters":[{"name":"value","type":"Integer"}],"returntype":"Void","safe":false},{"name":"setFeePerByte","offset":77,"parameters":[{"name":"value","type":"Integer"}],"returntype":"Void","safe":false},{"name":"setMaxTraceableBlocks","offset":84,"parameters":[{"name":"value","type":"Integer"}],"returntype":"Void","safe":false},{"name":"setMaxValidUntilBlockIncrement","offset":91,"parameters":[{"name":"value","type":"Integer"}],"returntype":"Void","safe":false},{"name":"setMillisecondsPerBlock","offset":98,"parameters":[{"name":"value","type":"Integer"}],"returntype":"Void","safe":false},{"name":"setStoragePrice","offset":105,"parameters":[{"name":"value","type":"Integer"}],"returntype":"Void","safe":false},{"name":"unblockAccount","offset":112,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Boolean","safe":false}],"events":[{"name":"MillisecondsPerBlockChanged","parameters":[{"name":"old","type":"Integer"},{"name":"new","type":"Integer"}]}]},"features":{},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":[],"trusts":[],"extra":null},"updatecounter":0}`,
		nativenames.Designation: `{"id":-8,"hash":"0x49cf4e5378ffcd4dec034fd98a174c5491e395e2","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0A=","checksum":983638438},"manifest":{"name":"RoleManagement","abi":{"methods":[{"name":"designateAsRole","offset":0,"parameters":[{"name":"role","type":"Integer"},{"name":"nodes","type":"Array"}],"returntype":"Void","safe":false},{"name":"getDesignatedByRole","offset":7,"parameters":[{"name":"role","type":"Integer"},{"name":"index","type":"Integer"}],"returntype":"Array","safe":true}],"events":[{"name":"Designation","parameters":[{"name":"Role","type":"Integer"},{"name":"BlockIndex","type":"Integer"},{"name":"Old","type":"Array"},{"name":"New","type":"Array"}]}]},"features":{},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":[],"trusts":[],"extra":null},"updatecounter":0}`,
	}
)

func init() {
	for k, v := range defaultCSS {
		if _, ok := cockatriceCSS[k]; !ok {
			cockatriceCSS[k] = v
		}
		if _, ok := echidnaCSS[k]; !ok {
			echidnaCSS[k] = cockatriceCSS[k]
		}
	}
}

func newManagementClient(t *testing.T) *neotest.ContractInvoker {
	return newNativeClient(t, nativenames.Management)
}

// newCustomManagementClient returns native Management invoker backed with chain with
// specified custom configuration.
func newCustomManagementClient(t *testing.T, f func(cfg *config.Blockchain)) *neotest.ContractInvoker {
	return newCustomNativeClient(t, nativenames.Management, f)
}

func TestManagement_MinimumDeploymentFee(t *testing.T) {
	testGetSet(t, newManagementClient(t), "MinimumDeploymentFee", 10_00000000, 0, 0)
}

func TestManagement_MinimumDeploymentFeeCache(t *testing.T) {
	c := newManagementClient(t)
	testGetSetCache(t, c, "MinimumDeploymentFee", 10_00000000)
}

func TestManagement_GenesisNativeState(t *testing.T) {
	// check ensures that contract state stored in native Management matches the expected one.
	check := func(t *testing.T, c *neotest.ContractInvoker, expected map[string]string) {
		for _, name := range nativenames.All {
			h := state.CreateNativeContractHash(name)
			c.InvokeAndCheck(t, func(t testing.TB, stack []stackitem.Item) {
				si := stack[0]
				var cs = &state.Contract{}
				require.NoError(t, cs.FromStackItem(si), name)
				jBytes, err := ojson.Marshal(cs)
				require.NoError(t, err)
				require.Equal(t, expected[name], string(jBytes), fmt.Errorf("contract %s state mismatch", name))
			}, "getContract", h.BytesBE())
		}
	}

	t.Run("disabled hardforks", func(t *testing.T) {
		mgmt := newCustomManagementClient(t, func(cfg *config.Blockchain) {
			cfg.Hardforks = map[string]uint32{}
			cfg.P2PSigExtensions = true
		})
		check(t, mgmt, defaultCSS)
	})
	t.Run("remotely enabled hardforks", func(t *testing.T) {
		mgmt := newCustomManagementClient(t, func(cfg *config.Blockchain) {
			cfg.Hardforks = map[string]uint32{
				config.HFAspidochelone.String(): 100500,
				config.HFBasilisk.String():      100500,
				config.HFCockatrice.String():    100500,
				config.HFEchidna.String():       100500,
			}
			cfg.P2PSigExtensions = true
		})
		check(t, mgmt, defaultCSS)
	})
	t.Run("Aspidochelone enabled", func(t *testing.T) {
		mgmt := newCustomManagementClient(t, func(cfg *config.Blockchain) {
			cfg.Hardforks = map[string]uint32{
				config.HFAspidochelone.String(): 0,
			}
			cfg.P2PSigExtensions = true
		})
		check(t, mgmt, defaultCSS)
	})
	t.Run("Basilisk enabled", func(t *testing.T) {
		mgmt := newCustomManagementClient(t, func(cfg *config.Blockchain) {
			cfg.Hardforks = map[string]uint32{
				config.HFAspidochelone.String(): 0,
				config.HFBasilisk.String():      0,
			}
			cfg.P2PSigExtensions = true
		})
		check(t, mgmt, defaultCSS)
	})
	t.Run("Cockatrice enabled", func(t *testing.T) {
		mgmt := newCustomManagementClient(t, func(cfg *config.Blockchain) {
			cfg.Hardforks = map[string]uint32{
				config.HFAspidochelone.String(): 0,
				config.HFBasilisk.String():      0,
				config.HFCockatrice.String():    0,
			}
			cfg.P2PSigExtensions = true
		})
		check(t, mgmt, cockatriceCSS)
	})
	t.Run("Echidna enabled", func(t *testing.T) {
		mgmt := newCustomManagementClient(t, func(cfg *config.Blockchain) {
			cfg.Hardforks = map[string]uint32{
				config.HFAspidochelone.String(): 0,
				config.HFBasilisk.String():      0,
				config.HFCockatrice.String():    0,
				config.HFEchidna.String():       0,
			}
			cfg.P2PSigExtensions = true
		})

		check(t, mgmt, echidnaCSS)
	})
}

func TestManagement_NativeDeployUpdateNotifications(t *testing.T) {
	const cockatriceHeight = 3
	mgmt := newCustomManagementClient(t, func(cfg *config.Blockchain) {
		cfg.Hardforks = map[string]uint32{
			config.HFAspidochelone.String(): 0,
			config.HFBasilisk.String():      0,
			config.HFCockatrice.String():    cockatriceHeight,
		}
		cfg.P2PSigExtensions = true
	})
	e := mgmt.Executor

	// Check Deploy notifications.
	aer, err := mgmt.Chain.GetAppExecResults(e.GetBlockByIndex(t, 0).Hash(), trigger.OnPersist)
	require.NoError(t, err)
	require.Equal(t, 1, len(aer))
	var expected []state.NotificationEvent
	for _, name := range nativenames.All {
		switch name {
		case nativenames.Gas:
			expected = append(expected, state.NotificationEvent{
				ScriptHash: nativehashes.GasToken,
				Name:       "Transfer",
				Item: stackitem.NewArray([]stackitem.Item{
					stackitem.Null{},
					stackitem.Make(mgmt.Validator.ScriptHash()),
					stackitem.Make(mgmt.Chain.GetConfig().InitialGASSupply),
				}),
			})
		case nativenames.Neo:
			expected = append(expected, state.NotificationEvent{
				ScriptHash: nativehashes.NeoToken,
				Name:       "Transfer",
				Item: stackitem.NewArray([]stackitem.Item{
					stackitem.Null{},
					stackitem.Make(mgmt.Validator.ScriptHash()),
					stackitem.Make(native.NEOTotalSupply),
				}),
			})
		}
		expected = append(expected, state.NotificationEvent{
			ScriptHash: nativehashes.ContractManagement,
			Name:       "Deploy",
			Item: stackitem.NewArray([]stackitem.Item{
				stackitem.Make(state.CreateNativeContractHash(name)),
			}),
		})
	}
	require.Equal(t, expected, aer[0].Events)

	// Generate some blocks and check Update notifications.
	cockatriceBlock := mgmt.GenerateNewBlocks(t, cockatriceHeight)[cockatriceHeight-1]
	aer, err = mgmt.Chain.GetAppExecResults(cockatriceBlock.Hash(), trigger.OnPersist)
	require.NoError(t, err)
	require.Equal(t, 1, len(aer))
	expected = expected[:0]
	for _, name := range []string{nativenames.CryptoLib, nativenames.Neo} {
		expected = append(expected, state.NotificationEvent{
			ScriptHash: nativehashes.ContractManagement,
			Name:       "Update",
			Item: stackitem.NewArray([]stackitem.Item{
				stackitem.Make(state.CreateNativeContractHash(name)),
			}),
		})
	}
	require.Equal(t, expected, aer[0].Events)
}

func TestManagement_NativeUpdate(t *testing.T) {
	const cockatriceHeight = 3
	c := newCustomManagementClient(t, func(cfg *config.Blockchain) {
		cfg.Hardforks = map[string]uint32{
			config.HFAspidochelone.String(): 0,
			config.HFBasilisk.String():      0,
			config.HFCockatrice.String():    cockatriceHeight,
		}
		cfg.P2PSigExtensions = true
	})

	// Add some blocks up to the Cockatrice enabling height and check the default natives state.
	for range cockatriceHeight - 1 {
		c.AddNewBlock(t)
		for _, name := range nativenames.All {
			h := state.CreateNativeContractHash(name)
			cs := c.Chain.GetContractState(h)
			require.NotNil(t, cs, name)
			jBytes, err := ojson.Marshal(cs)
			require.NoError(t, err, name)
			require.Equal(t, defaultCSS[name], string(jBytes), fmt.Errorf("contract %s state mismatch", name))
		}
	}

	// Add Cockatrice block and check the updated native state.
	c.AddNewBlock(t)
	for _, name := range nativenames.All {
		h := state.CreateNativeContractHash(name)
		cs := c.Chain.GetContractState(h)
		require.NotNil(t, cs, name)
		if name == nativenames.Neo || name == nativenames.CryptoLib {
			// A tiny hack to reuse cockatriceCSS map in the check below.
			require.Equal(t, uint16(1), cs.UpdateCounter, name)
			cs.UpdateCounter--
		}
		jBytes, err := ojson.Marshal(cs)
		require.NoError(t, err, name)
		require.Equal(t, cockatriceCSS[name], string(jBytes), fmt.Errorf("contract %s state mismatch", name))
	}
}

func TestManagement_NativeUpdate_Call(t *testing.T) {
	const (
		cockatriceHeight = 3
		method           = "getCommitteeAddress"
	)
	c := newCustomNativeClient(t, nativenames.Neo, func(cfg *config.Blockchain) {
		cfg.Hardforks = map[string]uint32{
			config.HFAspidochelone.String(): 0,
			config.HFBasilisk.String():      0,
			config.HFCockatrice.String():    cockatriceHeight,
		}
		cfg.P2PSigExtensions = true
	})

	// Invoke Cockatrice-dependant method before Cockatrice should fail.
	for range cockatriceHeight - 1 {
		c.InvokeFail(t, "at instruction 45 (SYSCALL): System.Contract.Call failed: method not found: getCommitteeAddress/0", method)
	}

	// Invoke Cockatrice-dependant method at Cockatrice should be OK.
	tx := c.NewUnsignedTx(t, c.Hash, method)
	c.SignTx(t, tx, 1_0000_0000, c.Signers...)
	c.AddNewBlock(t, tx)
	c.CheckHalt(t, tx.Hash(), stackitem.Make(c.CommitteeHash))

	// Invoke Cockatrice-dependant method after Cockatrice should be OK.
	c.Invoke(t, c.CommitteeHash, method)
}

// TestBlockchain_GetNatives ensures that Blockchain's GetNatives API works as expected with
// different block heights depending on hardfork settings. This test is located here since it
// depends on defaultCSS and cockatriceCSS.
func TestBlockchain_GetNatives(t *testing.T) {
	const cockatriceHeight = 3
	bc, acc := chain.NewSingleWithCustomConfig(t, func(cfg *config.Blockchain) {
		cfg.Hardforks = map[string]uint32{
			config.HFAspidochelone.String(): 0,
			config.HFBasilisk.String():      0,
			config.HFCockatrice.String():    cockatriceHeight,
		}
		cfg.P2PSigExtensions = true
	})
	e := neotest.NewExecutor(t, bc, acc, acc)

	// Check genesis-based native contract states.
	natives := bc.GetNatives()
	require.Equal(t, len(nativenames.All), len(natives))
	for _, cs := range natives {
		csFull := state.Contract{
			ContractBase:  cs.ContractBase,
			UpdateCounter: 0,
		}
		jBytes, err := ojson.Marshal(csFull)
		require.NoError(t, err, cs.Manifest.Name)
		require.Equal(t, defaultCSS[cs.Manifest.Name], string(jBytes), fmt.Errorf("contract %s state mismatch", cs.Manifest.Name))
	}

	// Check native state after update.
	e.GenerateNewBlocks(t, cockatriceHeight)
	natives = bc.GetNatives()
	require.Equal(t, len(nativenames.All), len(natives))
	for _, cs := range natives {
		csFull := state.Contract{
			ContractBase:  cs.ContractBase,
			UpdateCounter: 0, // Since we're comparing only state.NativeContract part, set the update counter to 0 to match the cockatriceCSS.
		}
		jBytes, err := ojson.Marshal(csFull)
		require.NoError(t, err, cs.Manifest.Name)
		require.Equal(t, cockatriceCSS[cs.Manifest.Name], string(jBytes), fmt.Errorf("contract %s state mismatch", cs.Manifest.Name))
	}
}

func TestManagement_ContractCache(t *testing.T) {
	c := newCustomNativeClient(t, nativenames.Management, func(cfg *config.Blockchain) {
		cfg.Hardforks = map[string]uint32{
			config.HFAspidochelone.String(): 0,
			config.HFBasilisk.String():      0,
			config.HFCockatrice.String():    0,
			config.HFDomovoi.String():       0,
			config.HFEchidna.String():       0,
		}
	})
	managementInvoker := c.WithSigners(c.Committee)

	cs1, _ := contracts.GetTestContractState(t, pathToInternalContracts, 1, 2, c.Committee.ScriptHash())
	manifestBytes, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nefBytes, err := cs1.NEF.Bytes()
	require.NoError(t, err)

	// Deploy contract, abort the transaction and check that Management cache wasn't persisted
	// for FAULTed tx at the same block.
	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, managementInvoker.Hash, "deploy", callflag.All, nefBytes, manifestBytes)
	emit.Opcodes(w.BinWriter, opcode.ABORT)
	tx1 := managementInvoker.PrepareInvocation(t, w.Bytes(), managementInvoker.Signers)
	tx2 := managementInvoker.PrepareInvoke(t, "getContract", cs1.Hash.BytesBE())
	tx3 := managementInvoker.PrepareInvoke(t, "isContract", cs1.Hash.BytesBE())
	managementInvoker.AddNewBlock(t, tx1, tx2, tx3)
	managementInvoker.CheckFault(t, tx1.Hash(), "ABORT")
	managementInvoker.CheckHalt(t, tx2.Hash(), stackitem.Null{})
	managementInvoker.CheckHalt(t, tx3.Hash(), stackitem.Make(false)) // here

	// Deploy the contract and check that cache was persisted for HALTed transaction at the same block.
	tx1 = managementInvoker.PrepareInvoke(t, "deploy", nefBytes, manifestBytes)
	tx2 = managementInvoker.PrepareInvoke(t, "getContract", cs1.Hash.BytesBE())
	tx3 = managementInvoker.PrepareInvoke(t, "isContract", cs1.Hash.BytesBE())
	managementInvoker.AddNewBlock(t, tx1, tx2, tx3)
	managementInvoker.CheckHalt(t, tx1.Hash())
	aer, err := managementInvoker.Chain.GetAppExecResults(tx2.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, vmstate.Halt, aer[0].VMState, aer[0].FaultException)
	require.False(t, aer[0].Stack[0].Equals(stackitem.Null{}))
	managementInvoker.CheckHalt(t, tx3.Hash(), stackitem.Make(true))

	// Check that persisted contract is reachable from the next block and native contracts
	// are cached poperly.
	for _, h := range []util.Uint160{
		cs1.Hash,
		nativehashes.ContractManagement,
		nativehashes.StdLib,
		nativehashes.CryptoLib,
		nativehashes.LedgerContract,
		nativehashes.NeoToken,
		nativehashes.GasToken,
		nativehashes.PolicyContract,
		nativehashes.RoleManagement,
		nativehashes.OracleContract,
		nativehashes.Notary,
	} {
		t.Run(h.StringLE(), func(t *testing.T) {
			tx1 = managementInvoker.PrepareInvoke(t, "getContract", h.BytesBE())
			tx2 = managementInvoker.PrepareInvoke(t, "isContract", h.BytesBE())
			managementInvoker.AddNewBlock(t, tx1, tx2)
			managementInvoker.CheckHalt(t, tx1.Hash())
			aer, err = managementInvoker.Chain.GetAppExecResults(tx1.Hash(), trigger.Application)
			require.NoError(t, err)
			require.Equal(t, vmstate.Halt, aer[0].VMState, aer[0].FaultException)
			cs := aer[0].Stack[0]
			if h.Equals(nativehashes.Notary) {
				require.True(t, cs.Equals(stackitem.Null{}))
			} else {
				require.False(t, cs.Equals(stackitem.Null{}))
			}
			managementInvoker.CheckHalt(t, tx2.Hash(), stackitem.Make(!h.Equals(nativehashes.Notary)))
		})
	}
}

func TestManagement_ContractDeploy(t *testing.T) {
	c := newManagementClient(t)
	managementInvoker := c.WithSigners(c.Committee)

	cs1, _ := contracts.GetTestContractState(t, pathToInternalContracts, 1, 2, c.Committee.ScriptHash())
	manifestBytes, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nefBytes, err := cs1.NEF.Bytes()
	require.NoError(t, err)

	t.Run("no NEF", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "no valid NEF provided", "deploy", nil, manifestBytes)
	})
	t.Run("no manifest", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "no valid manifest provided", "deploy", nefBytes, nil)
	})
	t.Run("int for NEF", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "invalid NEF file", "deploy", int64(1), manifestBytes)
	})
	t.Run("zero-length NEF", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "invalid NEF file", "deploy", []byte{}, manifestBytes)
	})
	t.Run("array for NEF", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "invalid NEF file", "deploy", []any{int64(1)}, manifestBytes)
	})
	t.Run("bad script in NEF", func(t *testing.T) {
		nf, err := nef.FileFromBytes(nefBytes) // make a full copy
		require.NoError(t, err)
		nf.Script[0] = 0xff
		nf.CalculateChecksum()
		nefBad, err := nf.Bytes()
		require.NoError(t, err)
		managementInvoker.InvokeFail(t, "invalid NEF file", "deploy", nefBad, manifestBytes)
	})
	t.Run("int for manifest", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "invalid manifest", "deploy", nefBytes, int64(1))
	})
	t.Run("zero-length manifest", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "invalid manifest", "deploy", nefBytes, []byte{})
	})
	t.Run("array for manifest", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "invalid manifest", "deploy", nefBytes, []any{int64(1)})
	})
	t.Run("non-utf8 manifest", func(t *testing.T) {
		manifestBad := bytes.Replace(manifestBytes, []byte("TestMain"), []byte("\xff\xfe\xfd"), 1) // Replace name.
		managementInvoker.InvokeFail(t, "manifest is not UTF-8 compliant", "deploy", nefBytes, manifestBad)
	})
	t.Run("invalid manifest", func(t *testing.T) {
		pkey, err := keys.NewPrivateKey()
		require.NoError(t, err)

		badManifest := cs1.Manifest
		badManifest.Groups = []manifest.Group{{PublicKey: pkey.PublicKey(), Signature: make([]byte, keys.SignatureLen)}}
		manifB, err := json.Marshal(&badManifest)
		require.NoError(t, err)

		managementInvoker.InvokeFail(t, "invalid manifest", "deploy", nefBytes, manifB)
	})
	t.Run("bad methods in manifest 1", func(t *testing.T) {
		badManifest := cs1.Manifest
		badManifest.ABI.Methods = slices.Clone(cs1.Manifest.ABI.Methods)
		badManifest.ABI.Methods[0].Offset = 100500 // out of bounds
		manifB, err := json.Marshal(&badManifest)
		require.NoError(t, err)

		managementInvoker.InvokeFail(t, "method add/2: offset is out of the script range", "deploy", nefBytes, manifB)
	})
	t.Run("bad methods in manifest 2", func(t *testing.T) {
		var badManifest = cs1.Manifest
		badManifest.ABI.Methods = slices.Clone(cs1.Manifest.ABI.Methods)
		badManifest.ABI.Methods[0].Offset = len(cs1.NEF.Script) - 2 // Ends with `CALLT(X,X);RET`.

		manifB, err := json.Marshal(badManifest)
		require.NoError(t, err)

		managementInvoker.InvokeFail(t, "some methods point to wrong offsets (not to instruction boundary)", "deploy", nefBytes, manifB)
	})
	t.Run("duplicated methods in manifest 1", func(t *testing.T) {
		badManifest := cs1.Manifest
		badManifest.ABI.Methods = slices.Clone(cs1.Manifest.ABI.Methods)
		badManifest.ABI.Methods[0] = badManifest.ABI.Methods[1] // duplicates
		manifB, err := json.Marshal(&badManifest)
		require.NoError(t, err)

		managementInvoker.InvokeFail(t, "duplicate method specifications", "deploy", nefBytes, manifB)
	})
	t.Run("duplicated events in manifest 1", func(t *testing.T) {
		badManifest := cs1.Manifest
		badManifest.ABI.Methods = slices.Clone(cs1.Manifest.ABI.Methods)
		badManifest.ABI.Events = []manifest.Event{{Name: "event"}, {Name: "event"}} // duplicates
		manifB, err := json.Marshal(&badManifest)
		require.NoError(t, err)

		managementInvoker.InvokeFail(t, "duplicate event names", "deploy", nefBytes, manifB)
	})

	t.Run("not enough GAS", func(t *testing.T) {
		tx := managementInvoker.NewUnsignedTx(t, managementInvoker.Hash, "deploy", nefBytes, manifestBytes)
		managementInvoker.SignTx(t, tx, 1_0000_0000, managementInvoker.Signers...)
		managementInvoker.AddNewBlock(t, tx)
		managementInvoker.CheckFault(t, tx.Hash(), "gas limit exceeded")
	})

	si, err := cs1.ToStackItem()
	require.NoError(t, err)

	t.Run("positive", func(t *testing.T) {
		tx1 := managementInvoker.PrepareInvoke(t, "deploy", nefBytes, manifestBytes)
		tx2 := managementInvoker.PrepareInvoke(t, "getContract", cs1.Hash.BytesBE())
		managementInvoker.AddNewBlock(t, tx1, tx2)
		managementInvoker.CheckHalt(t, tx1.Hash(), si)
		managementInvoker.CheckHalt(t, tx2.Hash(), si)
		managementInvoker.CheckTxNotificationEvent(t, tx1.Hash(), 0, state.NotificationEvent{
			ScriptHash: c.NativeHash(t, nativenames.Management),
			Name:       "Deploy",
			Item:       stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray(cs1.Hash.BytesBE())}),
		})
		t.Run("_deploy called", func(t *testing.T) {
			helperInvoker := c.Executor.CommitteeInvoker(cs1.Hash)
			expected := stackitem.NewArray([]stackitem.Item{stackitem.Make("create"), stackitem.Null{}})
			expectedBytes, err := stackitem.Serialize(expected)
			require.NoError(t, err)
			helperInvoker.Invoke(t, stackitem.NewByteArray(expectedBytes), "getValue")
		})
		t.Run("get after deploy", func(t *testing.T) {
			managementInvoker.Invoke(t, si, "getContract", cs1.Hash.BytesBE())
		})
		t.Run("hasMethod after deploy", func(t *testing.T) {
			managementInvoker.Invoke(t, stackitem.NewBool(true), "hasMethod", cs1.Hash.BytesBE(), "add", 2)
			managementInvoker.Invoke(t, stackitem.NewBool(false), "hasMethod", cs1.Hash.BytesBE(), "add", 1)
			managementInvoker.Invoke(t, stackitem.NewBool(false), "hasMethod", cs1.Hash.BytesLE(), "add", 2)
		})
		t.Run("get after restore", func(t *testing.T) {
			w := io.NewBufBinWriter()
			require.NoError(t, chaindump.Dump(c.Executor.Chain, w.BinWriter, 0, c.Executor.Chain.BlockHeight()+1))
			require.NoError(t, w.Err)

			r := io.NewBinReaderFromBuf(w.Bytes())
			bc2, acc := chain.NewSingle(t)
			e2 := neotest.NewExecutor(t, bc2, acc, acc)
			managementInvoker2 := e2.CommitteeInvoker(e2.NativeHash(t, nativenames.Management))

			require.NoError(t, chaindump.Restore(bc2, r, 0, c.Executor.Chain.BlockHeight()+1, nil))
			require.NoError(t, r.Err)
			managementInvoker2.Invoke(t, si, "getContract", cs1.Hash.BytesBE())
		})
	})
	t.Run("contract already exists", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "contract already exists", "deploy", nefBytes, manifestBytes)
	})
	t.Run("failed _deploy", func(t *testing.T) {
		deployScript := []byte{byte(opcode.ABORT)}
		m := manifest.NewManifest("TestDeployAbort")
		m.ABI.Methods = []manifest.Method{
			{
				Name:   manifest.MethodDeploy,
				Offset: 0,
				Parameters: []manifest.Parameter{
					manifest.NewParameter("data", smartcontract.AnyType),
					manifest.NewParameter("isUpdate", smartcontract.BoolType),
				},
				ReturnType: smartcontract.VoidType,
			},
		}
		nefD, err := nef.NewFile(deployScript)
		require.NoError(t, err)
		nefDb, err := nefD.Bytes()
		require.NoError(t, err)
		manifD, err := json.Marshal(m)
		require.NoError(t, err)
		managementInvoker.InvokeFail(t, "ABORT", "deploy", nefDb, manifD)

		t.Run("get after failed deploy", func(t *testing.T) {
			h := state.CreateContractHash(c.CommitteeHash, nefD.Checksum, m.Name)
			managementInvoker.Invoke(t, stackitem.Null{}, "getContract", h.BytesBE())
		})
	})
	t.Run("bad _deploy", func(t *testing.T) { // invalid _deploy signature
		deployScript := []byte{byte(opcode.RET)}
		m := manifest.NewManifest("TestBadDeploy")
		m.ABI.Methods = []manifest.Method{
			{
				Name:   manifest.MethodDeploy,
				Offset: 0,
				Parameters: []manifest.Parameter{
					manifest.NewParameter("data", smartcontract.AnyType),
					manifest.NewParameter("isUpdate", smartcontract.BoolType),
				},
				ReturnType: smartcontract.ArrayType,
			},
		}
		nefD, err := nef.NewFile(deployScript)
		require.NoError(t, err)
		nefDb, err := nefD.Bytes()
		require.NoError(t, err)
		manifD, err := json.Marshal(m)
		require.NoError(t, err)
		managementInvoker.InvokeFail(t, "invalid return values count: expected 0, got 2", "deploy", nefDb, manifD)

		t.Run("get after bad _deploy", func(t *testing.T) {
			h := state.CreateContractHash(c.CommitteeHash, nefD.Checksum, m.Name)
			managementInvoker.Invoke(t, stackitem.Null{}, "getContract", h.BytesBE())
		})
	})
}

func TestManagement_StartFromHeight(t *testing.T) {
	// Create database to be able to start another chain from the same height later.
	ldbDir := t.TempDir()
	dbConfig := dbconfig.DBConfiguration{
		Type: dbconfig.LevelDB,
		LevelDBOptions: dbconfig.LevelDBOptions{
			DataDirectoryPath: ldbDir,
		},
	}
	newLevelStore, err := storage.NewLevelDBStore(dbConfig.LevelDBOptions)
	require.Nil(t, err, "NewLevelDBStore error")

	// Create blockchain and put contract state to it.
	bc, acc := chain.NewSingleWithCustomConfigAndStore(t, nil, newLevelStore, false)
	go bc.Run()
	e := neotest.NewExecutor(t, bc, acc, acc)
	c := e.CommitteeInvoker(e.NativeHash(t, nativenames.Management))
	managementInvoker := c.WithSigners(c.Committee)

	cs1, _ := contracts.GetTestContractState(t, pathToInternalContracts, 1, 2, c.CommitteeHash)
	manifestBytes, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nefBytes, err := cs1.NEF.Bytes()
	require.NoError(t, err)

	si, err := cs1.ToStackItem()
	require.NoError(t, err)

	managementInvoker.Invoke(t, si, "deploy", nefBytes, manifestBytes)
	managementInvoker.Invoke(t, si, "getContract", cs1.Hash.BytesBE())

	// Close current blockchain and start the new one from the same height with the same db.
	bc.Close()
	newLevelStore, err = storage.NewLevelDBStore(dbConfig.LevelDBOptions)
	require.NoError(t, err)
	bc2, acc := chain.NewSingleWithCustomConfigAndStore(t, nil, newLevelStore, true)
	e2 := neotest.NewExecutor(t, bc2, acc, acc)
	managementInvoker2 := e2.CommitteeInvoker(e2.NativeHash(t, nativenames.Management))

	// Check that initialisation of native Management was correctly performed.
	managementInvoker2.Invoke(t, si, "getContract", cs1.Hash.BytesBE())
}

func TestManagement_DeployManifestOverflow(t *testing.T) {
	c := newManagementClient(t)
	managementInvoker := c.WithSigners(c.Committee)

	cs1, _ := contracts.GetTestContractState(t, pathToInternalContracts, 1, 2, c.CommitteeHash)
	manif1, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nef1, err := nef.NewFile(cs1.NEF.Script)
	require.NoError(t, err)
	nef1b, err := nef1.Bytes()
	require.NoError(t, err)

	w := io.NewBufBinWriter()
	emit.Bytes(w.BinWriter, manif1)
	emit.Int(w.BinWriter, manifest.MaxManifestSize)
	emit.Opcodes(w.BinWriter, opcode.NEWBUFFER, opcode.CAT)
	emit.Bytes(w.BinWriter, nef1b)
	emit.Int(w.BinWriter, 2)
	emit.Opcodes(w.BinWriter, opcode.PACK)
	emit.AppCallNoArgs(w.BinWriter, managementInvoker.Hash, "deploy", callflag.All)
	require.NoError(t, w.Err)
	script := w.Bytes()

	tx := transaction.New(script, 0)
	tx.ValidUntilBlock = managementInvoker.Chain.BlockHeight() + 1
	managementInvoker.SignTx(t, tx, 100_0000_0000, managementInvoker.Signers...)
	managementInvoker.AddNewBlock(t, tx)
	managementInvoker.CheckFault(t, tx.Hash(), fmt.Sprintf("invalid manifest: len is %d (max %d)", manifest.MaxManifestSize+len(manif1), manifest.MaxManifestSize))
}

func TestManagement_ContractDeployAndUpdateWithParameter(t *testing.T) {
	c := newManagementClient(t)
	managementInvoker := c.WithSigners(c.Committee)

	cs1, _ := contracts.GetTestContractState(t, pathToInternalContracts, 1, 2, c.CommitteeHash)
	cs1.Manifest.Permissions = []manifest.Permission{*manifest.NewPermission(manifest.PermissionWildcard)}
	cs1.ID = 1
	cs1.Hash = state.CreateContractHash(c.CommitteeHash, cs1.NEF.Checksum, cs1.Manifest.Name)
	manif1, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nef1b, err := cs1.NEF.Bytes()
	require.NoError(t, err)

	si, err := cs1.ToStackItem()
	require.NoError(t, err)
	managementInvoker.Invoke(t, si, "deploy", nef1b, manif1)
	helperInvoker := c.Executor.CommitteeInvoker(cs1.Hash)

	t.Run("_deploy called", func(t *testing.T) {
		expected := stackitem.NewArray([]stackitem.Item{stackitem.Make("create"), stackitem.Null{}})
		expectedBytes, err := stackitem.Serialize(expected)
		require.NoError(t, err)
		helperInvoker.Invoke(t, stackitem.NewByteArray(expectedBytes), "getValue")
	})

	cs1.NEF.Script = append(cs1.NEF.Script, byte(opcode.RET))
	cs1.NEF.Checksum = cs1.NEF.CalculateChecksum()
	nef1b, err = cs1.NEF.Bytes()
	require.NoError(t, err)
	cs1.UpdateCounter++

	helperInvoker.Invoke(t, stackitem.Null{}, "update", nef1b, nil, "new data")

	t.Run("_deploy called", func(t *testing.T) {
		expected := stackitem.NewArray([]stackitem.Item{stackitem.Make("update"), stackitem.Make("new data")})
		expectedBytes, err := stackitem.Serialize(expected)
		require.NoError(t, err)
		helperInvoker.Invoke(t, stackitem.NewByteArray(expectedBytes), "getValue")
	})
}

func TestManagement_ContractUpdate(t *testing.T) {
	c := newManagementClient(t)
	managementInvoker := c.WithSigners(c.Committee)

	cs1, _ := contracts.GetTestContractState(t, pathToInternalContracts, 1, 2, c.CommitteeHash)
	// Allow calling management contract.
	cs1.Manifest.Permissions = []manifest.Permission{*manifest.NewPermission(manifest.PermissionWildcard)}
	manifestBytes, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nefBytes, err := cs1.NEF.Bytes()
	require.NoError(t, err)

	si, err := cs1.ToStackItem()
	require.NoError(t, err)
	managementInvoker.Invoke(t, si, "deploy", nefBytes, manifestBytes)
	helperInvoker := c.Executor.CommitteeInvoker(cs1.Hash)

	t.Run("unknown contract", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "contract doesn't exist", "update", nefBytes, manifestBytes)
	})
	t.Run("zero-length NEF", func(t *testing.T) {
		helperInvoker.InvokeFail(t, "invalid NEF file: empty", "update", []byte{}, manifestBytes)
	})
	t.Run("zero-length manifest", func(t *testing.T) {
		helperInvoker.InvokeFail(t, "invalid manifest: empty", "update", nefBytes, []byte{})
	})
	t.Run("no real params", func(t *testing.T) {
		helperInvoker.InvokeFail(t, "both NEF and manifest are nil", "update", nil, nil)
	})
	t.Run("invalid manifest", func(t *testing.T) {
		pkey, err := keys.NewPrivateKey()
		require.NoError(t, err)

		var badManifest = cs1.Manifest
		badManifest.Groups = []manifest.Group{{PublicKey: pkey.PublicKey(), Signature: make([]byte, keys.SignatureLen)}}
		manifB, err := json.Marshal(badManifest)
		require.NoError(t, err)

		helperInvoker.InvokeFail(t, "invalid manifest: incorrect group signature", "update", nefBytes, manifB)
	})
	t.Run("manifest and script mismatch", func(t *testing.T) {
		nf, err := nef.FileFromBytes(nefBytes) // Make a full copy.
		require.NoError(t, err)
		nf.Script = append(nf.Script, byte(opcode.RET))
		copy(nf.Script[1:], nf.Script)  // Now all method offsets are wrong.
		nf.Script[0] = byte(opcode.RET) // Even though the script is correct.
		nf.CalculateChecksum()
		nefnew, err := nf.Bytes()
		require.NoError(t, err)
		helperInvoker.InvokeFail(t, "invalid NEF file: checksum verification failure", "update", nefnew, manifestBytes)
	})

	t.Run("change name", func(t *testing.T) {
		var badManifest = cs1.Manifest
		badManifest.Name += "tail"
		manifB, err := json.Marshal(badManifest)
		require.NoError(t, err)

		helperInvoker.InvokeFail(t, "contract name can't be changed", "update", nefBytes, manifB)
	})

	cs1.NEF.Script = append(cs1.NEF.Script, byte(opcode.RET))
	cs1.NEF.Checksum = cs1.NEF.CalculateChecksum()
	nefBytes, err = cs1.NEF.Bytes()
	require.NoError(t, err)
	cs1.UpdateCounter++
	si, err = cs1.ToStackItem()
	require.NoError(t, err)

	t.Run("update script, positive", func(t *testing.T) {
		tx1 := helperInvoker.PrepareInvoke(t, "update", nefBytes, nil)
		tx2 := managementInvoker.PrepareInvoke(t, "getContract", cs1.Hash.BytesBE())
		managementInvoker.AddNewBlock(t, tx1, tx2)
		managementInvoker.CheckHalt(t, tx1.Hash(), stackitem.Null{})
		managementInvoker.CheckHalt(t, tx2.Hash(), si)
		managementInvoker.CheckTxNotificationEvent(t, tx1.Hash(), 0, state.NotificationEvent{
			ScriptHash: c.NativeHash(t, nativenames.Management),
			Name:       "Update",
			Item:       stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray(cs1.Hash.BytesBE())}),
		})
		t.Run("_deploy called", func(t *testing.T) {
			helperInvoker := c.Executor.CommitteeInvoker(cs1.Hash)
			expected := stackitem.NewArray([]stackitem.Item{stackitem.Make("update"), stackitem.Null{}})
			expectedBytes, err := stackitem.Serialize(expected)
			require.NoError(t, err)
			helperInvoker.Invoke(t, stackitem.NewByteArray(expectedBytes), "getValue")
		})
		t.Run("check contract", func(t *testing.T) {
			managementInvoker.Invoke(t, si, "getContract", cs1.Hash.BytesBE())
		})
	})

	cs1.Manifest.Extra = []byte(`"update me"`)
	manifestBytes, err = json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	cs1.UpdateCounter++
	si, err = cs1.ToStackItem()
	require.NoError(t, err)

	t.Run("update manifest, positive", func(t *testing.T) {
		updHash := helperInvoker.Invoke(t, stackitem.Null{}, "update", nil, manifestBytes)
		helperInvoker.CheckTxNotificationEvent(t, updHash, 0, state.NotificationEvent{
			ScriptHash: helperInvoker.NativeHash(t, nativenames.Management),
			Name:       "Update",
			Item:       stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray(cs1.Hash.BytesBE())}),
		})
		t.Run("check contract", func(t *testing.T) {
			managementInvoker.Invoke(t, si, "getContract", cs1.Hash.BytesBE())
		})
	})

	cs1.NEF.Script = append(cs1.NEF.Script, byte(opcode.ABORT))
	cs1.NEF.Checksum = cs1.NEF.CalculateChecksum()
	nefBytes, err = cs1.NEF.Bytes()
	require.NoError(t, err)
	cs1.Manifest.Extra = []byte(`"update me once more"`)
	manifestBytes, err = json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	cs1.UpdateCounter++
	si, err = cs1.ToStackItem()
	require.NoError(t, err)

	t.Run("update both script and manifest", func(t *testing.T) {
		updHash := helperInvoker.Invoke(t, stackitem.Null{}, "update", nefBytes, manifestBytes)
		helperInvoker.CheckTxNotificationEvent(t, updHash, 0, state.NotificationEvent{
			ScriptHash: helperInvoker.NativeHash(t, nativenames.Management),
			Name:       "Update",
			Item:       stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray(cs1.Hash.BytesBE())}),
		})
		t.Run("check contract", func(t *testing.T) {
			managementInvoker.Invoke(t, si, "getContract", cs1.Hash.BytesBE())
		})
	})
}

func TestManagement_GetContract(t *testing.T) {
	c := newManagementClient(t)
	managementInvoker := c.WithSigners(c.Committee)

	cs1, _ := contracts.GetTestContractState(t, pathToInternalContracts, 1, 2, c.CommitteeHash)
	manifestBytes, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nefBytes, err := cs1.NEF.Bytes()
	require.NoError(t, err)

	si, err := cs1.ToStackItem()
	require.NoError(t, err)
	managementInvoker.Invoke(t, si, "deploy", nefBytes, manifestBytes)

	t.Run("bad parameter type", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "invalid conversion: Array/ByteString", "getContract", []any{int64(1)})
	})
	t.Run("not a hash", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "expected byte size of 20 got 3", "getContract", []byte{1, 2, 3})
	})
	t.Run("positive", func(t *testing.T) {
		managementInvoker.Invoke(t, si, "getContract", cs1.Hash.BytesBE())
	})
	t.Run("by ID, bad parameter type", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "invalid conversion: Array/Integer", "getContractById", []any{int64(1)})
	})
	t.Run("by ID, bad num", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "id is not a correct int32", "getContractById", []byte{1, 2, 3, 4, 5})
	})
	t.Run("by ID, positive", func(t *testing.T) {
		managementInvoker.Invoke(t, si, "getContractById", cs1.ID)
	})
	t.Run("by ID, native", func(t *testing.T) {
		csm := managementInvoker.Executor.Chain.GetContractState(managementInvoker.Hash)
		require.NotNil(t, csm)
		sim, err := csm.ToStackItem()
		require.NoError(t, err)
		managementInvoker.Invoke(t, sim, "getContractById", -1)
	})
	t.Run("by ID, empty", func(t *testing.T) {
		managementInvoker.Invoke(t, stackitem.Null{}, "getContractById", -100)
	})
	t.Run("contract hashes", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.AppCall(w.BinWriter, managementInvoker.Hash, "getContractHashes", callflag.All)
		emit.Opcodes(w.BinWriter, opcode.DUP) // Iterator.
		emit.Syscall(w.BinWriter, interopnames.SystemIteratorNext)
		emit.Opcodes(w.BinWriter, opcode.ASSERT) // Has one element.
		emit.Opcodes(w.BinWriter, opcode.DUP)    // Iterator.
		emit.Syscall(w.BinWriter, interopnames.SystemIteratorValue)
		emit.Opcodes(w.BinWriter, opcode.SWAP) // Iterator to the top.
		emit.Syscall(w.BinWriter, interopnames.SystemIteratorNext)
		emit.Opcodes(w.BinWriter, opcode.NOT)
		emit.Opcodes(w.BinWriter, opcode.ASSERT) // No more elements, single value left on the stack.
		require.NoError(t, w.Err)
		h := managementInvoker.InvokeScript(t, w.Bytes(), managementInvoker.Signers)
		managementInvoker.Executor.CheckHalt(t, h, stackitem.NewStruct([]stackitem.Item{stackitem.Make([]byte{0, 0, 0, 1}), stackitem.Make(cs1.Hash.BytesBE())}))
	})
}

func TestManagement_ContractDestroy(t *testing.T) {
	c := newManagementClient(t)
	managementInvoker := c.WithSigners(c.Committee)

	cs1, _ := contracts.GetTestContractState(t, pathToInternalContracts, 1, 2, c.CommitteeHash)
	// Allow calling management contract.
	cs1.Manifest.Permissions = []manifest.Permission{*manifest.NewPermission(manifest.PermissionWildcard)}
	manifestBytes, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nefBytes, err := cs1.NEF.Bytes()
	require.NoError(t, err)

	si, err := cs1.ToStackItem()
	require.NoError(t, err)
	managementInvoker.Invoke(t, si, "deploy", nefBytes, manifestBytes)
	helperInvoker := c.Executor.CommitteeInvoker(cs1.Hash)

	t.Run("no contract", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "key not found", "destroy")
	})
	t.Run("positive", func(t *testing.T) {
		dstrHash := helperInvoker.Invoke(t, stackitem.Null{}, "destroy")
		helperInvoker.CheckTxNotificationEvent(t, dstrHash, 0, state.NotificationEvent{
			ScriptHash: helperInvoker.NativeHash(t, nativenames.Management),
			Name:       "Destroy",
			Item:       stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray(cs1.Hash.BytesBE())}),
		})
		t.Run("check contract", func(t *testing.T) {
			managementInvoker.Invoke(t, stackitem.Null{}, "getContract", cs1.Hash.BytesBE())
		})
		// deploy after destroy should fail
		managementInvoker.InvokeFail(t, fmt.Sprintf("the contract %s has been blocked", cs1.Hash.StringLE()), "deploy", nefBytes, manifestBytes)
	})
}
