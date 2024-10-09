// Package cubic describes how to create and verify proofs on the Neo
// blockchain. The example shows how to check that the prover knows the solution
// of the cubic equation: y = x^3 + x + 5. The example is constructed for
// BLS12-381 curve points using Groth-16 proving system. The example includes
// everything that developer needs to start using ZKP on the Neo platform with
// Go SDK:
//  1. The described cubic circuit implementation.
//  2. The off-chain proof generation with the help of gnark-crypto library.
//  3. The Go verification contract generation and deployment with the help of
//     NeoGo library.
//  4. The on-chain proof verification for various sets of input data (implemented
//     as end-to-end test).
//  5. A set of unit-tests aimed to check the circuit validity.
package cubic

import (
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// CubicCircuit defines a simple circuit x**3 + x + 5 == y
// that checks that the prover knows the solution for the provided expression.
// The circuit must declare its public and secret inputs as frontend.Variable.
// At compile time, frontend.Compile(...) recursively parses the struct fields
// that contains frontend.Variable to build the frontend.constraintSystem.
// By default, a frontend.Variable has the gnark:",secret" visibility.
type CubicCircuit struct {
	// Struct tags on a variable is optional.
	// Default uses variable name and secret visibility.
	X frontend.Variable `gnark:"x,secret"` // Secret input.
	Y frontend.Variable `gnark:"y,public"` // Public input.
}

// A gnark circuit must implement the frontend.Circuit interface
// (https://docs.gnark.consensys.net/HowTo/write/circuit_structure).
var _ = frontend.Circuit(&CubicCircuit{})

// Define declares the circuit constraints
// x**3 + x + 5 == y.
func (circuit *CubicCircuit) Define(api frontend.API) error {
	x3 := api.Mul(circuit.X, circuit.X, circuit.X)

	// Can be used for the circuit debugging.
	api.Println("X^3", x3)

	api.AssertIsEqual(circuit.Y, api.Add(x3, circuit.X, 5))
	return nil
}

// main demonstrates how to build the proof and verify it with the help of gnark
// library. Error handling omitted intentionally to simplify the example.
func main() { // nolint: unused
	var (
		circuit    CubicCircuit
		assignment = CubicCircuit{X: 3, Y: 35}
	)

	// Compile our circuit into a R1CS (a constraint system).
	ccs, _ := frontend.Compile(ecc.BLS12_381.ScalarField(), r1cs.NewBuilder, &circuit)

	// Once the circuit is compiled, you can run the three algorithms of a zk-SNARK back end:

	// 1. One time setup (groth16 zkSNARK).
	pk, vk, _ := groth16.Setup(ccs)

	// Intermediate step: witness definition.
	witness, _ := frontend.NewWitness(&assignment, ecc.BLS12_381.ScalarField())
	publicWitness, _ := witness.Public()

	// 2. Proof creation (groth16).
	proof, _ := groth16.Prove(ccs, pk, witness)

	// 3. Proof verification (groth16) via gnark-crypto library.
	_ = groth16.Verify(proof, vk, publicWitness)

	// 4. If building ZKP systems for Neo, you'll need a verification contract
	// deployed to the Neo chain to be able to verify generated proofs. This
	// contract can be generated automatically using NeoGo zkpbinding package:
	// err := zkpbinding.GenerateVerifier(zkpbinding.Config{
	//  VerifyingKey: vk,
	// 	Output:       f,    // Verifier Go contract writer
	// 	CfgOutput:    fCfg, // Verifier Go contract configuration YAML file
	// 	GomodOutput:  fMod, // go.mod file for the Verifier contract
	// 	GosumOutput:  fSum, // go.sum file for the Verifier contract
	// })
	//
	// Create arguments to invoke `verifyProof` mathod of Verifier contract:
	// verifyProofArgs, err := zkpbinding.GetVerifyProofArgs(proof, publicWitness)
	//
	// For end-to-end usage example, please, see the TestCubicCircuit_EndToEnd test.
}
