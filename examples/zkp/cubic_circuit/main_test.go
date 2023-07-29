package cubic

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
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

	// One time setup (groth16 zkSNARK).
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
