// Package xor contains an example of smart contract that uses BLS12-381
// curves interoperability functionality to verify provided proof against provided
// public input. This example is a full copy of C# smart contract presented in
// https://github.com/neo-project/neo/issues/2647#issuecomment-1129849870 and is
// aimed to check the compatibility of BLS12-381 interoperability functionality
// between the NeoC# and NeoGo nodes. This example is circuit-specific and can
// not be deployed into chain to verify other circuits.
//
// This example is constructed to prove that a&b=0 where a and b are private input.
//
// Please, do not use this example to create, proof and verify your own circuits.
// Refer to examples/cubic_circuit package to get an example of a custom circuit
// creation in Go and check out the
// [zkpbinding](https://pkg.go.dev/github.com/nspcc-dev/neo-go/pkg/smartcontract/zkpbinding)
// package to create your own verification contract for the circuit.
package xor

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/native/crypto"
	"github.com/nspcc-dev/neo-go/pkg/interop/util"
)

// Constants needed for verification should be obtained via MPC process.
var (
	// G1 Affine.
	alpha = []byte{160, 106, 167, 155, 164, 170, 67, 158, 237, 78, 91, 7, 243, 191, 186, 221, 27, 97, 6, 190, 193, 204, 85, 206, 83, 56, 3, 209, 132, 249, 221, 94, 124, 20, 245, 113, 143, 70, 245, 159, 104, 213, 37, 151, 209, 125, 160, 143}
	// G2 Affine.
	beta = []byte{163, 91, 30, 20, 61, 202, 142, 33, 164, 33, 215, 106, 219, 39, 136, 96, 112, 254, 117, 55, 156, 44, 55, 125, 240, 63, 166, 206, 157, 17, 201, 11, 33, 172, 226, 58, 254, 202, 46, 128, 2, 179, 227, 37, 230, 127, 121, 118, 6, 59, 84, 145, 104, 196, 68, 37, 209, 54, 86, 148, 155, 251, 36, 110, 127, 190, 205, 52, 100, 136, 226, 196, 249, 172, 122, 215, 230, 42, 92, 175, 190, 120, 19, 80, 56, 148, 236, 157, 108, 74, 45, 29, 157, 243, 96, 94}
	// G2 Affine.
	gamma = []byte{183, 69, 47, 108, 115, 173, 254, 203, 89, 67, 183, 224, 176, 26, 127, 132, 89, 162, 99, 241, 66, 228, 177, 17, 57, 85, 3, 13, 148, 88, 162, 54, 220, 189, 33, 172, 38, 192, 116, 236, 13, 115, 219, 201, 51, 166, 253, 240, 12, 32, 77, 82, 161, 189, 240, 198, 148, 184, 17, 92, 162, 145, 166, 55, 252, 245, 194, 95, 71, 208, 215, 23, 19, 95, 138, 147, 149, 26, 35, 108, 141, 25, 139, 103, 59, 48, 189, 88, 204, 100, 255, 116, 194, 229, 157, 5}
	// G2 Affine.
	delta = []byte{129, 78, 83, 175, 159, 103, 127, 217, 80, 213, 0, 194, 108, 30, 210, 241, 138, 209, 0, 164, 117, 32, 68, 102, 121, 36, 40, 65, 89, 205, 198, 1, 14, 144, 196, 236, 176, 214, 119, 139, 225, 118, 215, 185, 36, 216, 183, 27, 22, 126, 193, 21, 173, 212, 250, 104, 25, 69, 107, 40, 199, 160, 228, 239, 112, 102, 144, 85, 58, 109, 122, 73, 221, 170, 145, 188, 60, 9, 228, 178, 36, 227, 175, 140, 40, 181, 158, 175, 91, 189, 92, 169, 90, 90, 30, 153}
	// A set of G1 Affine points.
	ic = [][]byte{
		{174, 152, 253, 159, 101, 142, 227, 5, 166, 71, 152, 207, 32, 152, 56, 172, 191, 43, 184, 28, 148, 40, 224, 42, 135, 137, 181, 215, 96, 34, 200, 127, 77, 151, 165, 11, 130, 57, 91, 83, 71, 38, 253, 159, 103, 191, 139, 120},
		{177, 158, 199, 19, 137, 211, 161, 248, 118, 149, 250, 145, 46, 221, 160, 86, 40, 165, 110, 198, 160, 203, 188, 84, 210, 83, 159, 176, 113, 111, 10, 235, 192, 243, 242, 110, 188, 210, 98, 199, 74, 66, 118, 251, 3, 188, 58, 84},
	}
)

// VerifyProof verifies the given proof represented as three BLS12-381 points
// against the public information represented as a list of serialized 32-bytes
// field elements in the LE form. The verification process follows the GROTH-16
// proving system and is taken from the
// https://github.com/neo-project/neo/issues/2647#issuecomment-1002893109 without
// changes. The verification process checks the following equality:
// A * B = alpha * beta + sum(pub_input[i] * (beta * u_i(x) + alpha * v_i(x) + w_i(x)) / gamma) * gamma + C * delta
func VerifyProof(a []byte, b []byte, c []byte, publicInput [][]byte) bool {
	alphaPoint := crypto.Bls12381Deserialize(alpha)
	betaPoint := crypto.Bls12381Deserialize(beta)
	gammaInversePoint := crypto.Bls12381Deserialize(gamma)
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
	rt2 := crypto.Bls12381Pairing(acc, gammaInversePoint)

	// Equation right3: C*delta
	rt3 := crypto.Bls12381Pairing(cPoint, deltaPoint)

	// Check equality.
	t1 := crypto.Bls12381Add(rt1, rt2)
	t2 := crypto.Bls12381Add(t1, rt3)

	return util.Equals(lt, t2)
}
