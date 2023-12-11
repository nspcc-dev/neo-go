package wallet

import (
	"crypto/elliptic"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"

	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

type (
	walletV2 struct {
		Version  string            `json:"version"`
		Accounts []accountV2       `json:"accounts"`
		Scrypt   keys.ScryptParams `json:"scrypt"`
		Extra    wallet.Extra      `json:"extra"`
	}
	accountV2 struct {
		Address      string `json:"address"`
		EncryptedWIF string `json:"key"`
		Label        string `json:"label"`
		Contract     *struct {
			Script     string                 `json:"script"`
			Parameters []wallet.ContractParam `json:"parameters"`
			Deployed   bool                   `json:"deployed"`
		} `json:"contract"`
		Locked  bool `json:"lock"`
		Default bool `json:"isdefault"`
	}
)

// newWalletV2FromFile reads a Neo Legacy wallet from the file.
// This should be used read-only, no operations are supported on the returned wallet.
func newWalletV2FromFile(path string, configPath string) (*walletV2, *string, error) {
	if len(path) != 0 && len(configPath) != 0 {
		return nil, nil, errConflictingWalletFlags
	}
	if len(path) == 0 && len(configPath) == 0 {
		return nil, nil, errNoPath
	}
	var pass *string
	if len(configPath) != 0 {
		cfg, err := options.ReadWalletConfig(configPath)
		if err != nil {
			return nil, nil, err
		}
		path = cfg.Path
		pass = &cfg.Password
	}
	file, err := os.OpenFile(path, os.O_RDWR, os.ModeAppend)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	wall := new(walletV2)
	return wall, pass, json.NewDecoder(file).Decode(wall)
}

const simpleSigLen = 35

func (a *accountV2) convert(pass string, scrypt keys.ScryptParams) (*wallet.Account, error) {
	address.Prefix = address.NEO2Prefix
	priv, err := keys.NEP2Decrypt(a.EncryptedWIF, pass, scrypt)
	if err != nil {
		return nil, err
	}

	address.Prefix = address.NEO3Prefix
	newAcc := wallet.NewAccountFromPrivateKey(priv)
	if a.Contract != nil {
		script, err := hex.DecodeString(a.Contract.Script)
		if err != nil {
			return nil, err
		}
		// If it is a simple signature script, a newAcc does already have it.
		if len(script) != simpleSigLen {
			nsigs, pubs, ok := parseMultisigContract(script)
			if !ok {
				return nil, errors.New("invalid multisig contract")
			}
			script, err := smartcontract.CreateMultiSigRedeemScript(nsigs, pubs)
			if err != nil {
				return nil, errors.New("can't create new multisig contract")
			}
			newAcc.Contract.Script = script
			newAcc.Contract.Parameters = a.Contract.Parameters
			newAcc.Contract.Deployed = a.Contract.Deployed
		}
	}
	newAcc.Address = address.Uint160ToString(newAcc.Contract.ScriptHash())
	newAcc.Default = a.Default
	newAcc.Label = a.Label
	newAcc.Locked = a.Locked
	return newAcc, newAcc.Encrypt(pass, scrypt)
}

const (
	opPush1         = 0x51
	opPush16        = 0x60
	opPushBytes1    = 0x01
	opPushBytes2    = 0x02
	opPushBytes33   = 0x21
	opCheckMultisig = 0xAE
	opRet           = 0x66
)

func getNumOfThingsFromInstr(script []byte) (int, int, bool) {
	var op = script[0]
	switch {
	case opPush1 <= op && op <= opPush16:
		return int(op-opPush1) + 1, 1, true
	case op == opPushBytes1 && len(script) >= 2:
		return int(script[1]), 2, true
	case op == opPushBytes2 && len(script) >= 3:
		return int(binary.LittleEndian.Uint16(script[1:])), 3, true
	default:
		return 0, 0, false
	}
}

const minMultisigLen = 37

// parseMultisigContract accepts a multisig verification script from Neo2
// and returns a list of public keys in the same order as in the script.
func parseMultisigContract(script []byte) (int, keys.PublicKeys, bool) {
	// It should contain at least 1 public key.
	if len(script) < minMultisigLen {
		return 0, nil, false
	}

	nsigs, offset, ok := getNumOfThingsFromInstr(script)
	if !ok {
		return 0, nil, false
	}
	var pubs [][]byte
	var nkeys int
	for offset < len(script) && script[offset] == opPushBytes33 {
		if len(script[offset:]) < 34 {
			return 0, nil, false
		}
		pubs = append(pubs, script[offset+1:offset+34])
		nkeys++
		offset += 34
	}
	if nkeys < nsigs || offset >= len(script) {
		return 0, nil, false
	}
	nkeys2, off, ok := getNumOfThingsFromInstr(script[offset:])
	if !ok || nkeys2 != nkeys {
		return 0, nil, false
	}
	end := script[offset+off:]
	switch {
	case len(end) == 1 && end[0] == opCheckMultisig:
	case len(end) == 2 && end[0] == opCheckMultisig && end[1] == opRet:
	default:
		return 0, nil, false
	}

	ret := make(keys.PublicKeys, len(pubs))
	for i := range pubs {
		pub, err := keys.NewPublicKeyFromBytes(pubs[i], elliptic.P256())
		if err != nil {
			return 0, nil, false
		}
		ret[i] = pub
	}
	return nsigs, ret, true
}
