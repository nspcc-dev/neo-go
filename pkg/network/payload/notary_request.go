package payload

import (
	"bytes"
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// P2PNotaryRequest contains main and fallback transactions for the Notary service.
type P2PNotaryRequest struct {
	MainTransaction     *transaction.Transaction
	FallbackTransaction *transaction.Transaction
	Network             netmode.Magic

	Witness transaction.Witness

	hash       util.Uint256
	signedHash util.Uint256
}

// NewP2PNotaryRequestFromBytes decodes P2PNotaryRequest from the given bytes.
func NewP2PNotaryRequestFromBytes(network netmode.Magic, b []byte) (*P2PNotaryRequest, error) {
	req := &P2PNotaryRequest{Network: network}
	br := io.NewBinReaderFromBuf(b)
	req.DecodeBinary(br)
	if br.Err != nil {
		return nil, br.Err
	}
	_ = br.ReadB()
	if br.Err == nil {
		return nil, errors.New("additional data after the payload")
	}
	return req, nil
}

// Bytes returns serialized P2PNotaryRequest payload.
func (r *P2PNotaryRequest) Bytes() ([]byte, error) {
	buf := io.NewBufBinWriter()
	r.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return nil, buf.Err
	}
	return buf.Bytes(), nil
}

// Hash returns payload's hash.
func (r *P2PNotaryRequest) Hash() util.Uint256 {
	if r.hash.Equals(util.Uint256{}) {
		if r.createHash() != nil {
			panic("failed to compute hash!")
		}
	}
	return r.hash
}

// GetSignedHash returns a hash of the payload used to verify it.
func (r *P2PNotaryRequest) GetSignedHash() util.Uint256 {
	if r.signedHash.Equals(util.Uint256{}) {
		if r.createHash() != nil {
			panic("failed to compute hash!")
		}
	}
	return r.signedHash
}

// GetSignedPart returns a part of the payload which must be signed.
func (r *P2PNotaryRequest) GetSignedPart() []byte {
	if r.hash.Equals(util.Uint256{}) {
		if r.createHash() != nil {
			panic("failed to compute hash!")
		}
	}
	buf := io.NewBufBinWriter()
	buf.WriteU32LE(uint32(r.Network))
	buf.WriteBytes(r.hash[:])
	return buf.Bytes()
}

// createHash creates hash of the payload.
func (r *P2PNotaryRequest) createHash() error {
	buf := io.NewBufBinWriter()
	r.encodeHashableFields(buf.BinWriter)
	r.hash = hash.Sha256(buf.Bytes())
	signed := r.GetSignedPart()
	r.signedHash = hash.Sha256(signed)
	return nil
}

// DecodeBinaryUnsigned reads payload from w excluding signature.
func (r *P2PNotaryRequest) decodeHashableFields(br *io.BinReader) {
	r.MainTransaction = &transaction.Transaction{}
	r.FallbackTransaction = &transaction.Transaction{}
	r.MainTransaction.DecodeBinary(br)
	r.FallbackTransaction.DecodeBinary(br)
	if br.Err == nil {
		br.Err = r.isValid()
	}
	if br.Err == nil {
		br.Err = r.createHash()
	}
}

// DecodeBinary implements io.Serializable interface.
func (r *P2PNotaryRequest) DecodeBinary(br *io.BinReader) {
	r.decodeHashableFields(br)
	if br.Err == nil {
		r.Witness.DecodeBinary(br)
	}
}

// encodeHashableFields writes payload to w excluding signature.
func (r *P2PNotaryRequest) encodeHashableFields(bw *io.BinWriter) {
	r.MainTransaction.EncodeBinary(bw)
	r.FallbackTransaction.EncodeBinary(bw)
}

// EncodeBinary implements Serializable interface.
func (r *P2PNotaryRequest) EncodeBinary(bw *io.BinWriter) {
	r.encodeHashableFields(bw)
	r.Witness.EncodeBinary(bw)
}

func (r *P2PNotaryRequest) isValid() error {
	nKeysMain := r.MainTransaction.GetAttributes(transaction.NotaryAssistedT)
	if len(nKeysMain) == 0 {
		return errors.New("main transaction should have NotaryAssisted attribute")
	}
	if nKeysMain[0].Value.(*transaction.NotaryAssisted).NKeys == 0 {
		return errors.New("main transaction should have NKeys > 0")
	}
	if len(r.FallbackTransaction.Signers) != 2 {
		return errors.New("fallback transaction should have two signers")
	}
	if len(r.FallbackTransaction.Scripts[0].InvocationScript) != 66 ||
		len(r.FallbackTransaction.Scripts[0].VerificationScript) != 0 ||
		!bytes.HasPrefix(r.FallbackTransaction.Scripts[0].InvocationScript, []byte{byte(opcode.PUSHDATA1), 64}) {
		return errors.New("fallback transaction has invalid dummy Notary witness")
	}
	if !r.FallbackTransaction.HasAttribute(transaction.NotValidBeforeT) {
		return errors.New("fallback transactions should have NotValidBefore attribute")
	}
	conflicts := r.FallbackTransaction.GetAttributes(transaction.ConflictsT)
	if len(conflicts) != 1 {
		return errors.New("fallback transaction should have one Conflicts attribute")
	}
	if conflicts[0].Value.(*transaction.Conflicts).Hash != r.MainTransaction.Hash() {
		return errors.New("fallback transaction does not conflicts with the main transaction")
	}
	nKeysFallback := r.FallbackTransaction.GetAttributes(transaction.NotaryAssistedT)
	if len(nKeysFallback) == 0 {
		return errors.New("fallback transaction should have NotaryAssisted attribute")
	}
	if nKeysFallback[0].Value.(*transaction.NotaryAssisted).NKeys != 0 {
		return errors.New("fallback transaction should have NKeys = 0")
	}
	if r.MainTransaction.ValidUntilBlock != r.FallbackTransaction.ValidUntilBlock {
		return errors.New("both main and fallback transactions should have the same ValidUntil value")
	}
	return nil
}
