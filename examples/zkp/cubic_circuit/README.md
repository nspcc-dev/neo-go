 ### Example description
 
 This example demonstrates how to create your own circuit and generate Groth-16
 proof based on BLS12-381 elliptic curve points with the help of
 [consensys/gnark](https://pkg.go.dev/github.com/consensys/gnark). It also shows how to generate, deploy and invoke Verifier
 smart contract to verify proofs for the given circuit on the Neo chain with the
 help of [zkpbindings](https://pkg.go.dev/github.com/nspcc-dev/neo-go/pkg/smartcontract/zkpbinding) NeoGo package. The package also contains circuit
 tests implemented with [gnark/test](https://pkg.go.dev/github.com/consensys/gnark/test) to check the circuit validity and
 end-to-end proof generation/verification test implemented with [neotest](https://pkg.go.dev/github.com/nspcc-dev/neo-go/pkg/neotest)
 to demonstrate how to build, deploy and verify proofs via Verifier smart
 contract for the given circuit.
 
### Groth-16 setup notes

Common reference string (CRS) is needed to generate proving and verifying keys
for the given constrained system. In production environment, CRS generation can
be performed via Multi-Party Computation (MPC) ceremony that includes two
phases: Phase 1 (a.k.a. Powers of Tau) that is curve-specific and those
results may be used by all circuits; and Phase 2 that is circuit-specific and
uses the result of Phase 1 as an input.

For testing setups, check out the [`TestCubicCircuit_EndToEnd`](./main_test.go)
keys generation stage. For production usage, read the information below.

Both phases for BLS12-381 curve can be implemented in the Go programming language
using the corresponding `consensys/gnark` API (see the
[test example](https://github.com/Consensys/gnark/blob/36b0b58f02d0381774b24efba0a48032e5f794b4/backend/groth16/bls12-381/mpcsetup/setup_test.go#L34))
and the example of a
[CLI tool that uses the API with BN254 elliptic curve](https://github.com/bnb-chain/zkbnb-setup)
to organize the ceremony and generate proving and verifying keys for a circuit.
However, both phases take a significant amount of time and computations to be
performed. Luckily for the developers, it is possible to omit a curve-specific
part of the MPC and reuse the existing results of Phase 1 got from a trusted
source, e.g. from [Zcash PowersOfTau](https://github.com/ZcashFoundation/powersoftau-attestations)
held by the [Zcash Foundation](https://github.com/ZcashFoundation).
`TestCubicCircuit_EndToEnd_Prod` test of the current circuit example demonstrates
how to use the `response` output file from the Phase 1 of the Filecoin's Powers
of Tau ceremony for BLS12-381 curve:
* [`response8`](./response8) file is the response output from the ceremony that was run locally
  based on the [Filecoin Powers of Tau](https://github.com/filecoin-project/powersoftau/)
  with the `REQUIRED_POWER` set to 8 (to reduce computations and response file size).
  The ceremony itself was run with the help of [testing script](https://github.com/filecoin-project/powersoftau/blob/master/test.sh).
  To get the response file for a production environment, the user has two options:
  1. Organize his own ceremony with required number of powers following the
     [guide](https://github.com/filecoin-project/powersoftau/tree/master#instructions)
     from the ceremony source repo.
  2. Download the existing suitable `response` file from the trusted existing ceremony.
     Please, be careful while choosing `response` file and ensure that it has enough
     powers computed (at least as much as the number of the circuit's constraints requires).
     Example of suitable ceremonies:
     * Zcash Powers Of Tau [attestations page](https://github.com/ZcashFoundation/powersoftau-attestations) (up to 2^21)
     * Filecoin Perpetual Powers Of Tau [attestations page](https://github.com/arielgabizon/perpetualpowersoftau#perpetual-powers-of-tau-for-bls381) (up to 2^27)
* [main_test](./main_test.go) contains the `TestCubicCircuit_EndToEnd_Prod` test
  itself and demonstrates how to properly initialize Phase 2 based on the given
  response file and make some dummy contributions into it.

Take the [`TestCubicCircuit_EndToEnd_Prod`](./main_test.go) test logic as a basis
while generating the circuit-specific proving and verifying keys for the production
usage. Currently, we don't have a BLS12-381 specific Groth-16 setup CLI utility
like for [BN254 curve](https://github.com/bnb-chain/zkbnb-setup), but eventually
it will be included into the NeoGo toolkit to make the development process easier.
Don't hesitate to contact us if you're looking for a similar CLI utility for
BLS12-381 curve.