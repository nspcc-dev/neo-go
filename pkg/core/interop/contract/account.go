package contract

import (
	"crypto/elliptic"
	"errors"
	"fmt"
	"math"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Prices since [config.HFHuyao] in 10^-11 GAS units.
const (
	CreateMultisigAccountPricePerKey = 10581
	CreateMultisigAccountBasePrice   = 62153
)

// CreateMultisigAccount calculates multisig contract scripthash for a
// given m and a set of public keys.
func CreateMultisigAccount(ic *interop.Context) error {
	m := ic.VM.Estack().Pop().BigInt()
	m64 := m.Int64()
	if !m.IsInt64() || m64 > math.MaxInt32 || m64 < math.MinInt32 {
		return errors.New("m is out of int32 range")
	}
	arr := ic.VM.Estack().Pop().Array()
	rawPubs := make([][]byte, len(arr))
	for i, pk := range arr {
		b, err := pk.TryBytes()
		if err != nil {
			return fmt.Errorf("failed to retrieve public key bytes: %w", err)
		}
		rawPubs[i] = b
	}
	if m64 < 1 || m64 > int64(len(rawPubs)) || len(rawPubs) > smartcontract.MaxMultiSigRedeemScriptKeys {
		return fmt.Errorf("invalid multisig parameters: m=%d, public keys count=%d", m64, len(rawPubs))
	}
	if ic.IsHardforkEnabled(config.HFHuyao) {
		if err := ic.VM.AddFemtoGas(ic.BaseExecFee() * CreateMultisigAccountPricePerKey * int64(len(rawPubs))); err != nil {
			return err
		}
	} else {
		var invokeFee int64
		if ic.IsHardforkEnabled(config.HFAspidochelone) {
			invokeFee = fee.ECDSAVerifyPrice * int64(len(rawPubs))
		} else {
			invokeFee = 1 << 8
		}
		invokeFee *= ic.BaseExecFee()
		if err := ic.VM.AddPicoGas(invokeFee); err != nil {
			return err
		}
	}
	pubs := make(keys.PublicKeys, len(rawPubs))
	for i, b := range rawPubs {
		p, err := keys.NewPublicKeyFromBytes(b, elliptic.P256())
		if err != nil {
			return fmt.Errorf("failed to convert public key from bytes: %w", err)
		}
		pubs[i] = p
	}
	script, err := smartcontract.CreateMultiSigRedeemScript(int(m64), pubs)
	if err != nil {
		return err
	}
	ic.VM.Estack().PushItem(stackitem.NewByteArray(hash.Hash160(script).BytesBE()))
	return nil
}

// CreateStandardAccount calculates contract scripthash for a given public key.
func CreateStandardAccount(ic *interop.Context) error {
	h := ic.VM.Estack().Pop().Bytes()
	p, err := keys.NewPublicKeyFromBytes(h, elliptic.P256())
	if err != nil {
		return err
	}
	if !ic.IsHardforkEnabled(config.HFHuyao) {
		var invokeFee int64
		if ic.IsHardforkEnabled(config.HFAspidochelone) {
			invokeFee = fee.ECDSAVerifyPrice
		} else {
			invokeFee = 1 << 8
		}
		invokeFee *= ic.BaseExecFee()
		if err := ic.VM.AddPicoGas(invokeFee); err != nil {
			return err
		}
	}
	ic.VM.Estack().PushItem(stackitem.NewByteArray(p.GetScriptHash().BytesBE()))
	return nil
}
