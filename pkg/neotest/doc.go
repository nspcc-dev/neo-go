/*
Package neotest contains a framework for automated contract testing.
It can be used to implement unit-tests for contracts in Go using regular Go
conventions.

Usually it's used like this:
 * an instance of the blockchain is created using chain subpackage
 * the target contract is compiled using one of Compile* functions
 * and Executor is created for the blockchain
 * it's used to deploy a contract with DeployContract
 * CommitteeInvoker and/or ValidatorInvoker are then created to perform test invocations
 * if needed, NewAccount is used to create an appropriate number of accounts for the test

Higher-order methods provided in Executor and ContractInvoker hide the details
of transaction creation for the most part, but there are lower-level methods as
well that can be used for specific tasks.
*/
package neotest
