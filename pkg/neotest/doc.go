/*
Package neotest contains a framework for automated contract testing.
It can be used to implement unit-tests for contracts in Go using regular Go
conventions.

Usually it's used like this:

  - an instance of the blockchain is created using chain subpackage
  - the target contract is compiled using one of Compile* functions
  - and Executor is created for the blockchain
  - it's used to deploy a contract with DeployContract
  - CommitteeInvoker and/or ValidatorInvoker are then created to perform test invocations
  - if needed, NewAccount is used to create an appropriate number of accounts for the test

Higher-order methods provided in Executor and ContractInvoker hide the details
of transaction creation for the most part, but there are lower-level methods as
well that can be used for specific tasks.

It's recommended to have a separate folder/package for tests, because having
them in the same package with the smart contract iself can lead to unxpected
results if smart contract has any init() functions. If that's the case they
will be compiled into the testing binary even when using package_test and their
execution can affect tests. See https://github.com/nspcc-dev/neo-go/issues/3120 for details.
*/
package neotest
