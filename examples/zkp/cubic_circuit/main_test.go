package cubic

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	curve "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/backend/groth16/bls12-381/mpcsetup"
	"github.com/consensys/gnark/constraint"
	cs "github.com/consensys/gnark/constraint/bls12-381"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/test"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/zkpbinding"
	"github.com/stretchr/testify/require"
)

// First of all, you'll need to ensure that your circuit is properly constructed.
// Use unit tests to test execute the circuit and verify it against a various set
// of curves and backends with gnark/test package.
// More about circuit testing using gnark/test package: https://pkg.go.dev/github.com/consensys/gnark/test@v0.7.0

// TestCubicCircuit_TestExecution runs the provided circuit in the test execution engine.
func TestCubicCircuit_TestExecution(t *testing.T) {
	var (
		circuit    CubicCircuit
		assignment = CubicCircuit{X: 3, Y: 35}
	)

	// Test executing the circuit without running a ZK-SNARK prover (with the
	// help of test engine). It can be useful for the circuit debugging, see
	// https://docs.gnark.consensys.net/HowTo/debug_test#common-errors.
	err := test.IsSolved(&circuit, &assignment, ecc.BLS12_381.ScalarField())
	require.NoError(t, err)
}

// TestCubicCircuit_Verification performs the circuit correctness testing over a
// set of all supported curves and backends and over a specified curve with a
// set of exact input and output values.
func TestCubicCircuit_Verification(t *testing.T) {
	// Assert object wrapping testing.T.
	assert := test.NewAssert(t)

	// Declare the circuit.
	var cubicCircuit CubicCircuit

	// The default behavior of the assert helper is to test the circuit across
	// all supported curves and backends, ensure correct serialization, and
	// cross-test the constraint system solver against a big.Int test execution
	// engine.
	assert.ProverFailed(&cubicCircuit, &CubicCircuit{
		X: 3, // Wrong value.
		Y: 5,
	})

	// If needed, we can directly specify the desired curves or backends.
	assert.ProverSucceeded(&cubicCircuit, &CubicCircuit{
		X: 3, // Good value.
		Y: 35,
	}, test.WithCurves(ecc.BLS12_381))
}

// TestCubicCircuit_EndToEnd shows how to generate proof for pre-defined cubic circuit,
// how to generate Go verification contract that can be compiled by NeoGo and deployed
// to the chain and how to verify proofs via verification contract invocation.
func TestCubicCircuit_EndToEnd(t *testing.T) {
	var (
		circuit    CubicCircuit
		assignment = CubicCircuit{X: 3, Y: 35}
	)

	// Compile our circuit into a R1CS (a constraint system).
	ccs, err := frontend.Compile(ecc.BLS12_381.ScalarField(), r1cs.NewBuilder, &circuit)
	require.NoError(t, err)

	// One time setup (groth16 zkSNARK). Built-in groth16.Setup function is used
	// for the test purposes. In production environment it is recommended to use
	// MPC-based solution for proving and verifying keys generation, see the
	// TestCubicCircuit_EndToEnd_Prod test.
	pk, vk, err := groth16.Setup(ccs)
	require.NoError(t, err)

	// Intermediate step: witness definition.
	witness, err := frontend.NewWitness(&assignment, ecc.BLS12_381.ScalarField())
	require.NoError(t, err)
	publicWitness, err := witness.Public()
	require.NoError(t, err)

	// Proof creation (groth16).
	proof, err := groth16.Prove(ccs, pk, witness)
	require.NoError(t, err)

	// Ensure that gnark can successfully verify the proof (just in case).
	err = groth16.Verify(proof, vk, publicWitness)
	require.NoError(t, err)

	// Now, when we're sure that the proof is valid, we can create and deploy verification
	// contract to the Neo testing chain.
	args, err := zkpbinding.GetVerifyProofArgs(proof, publicWitness)
	require.NoError(t, err)

	// Create contract file.
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "verify.go")
	f, err := os.Create(srcPath)
	require.NoError(t, err)

	// Create contract configuration file.
	cfgPath := filepath.Join(tmpDir, "verify.yml")
	fCfg, err := os.Create(cfgPath)
	require.NoError(t, err)

	// Create contract go.mod and go.sum files.
	fMod, err := os.Create(filepath.Join(tmpDir, "go.mod"))
	require.NoError(t, err)
	fSum, err := os.Create(filepath.Join(tmpDir, "go.sum"))
	require.NoError(t, err)

	err = zkpbinding.GenerateVerifier(zkpbinding.Config{
		VerifyingKey: vk,
		Output:       f,
		CfgOutput:    fCfg,
		GomodOutput:  fMod,
		GosumOutput:  fSum,
	})
	require.NoError(t, err)

	require.NoError(t, f.Close())
	require.NoError(t, fCfg.Close())
	require.NoError(t, fMod.Close())
	require.NoError(t, fSum.Close())

	// Create testing chain and deploy contract onto it.
	bc, committee := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, committee, committee)

	// Compile verification contract and deploy the contract onto chain.
	c := neotest.CompileFile(t, e.Validator.ScriptHash(), srcPath, cfgPath)
	e.DeployContract(t, c, nil)

	// Verify proof via verification contract call.
	validatorInvoker := e.ValidatorInvoker(c.Hash)
	validatorInvoker.Invoke(t, true, "verifyProof", args.A, args.B, args.C, args.PublicWitnesses)
}

// TestCubicCircuit_EndToEnd shows how to generate proof for pre-defined cubic circuit,
// how to generate Go verification contract that can be compiled by NeoGo and deployed
// to the chain and how to verify proofs via verification contract invocation. It
// differs from TestCubicCircuit_EndToEnd in that it uses pre-generated Powers of Tau
// result for proving/verifying keys generation and demonstrates how to contribute
// some randomness into it.
func TestCubicCircuit_EndToEnd_Prod(t *testing.T) {
	const (
		// Response file generated locally for 2^8 powers.
		pathToResponseFile = "./response8"
		// The order of Powers of Tau ceremony, it depends on the response file.
		orderOfResponseFile = 8
	)
	var (
		circuit    CubicCircuit
		assignment = CubicCircuit{X: 3, Y: 35}
	)

	// Compile our circuit into a R1CS (a constraint system).
	ccs, err := frontend.Compile(ecc.BLS12_381.ScalarField(), r1cs.NewBuilder, &circuit)
	require.NoError(t, err)

	// Setup (groth16 zkSNARK), use MPC-based solution for proving and verifying
	// keys generation. Please, be careful while adopting this code for your circuit.
	// Ensure that response file that you've provided contains enough powers computed
	// so that the number of constraints in your circuit can be handled.
	pk, vk := setup(t, ccs, pathToResponseFile, orderOfResponseFile)

	// Intermediate step: witness definition.
	witness, err := frontend.NewWitness(&assignment, ecc.BLS12_381.ScalarField())
	require.NoError(t, err)
	publicWitness, err := witness.Public()
	require.NoError(t, err)

	// Proof creation (groth16).
	proof, err := groth16.Prove(ccs, pk, witness)
	require.NoError(t, err)

	// Ensure that gnark can successfully verify the proof (just in case).
	err = groth16.Verify(proof, vk, publicWitness)
	require.NoError(t, err)

	// Now, when we're sure that the proof is valid, we can create and deploy verification
	// contract to the Neo testing chain.
	args, err := zkpbinding.GetVerifyProofArgs(proof, publicWitness)
	require.NoError(t, err)

	// Create contract file.
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "verify.go")
	f, err := os.Create(srcPath)
	require.NoError(t, err)

	// Create contract configuration file.
	cfgPath := filepath.Join(tmpDir, "verify.yml")
	fCfg, err := os.Create(cfgPath)
	require.NoError(t, err)

	// Create contract go.mod and go.sum files.
	fMod, err := os.Create(filepath.Join(tmpDir, "go.mod"))
	require.NoError(t, err)
	fSum, err := os.Create(filepath.Join(tmpDir, "go.sum"))
	require.NoError(t, err)

	err = zkpbinding.GenerateVerifier(zkpbinding.Config{
		VerifyingKey: vk,
		Output:       f,
		CfgOutput:    fCfg,
		GomodOutput:  fMod,
		GosumOutput:  fSum,
	})
	require.NoError(t, err)

	require.NoError(t, f.Close())
	require.NoError(t, fCfg.Close())
	require.NoError(t, fMod.Close())
	require.NoError(t, fSum.Close())

	// Create testing chain and deploy contract onto it.
	bc, committee := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, committee, committee)

	// Compile verification contract and deploy the contract onto chain.
	c := neotest.CompileFile(t, e.Validator.ScriptHash(), srcPath, cfgPath)
	e.DeployContract(t, c, nil)

	// Verify proof via verification contract call.
	validatorInvoker := e.ValidatorInvoker(c.Hash)
	validatorInvoker.Invoke(t, true, "verifyProof", args.A, args.B, args.C, args.PublicWitnesses)
}

// setup generates proving and verifying keys for the given compiled constrained
// system. It accepts path to the response file from Phase 1 of the Powers of Tau
// ceremony for the BLS12-381 curve and the power of the ceremony.
// See the README.md for details on the Phase 1 response file. It makes
// circuit-specific Phase 2 initialisation of the MPC ceremony and performs some
// dummy contributions for Phase 2. In production environment, participant will
// receive a []byte, deserialize it, add his contribution and send back to the
// coordinator.
func setup(t *testing.T, ccs constraint.ConstraintSystem, phase1ResponsePath string, inPow int) (groth16.ProvingKey, groth16.VerifyingKey) {
	const (
		nContributionsPhase2 = 3
		blake2bHashSize      = 64
	)

	f, err := os.Open(phase1ResponsePath)
	require.NoError(t, err)

	// Skip hash of the previous contribution, don't need it for the MPC initialisation.
	_, err = f.Seek(blake2bHashSize, 0)
	require.NoError(t, err)
	dec := curve.NewDecoder(f)

	// Retrieve parameters.
	inN := int(math.Pow(2, float64(inPow)))
	coef_g1 := make([]curve.G1Affine, 2*inN-1)
	coef_g2 := make([]curve.G2Affine, inN)
	alpha_coef_g1 := make([]curve.G1Affine, inN)
	beta_coef_g1 := make([]curve.G1Affine, inN)

	// Accumulator serialization: https://github.com/filecoin-project/powersoftau/blob/ab8f85c28f04af5a99cfcc93a3b1f74c06f94105/src/accumulator.rs#L111
	errMessage := fmt.Sprintf("ensure your response file contains exactly 2^%d powers of tau for BLS12-381 curve", inPow)
	for i := range coef_g1 {
		require.NoError(t, dec.Decode(&coef_g1[i]), errMessage)
	}
	for i := range coef_g2 {
		require.NoError(t, dec.Decode(&coef_g2[i]), errMessage)
	}
	for i := range alpha_coef_g1 {
		require.NoError(t, dec.Decode(&alpha_coef_g1[i]), errMessage)
	}
	for i := range beta_coef_g1 {
		require.NoError(t, dec.Decode(&beta_coef_g1[i]), errMessage)
	}
	beta_g2 := &curve.G2Affine{}
	require.NoError(t, dec.Decode(beta_g2), errMessage)

	// Transform (take exactly those number of powers that needed for the given number of constraints).
	var (
		numConstraints = ccs.GetNbConstraints()
		outPow         int
	)
	for ; 1<<outPow < numConstraints; outPow++ {
	}
	outN := int64(math.Pow(2, float64(outPow)))

	if len(coef_g1) < int(2*outN-1) {
		t.Fatalf("number of circuit constraints is too large for the provided response file: nbConstraints is %d, required at least %d powers to be computed", numConstraints, outN)
	}
	srs1 := mpcsetup.Phase1{}
	srs1.Parameters.G1.Tau = coef_g1[:2*outN-1]        // outN + (outN-1)
	srs1.Parameters.G2.Tau = coef_g2[:outN]            // outN
	srs1.Parameters.G1.AlphaTau = alpha_coef_g1[:outN] // outN
	srs1.Parameters.G1.BetaTau = beta_coef_g1[:outN]   // outN
	srs1.Parameters.G2.Beta = *beta_g2                 // 1

	// Prepare for phase-2
	var evals mpcsetup.Phase2Evaluations
	r1cs := ccs.(*cs.R1CS)
	srs2, evals := mpcsetup.InitPhase2(r1cs, &srs1)

	// Make some dummy contributions for phase2. In practice, participant will
	// receive a []byte, deserialize it, add his contribution and send back to
	// coordinator, like it is done in https://github.com/bnb-chain/zkbnb-setup
	// for BN254 elliptic curve.
	for i := 0; i < nContributionsPhase2; i++ {
		srs2.Contribute()
	}

	// Extract the proving and verifying keys
	pk, vk := mpcsetup.ExtractKeys(&srs1, &srs2, &evals, ccs.GetNbConstraints())
	return &pk, &vk
}
