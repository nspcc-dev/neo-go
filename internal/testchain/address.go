package testchain

import (
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/stretchr/testify/require"
)

// privNetKeys is a list of unencrypted WIFs sorted by public key.
var privNetKeys = []string{
	"KzfPUYDC9n2yf4fK5ro4C8KMcdeXtFuEnStycbZgX3GomiUsvX6W",
	"KzgWE3u3EDp13XPXXuTKZxeJ3Gi8Bsm8f9ijY3ZsCKKRvZUo1Cdn",
	"KxyjQ8eUa4FHt3Gvioyt1Wz29cTUrE4eTqX3yFSk1YFCsPL8uNsY",
	"L2oEXKRAAMiPEZukwR5ho2S6SMeQLhcK9mF71ZnF7GvT8dU4Kkgz",

	// Provide 2 committee extra members so that committee address differs from
	// the validators one.
	"L1Tr1iq5oz1jaFaMXP21sHDkJYDDkuLtpvQ4wRf1cjKvJYvnvpAb",
	"Kz6XTUrExy78q8f4MjDHnwz8fYYyUE8iPXwPRAkHa3qN2JcHYm7e",
}

// ValidatorsCount returns number of validators in the testchain.
const ValidatorsCount = 4

var (
	// ids maps validators order by public key sorting to validators ID.
	// which is an order of the validator in the StandByValidators list.
	ids = []int{1, 3, 0, 2, 4, 5}
	// orders maps to validators id to it's order by public key sorting.
	orders = []int{2, 0, 3, 1, 4, 5}
)

// Size returns testchain initial validators amount.
func Size() int {
	return ValidatorsCount
}

// CommitteeSize returns testchain committee size.
func CommitteeSize() int {
	return len(privNetKeys)
}

// IDToOrder returns node's order in privnet.
func IDToOrder(id int) int {
	return orders[id]
}

// WIF returns unencrypted wif of the specified validator.
func WIF(i int) string {
	return privNetKeys[i]
}

// PrivateKey returns private key of node #i.
func PrivateKey(i int) *keys.PrivateKey {
	wif := WIF(i)
	priv, err := keys.NewPrivateKeyFromWIF(wif)
	if err != nil {
		panic(err)
	}
	return priv
}

// PrivateKeyByID returns private keys of a node with the specified id.
func PrivateKeyByID(id int) *keys.PrivateKey {
	return PrivateKey(IDToOrder(id))
}

// MultisigVerificationScript returns script hash of the consensus multisig address.
func MultisigVerificationScript() []byte {
	var pubs keys.PublicKeys
	for i := range privNetKeys[:ValidatorsCount] {
		priv := PrivateKey(ids[i])
		pubs = append(pubs, priv.PublicKey())
	}

	script, err := smartcontract.CreateDefaultMultiSigRedeemScript(pubs)
	if err != nil {
		panic(err)
	}
	return script
}

// MultisigScriptHash returns consensus address as Uint160.
func MultisigScriptHash() util.Uint160 {
	return hash.Hash160(MultisigVerificationScript())
}

// MultisigAddress return consensus address as string.
func MultisigAddress() string {
	return address.Uint160ToString(MultisigScriptHash())
}

// CommitteeVerificationScript returns script hash of the committee multisig address.
func CommitteeVerificationScript() []byte {
	var pubs keys.PublicKeys
	for i := range privNetKeys {
		priv := PrivateKey(ids[i])
		pubs = append(pubs, priv.PublicKey())
	}

	script, err := smartcontract.CreateMajorityMultiSigRedeemScript(pubs)
	if err != nil {
		panic(err)
	}
	return script
}

// CommitteeScriptHash returns committee address as Uint160.
func CommitteeScriptHash() util.Uint160 {
	return hash.Hash160(CommitteeVerificationScript())
}

// CommitteeAddress return committee address as string.
func CommitteeAddress() string {
	return address.Uint160ToString(CommitteeScriptHash())
}

// Sign signs data by all consensus nodes and returns invocation script.
func Sign(h hash.Hashable) []byte {
	buf := io.NewBufBinWriter()
	for i := 0; i < 3; i++ {
		pKey := PrivateKey(i)
		sig := pKey.SignHashable(uint32(Network()), h)
		if len(sig) != 64 {
			panic("wrong signature length")
		}
		emit.Bytes(buf.BinWriter, sig)
	}
	return buf.Bytes()
}

// SignCommittee signs data by a majority of committee members.
func SignCommittee(h hash.Hashable) []byte {
	buf := io.NewBufBinWriter()
	for i := 0; i < CommitteeSize()/2+1; i++ {
		pKey := PrivateKey(i)
		sig := pKey.SignHashable(uint32(Network()), h)
		if len(sig) != 64 {
			panic("wrong signature length")
		}
		emit.Bytes(buf.BinWriter, sig)
	}
	return buf.Bytes()
}

// NewBlock creates new block for the given blockchain with the given offset
// (usually, 1), primary node index and transactions.
func NewBlock(t *testing.T, bc blockchainer.Blockchainer, offset uint32, primary uint32, txs ...*transaction.Transaction) *block.Block {
	witness := transaction.Witness{VerificationScript: MultisigVerificationScript()}
	height := bc.BlockHeight()
	h := bc.GetHeaderHash(int(height))
	hdr, err := bc.GetHeader(h)
	require.NoError(t, err)
	b := &block.Block{
		Header: block.Header{
			PrevHash:      hdr.Hash(),
			Timestamp:     (uint64(time.Now().UTC().Unix()) + uint64(hdr.Index)) * 1000,
			Index:         hdr.Index + offset,
			PrimaryIndex:  byte(primary),
			NextConsensus: witness.ScriptHash(),
			Script:        witness,
			Network:       bc.GetConfig().Magic,
		},
		Transactions: txs,
	}
	b.RebuildMerkleRoot()

	b.Script.InvocationScript = Sign(b)
	return b
}
