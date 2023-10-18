// Package zkpbinding contains a set of helper functions aimed to generate and
// interact with Verifier smart contract written in Go and using Groth-16 proving
// system over BLS12-381 elliptic curve to verify proofs. Package zkpbinding
// provides the Veifier contract generation functionality itself as far as a
// helper that converts groth16.Proof to the Verifier-specific set of arguments.
//
// Please, check out the example of zkpbinding package usage to generate and
// verify proofs on the Neo chain:
// https://github.com/nspcc-dev/neo-go/blob/91c928e8d35164055e5b2e8efbc898440cc2b486/examples/zkp/cubic_circuit/README.md
package zkpbinding

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"text/template"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/backend/groth16"
	curve "github.com/consensys/gnark/backend/groth16/bls12-381"
	"github.com/consensys/gnark/backend/witness"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/binding"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
)

// Config represents a configuration for Verifier Go smart contract generator.
type Config struct {
	// VerifyingKey must be a Groth-16 BLS12-381 specific verifier key,
	// parameters of which will be used to generate Verifier Neo smart contract.
	VerifyingKey groth16.VerifyingKey
	// Output is a writer for the resulting Verifier Go smart contract, it must
	// not be nil.
	Output io.Writer
	// CfgOutput is a writer for the resulting Verifier Go smart contract YAML
	// configuration file needed to compile the contract. It may be nil if the
	// contract configuration file generation should be omitted.
	CfgOutput io.Writer
	// GomodOutput is a writer for the resulting go.mod file of the Verifier Go
	// smart contract needed to compile it. It may be nil if the go.mod file
	// generation should be omitted.
	GomodOutput io.Writer
	// GosumOutput is a writer for the resulting go.sum file of the Verifier Go
	// smart contract needed to compile it. It may be nil if the go.sum file
	// generation should be omitted.
	GosumOutput io.Writer
}

// A set of Verifier smart contract template related constants.
const (
	// goVerificationTmpl is a verification smart contract template. It contains
	// a single `verifyProof` method that accepts a proof represented as three
	// BLS12-381 curve points and public information required for verification
	// represented as a list of serialized 32-bytes field elements in the LE form.
	// The boolean result of `verifyProof` is either `true` (if the proof is
	// valid) or `false` (if the proof is invalid). The smart contract generated
	// from this template can be immediately compiled without any additional
	// changes using NeoGo compiler, deployed to the Neo chain and invoked. The
	// verification contract is circuit-specific, i.e. corresponds to a specific
	// single constraint system. Thus, every new circuit requires vew verification
	// contract to be generated and deployed to the chain.
	goVerificationTmpl = `// Package main contains verification smart contract that uses Neo BLS12-381
// curves interoperability functionality to verify provided proof against provided
// public input. The contract contains a single 'verifyProof' method that accepts
// a proof represented as three BLS12-381 curve points and public witnesses
// required for verification represented as a list of serialized 32-bytes field
// elements in the LE form. This contract is circuit-specific and can not be used
// to verify other circuits.
//
// Use NeoGo smart contract compiler to compile this contract:
// https://github.com/nspcc-dev/neo-go/blob/master/docs/compiler.md#compiling.
// You will need to create contract YAML configuration file and proper go.mod and
// go.sum files required for compilation. Please, refer to the NeoGo ZKP example
// to see how to verify proofs via the Verifier contract:
// https://github.com/nspcc-dev/neo-go/tree/master/examples/zkp/cubic_circuit.
//
// This contract is automatically generated.
package main

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/native/crypto"
	"github.com/nspcc-dev/neo-go/pkg/interop/util"
)

// A set of circuit-specific variables required for verification. Should be generated
// using MPC process.
var (
	// G1 Affine point.
	alpha = []byte{{ byteSliceToStr .Alpha }}
	// G2 Affine point.
	beta = []byte{{ byteSliceToStr .Beta }}
	// G2 Affine point.
	gamma = []byte{{ byteSliceToStr .Gamma }}
	// G2 Affine point.
	delta = []byte{{ byteSliceToStr .Delta }}
	// A set of G1 Affine points.
	ic = [][]byte{
		{{- range $i := .ICs }}
		{{ byteSliceToStr $i }},{{ end -}}
	}
)

// VerifyProof verifies the given proof represented as three serialized compressed
// BLS12-381 points against the public information represented as a list of
// serialized 32-bytes field elements in the LE form. Verification process
// follows the Groth-16 proving system and is taken from the
// https://github.com/neo-project/neo/issues/2647#issuecomment-1002893109 without
// any changes. Verification process checks the following equality:
//
//	A * B = alpha * beta + sum(pub_input[i] * (beta * u_i(x) + alpha * v_i(x) + w_i(x)) / gamma) * gamma + C * delta
func VerifyProof(a []byte, b []byte, c []byte, publicInput [][]byte) bool {
	alphaPoint := crypto.Bls12381Deserialize(alpha)
	betaPoint := crypto.Bls12381Deserialize(beta)
	gammaPoint := crypto.Bls12381Deserialize(gamma)
	deltaPoint := crypto.Bls12381Deserialize(delta)

	aPoint := crypto.Bls12381Deserialize(a)
	bPoint := crypto.Bls12381Deserialize(b)
	cPoint := crypto.Bls12381Deserialize(c)

	// Equation left1: A*B
	lt := crypto.Bls12381Pairing(aPoint, bPoint)

	// Equation right1: alpha*beta
	rt1 := crypto.Bls12381Pairing(alphaPoint, betaPoint)

	// Equation right2: sum(pub_input[i]*(beta*u_i(x)+alpha*v_i(x)+w_i(x))/gamma)*gamma
	inputlen := len(publicInput)
	iclen := len(ic)

	if iclen != inputlen+1 {
		panic("error: inputlen or iclen")
	}
	icPoints := make([]crypto.Bls12381Point, iclen)
	for i := 0; i < iclen; i++ {
		icPoints[i] = crypto.Bls12381Deserialize(ic[i])
	}
	acc := icPoints[0]
	for i := 0; i < inputlen; i++ {
		scalar := publicInput[i] // 32-bytes LE field element.
		temp := crypto.Bls12381Mul(icPoints[i+1], scalar, false)
		acc = crypto.Bls12381Add(acc, temp)
	}
	rt2 := crypto.Bls12381Pairing(acc, gammaPoint)

	// Equation right3: C*delta
	rt3 := crypto.Bls12381Pairing(cPoint, deltaPoint)

	// Check equality.
	t1 := crypto.Bls12381Add(rt1, rt2)
	t2 := crypto.Bls12381Add(t1, rt3)

	return util.Equals(lt, t2)
}
`

	// verifyCfg is a contract configuration file required to compile smart
	// contract.
	verifyCfg = `name: "Groth-16 Verifier contract"
sourceurl: https://github.com/nspcc-dev/neo-go/
supportedstandards: []`

	// verifyGomod is a standard go.mod file containing module name, go version
	// and dependency packages version needed for smart contract compilation.
	verifyGomod = `module verify

go 1.19

require github.com/nspcc-dev/neo-go/pkg/interop v0.0.0-20231004150345-8849ccde2524
`

	// verifyGosum is a standard go.sum file needed for contract compilation.
	verifyGosum = `github.com/nspcc-dev/neo-go/pkg/interop v0.0.0-20231004150345-8849ccde2524 h1:LKp/89ftf+MwMExKgnbwjQp5zQTUZ3lDCc+DZ4VeSRc=
github.com/nspcc-dev/neo-go/pkg/interop v0.0.0-20231004150345-8849ccde2524/go.mod h1:ZUuXOkdtHZgaC13za/zMgXfQFncZ0jLzfQTe+OsDOtg=
`
)

// VerifyProofArgs is the set of arguments of `verifyProof` method of a
// Verifier contract in serialized form (as the contract accepts them).
type VerifyProofArgs struct {
	A               []byte
	B               []byte
	C               []byte
	PublicWitnesses []any
}

// tmplParams is a set of parameters used by verification contract template.
type tmplParams struct {
	Alpha []byte
	Beta  []byte
	Gamma []byte
	Delta []byte
	ICs   [][]byte
}

// GenerateVerifier generates a Verifier smart contract written in Go for Neo
// blockchain. The contract contains a single `verifyProof` method that accepts
// a proof represented as three BLS12-381 curve points and public witnesses
// required for verification represented as a list of serialized 32-bytes field
// elements in the LE form. The boolean result of `verifyProof` is either `true`
// (if the proof is valid) or `false` (if the proof is invalid). The smart
// contract generated from this template can be immediately compiled without
// any additional changes using NeoGo compiler, deployed to the Neo chain and
// invoked. The verification contract is circuit-specific, i.e. corresponds to
// a specific constraint system. Thus, every new circuit requires its own
// verification contract to be generated and deployed to the chain.
//
// GenerateVerifier also generates a proper contract YAML configuration file,
// go.mod and go.sum files if the corresponding writers are provided via cfg.
func GenerateVerifier(cfg Config) error {
	if cfg.VerifyingKey == nil {
		return fmt.Errorf("nil verifying key")
	}
	if cfg.VerifyingKey.CurveID() != ecc.BLS12_381 {
		return fmt.Errorf("unexpected elliptic curve: %s", cfg.VerifyingKey.CurveID())
	}

	// Fetch the contract's public verification parameters. We can directly access
	// the VerifyingKey elements since gnark v0.9.0.
	vk := cfg.VerifyingKey.(*curve.VerifyingKey)
	alphaG1 := vk.G1.Alpha.Bytes()
	betaG2 := vk.G2.Beta.Bytes()
	gammaG2 := vk.G2.Gamma.Bytes()
	deltaG2 := vk.G2.Delta.Bytes()
	kvks := make([][]byte, len(vk.G1.K))
	for i := range kvks {
		arr := vk.G1.K[i].Bytes()
		kvks[i] = arr[:]
	}

	// Generate verification contract from the template using the retrieved
	// verification parameters.
	tmpl := template.Must(template.New("generate").Funcs(template.FuncMap{
		"byteSliceToStr": byteSliceToStr,
	}).Parse(goVerificationTmpl))

	err := binding.FExecute(tmpl, cfg.Output, tmplParams{
		Alpha: alphaG1[:],
		Beta:  betaG2[:],
		Gamma: gammaG2[:],
		Delta: deltaG2[:],
		ICs:   kvks,
	})
	if err != nil {
		return err
	}

	if cfg.CfgOutput != nil {
		_, err = cfg.CfgOutput.Write([]byte(verifyCfg))
		if err != nil {
			return fmt.Errorf("failed to generate contract configuration file: %w", err)
		}
	}
	if cfg.GomodOutput != nil {
		_, err = cfg.GomodOutput.Write([]byte(verifyGomod))
		if err != nil {
			return fmt.Errorf("failed to generate go.mod file: %w", err)
		}
	}
	if cfg.GosumOutput != nil {
		_, err = cfg.GosumOutput.Write([]byte(verifyGosum))
		if err != nil {
			return fmt.Errorf("failed to generate go.mod file: %w", err)
		}
	}

	return nil
}

// byteSliceToStr is a codegen helper that converts byte slice to a go-like slice.
func byteSliceToStr(s []byte) string {
	var res string
	for _, b := range s {
		res += fmt.Sprintf("%d, ", b)
	}
	return `{` + res[:len(res)-2] + `}`
}

// GetVerifyProofArgs returns a serialized set of arguments `verifyProof` method
// of a generated Verifier contract accepts. The set of arguments may be directly
// used as parameters to contract invocation.
func GetVerifyProofArgs(proof groth16.Proof, publicWitness witness.Witness) (*VerifyProofArgs, error) {
	if proof == nil {
		return nil, errors.New("nil proof")
	}
	if proof.CurveID() != ecc.BLS12_381 {
		return nil, fmt.Errorf("unexpected elliptic curve: %s", proof.CurveID())
	}
	// If a full witness was provided, then retrieve public part, we don't need the secret part of it.
	publicWitness, err := publicWitness.Public()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve public witness: %w", err)
	}
	// Get the proof bytes (points are in the compressed form, as Verification contract accepts it).
	p := proof.(*curve.Proof)
	aBytes := p.Ar.Bytes()
	bBytes := p.Bs.Bytes()
	cBytes := p.Krs.Bytes()

	publicWitnessBytes, err := publicWitness.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to encode public witness: %w", err)
	}
	numPublicWitness := binary.BigEndian.Uint32(publicWitnessBytes[:4])
	numSecretWitness := binary.BigEndian.Uint32(publicWitnessBytes[4:8])
	numVectorElements := binary.BigEndian.Uint32(publicWitnessBytes[8:12])

	// Ensure that serialization format is as expected (just in case).
	if numSecretWitness != 0 {
		return nil, fmt.Errorf("unexpected number of secret witnesses: %d", numSecretWitness)
	}
	if numPublicWitness+numSecretWitness != numVectorElements {
		return nil, fmt.Errorf("unexpected number of public witness elements: %d", numVectorElements)
	}

	// Create public witness input.
	input := make([]any, numVectorElements)
	offset := 12
	for i := range input { // firstly - public witnesses, after that - private ones (but they are missing from publicWitness anyway).
		start := offset + i*fr.Bytes
		end := start + fr.Bytes
		slice.Reverse(publicWitnessBytes[start:end]) // gnark stores witnesses in the BE form, but native CryptoLib accepts LE-encoded fields elements (not a canonical form).
		input[i] = publicWitnessBytes[start:end]
	}
	return &VerifyProofArgs{
		A:               aBytes[:],
		B:               bBytes[:],
		C:               cBytes[:],
		PublicWitnesses: input,
	}, nil
}
