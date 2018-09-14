# Runtime
A brief overview of NEO smart contract API's that can be used in the neo-storm framework.

# Overview
1. [Account]()
2. [Asset]()
3. [Attribute]()
4. [Block]()
5. [Blockchain]()
6. [Contract]()
7. [Crypto]()
8. [Engine]()
9. [Enumerator]()
10. [Iterator]()
11. [Header]()
12. [Input]()
13. [Output]()
14. [Runtime]()
15. [Storage]()
16. [Transaction]()
17. [Util]()

## Account 
#### GetScriptHash
```
GetScriptHash(a Account) []byte
```
Returns the script hash of the given account.

#### GetVotes
```
GetVotes(a Account) [][]byte
```
Returns the the votes (a slice of public keys) of the given account.

#### GetBalance
```
GetBalance(a Account, assetID []byte) int
```
Returns the balance of the given asset id for the given account.

## Asset
#### GetAssetID 
```
GetAssetID(a Asset) []byte
```
Returns the id of the given asset.

#### GetAmount
```
GetAmount(a Asset) int 
```
Returns the amount of the given asset id.

#### GetAvailable
```
GetAvailable(a Asset) int 
```
Returns the available amount of the given asset.

#### GetPrecision
```
GetPrecision(a Asset) byte
```
Returns the precision of the given Asset.

#### GetOwner
```
GetOwner(a Asset) []byte
```
Returns the owner of the given asset.

#### GetAdmin
```
GetAdmin(a Asset) []byte
```
Returns the admin of the given asset.

#### GetIssuer
```
GetIssuer(a Asset) []byte
```
Returns the issuer of the given asset.

#### Create
```
Create(type byte, name string, amount int, precision byte, owner, admin, issuer []byte)
```
Creates a new asset on the blockchain.

#### Renew
```
Renew(asset Asset, years int)
```
Renews the given asset as long as the given years.

## Attribute
#### GetUsage
```
GetUsage(attr Attribute) []byte
```
Returns the usage of the given attribute.

#### GetData
```
GetData(attr Attribute) []byte
```
Returns the data of the given attribute.

## Block
#### GetTransactionCount
```
GetTransactionCount(b Block) int
```
Returns the number of transactions that are recorded in the given block.

#### GetTransactions
```
GetTransactions(b Block) []transaction.Transaction
```
Returns a slice of the transactions that are recorded in the given block.

#### GetTransaction
```
GetTransaction(b Block, hash []byte) transaction.Transaction
```
Returns the transaction by the given hash that is recorded in the given block.

## Blockchain
#### GetHeight
```
GetHeight() int
```
Returns the current height of the blockchain.

#### GetHeader
```
GetHeader(heightOrHash []interface{}) header.Header
```
Return the header by the given hash or index.

#### GetBlock
```
GetBlock(heightOrHash interface{}) block.Block
```
Returns the block by the given hash or index.

#### GetTransaction
```
GetTransaction(hash []byte) transaction.Transaction
```
Returns a transaction by the given hash.

#### GetContract
```
GetContract(scriptHash []byte) contract.Contract
```
Returns the contract found by the given script hash.

#### GetAccount
```
GetAccount(scriptHash []byte) account.Account
```
Returns the account found by the given script hash.

#### GetValiditors
```
GetValidators() [][]byte
```
Returns a list of validators public keys.

#### GetAsset
```
GetAsset(assetID []byte) asset.Asset
```
Returns the asset found by the given asset id.

## Contract
#### GetScript
```
GetScript(c Contract) []byte
```
Return the script of the given contract.

#### IsPayable
```
IsPayable(c Contract) bool
```
Returns whether the given contract is payable.

#### GetStorageContext 
```
GetStorageContext(c Contract)
```
Returns the storage context of the given contract.

#### Create
```
Create(
    script []byte, 
    params []interface{}, 
    returnType byte, 
    properties interface{}, 
    name, 
    version, 
    author, 
    email, 
    description string)
```
Creates a new contract on the blockchain.

#### Migrate
```
Migrate(
    script []byte, 
    params []interface{}, 
    returnType byte, 
    properties interface{}, 
    name, 
    version, 
    author, 
    email, 
    description string)
```
Migrates a contract on the blockchain.

#### Destroy
```
Destroy(c Contract) 
```
Deletes the given contract from the blockchain.

## Crypto
#### SHA1
```
SHA1(data []byte) []byte
```
Computes the sha1 hash of the given bytes

#### SHA256
```
SHA256(data []byte) []byte
```
Computes the sha256 hash of the given bytes

#### Hash256
```
Hash256(data []byte) []byte
```
Computes the sha256^2 of the given data.

#### Hash160
```
Hash160(data []byte) []byte) []byte
```
Computes the ripemd160 over the sha256 hash of the given data.

## Engine
#### GetScriptContainer
```
GetScriptContainer() transaction.Transaction
```
Returns the transaction that is in the context of the VM execution.

#### GetExecutingScriptHash
```
GetExecutingScriptHash() []byte
```
Returns the script hash of the contract that is currently being executed.

#### GetCallingScriptHash
```
GetCallingScriptHash() []byte
```
Returns the script hash of the contract that has started the execution of the current script.

#### GetEntryScriptHash
```
GetEntryScriptHash() []byte
```
Returns the script hash of the contract that started the execution from the start. 

## Enumerator
#### Create
```
Create(items []inteface{}) Enumerator
```
Create a enumerator from the given items.

#### Next
```
Next(e Enumerator) interface{}
```
Returns the next item from the given enumerator.

#### Value
```
Value(e Enumerator) interface{}
```
Returns the enumerator value.

## Iterator
#### Create
```
Create(items []inteface{}) Iterator
```
Creates an iterator from the given items.

#### Key
```
Key(it Iterator) interface{}
```
Return the key from the given iterator.

#### Keys
```
Keys(it Iterator) []interface{}
```
Returns the iterator's keys 

#### Values
```
Values(it Iterator) []interface{}
```
Returns the iterator's values 

## Header
#### GetIndex
```
GetIndex(h Header) int
```
Returns the height of the given header.

#### GetHash
```
GetHash(h Header) []byte
```
Returns the hash of the given header.

#### GetPrevHash
```
GetPrevhash(h Header) []byte
```
Returns the previous hash of the given header.

#### GetTimestamp
```
GetTimestamp(h Header) int
```
Returns the timestamp of the given header.

#### GetVersion
```
GetVersion(h Header) int
```
Returns the version of the given header.

#### GetMerkleroot
```
GetMerkleRoot(h Header) []byte
```
Returns the merkle root of the given header.

#### GetConsensusData
```
GetConsensusData(h Header) int
```
Returns the consensus data of the given header.

#### GetNextConsensus
```
GetNextConsensus(h Header) []byte
```
Returns the next consensus of the given header.

## Input
#### GetHash
```
GetHash(in Input) []byte
```
Returns the hash field of the given input.

#### GetIndex
```
GetIndex(in Input) int
```
Returns the index field of the given input.

## Output
#### GetAssetID
```
GetAssetId(out Output) []byte
```
Returns the asset id field of the given output.

#### GetValue
```
GetValue(out Output) int
```
Returns the value field of the given output.

#### GetScriptHash
```
GetScriptHash(out Output) []byte
```
Returns the script hash field of the given output.

## Runtime
#### CheckWitness
```
CheckWitness(hash []byte) bool 
```
Verifies if the given hash is the hash of the contract owner.

#### Log
```
Log(message string)
```
Logs the given message.

#### Notify
```
Notify(args ...interface{}) int
```
Notify any number of arguments to the VM.

#### GetTime
```
GetTime() int
```
Returns the current time based on the highest block in the chain.

#### GetTrigger
```
GetTrigger() byte
```
Returns the trigger type of the execution.

#### Serialize
```
Serialize(item interface{}) []byte
```
Serialize the given stack item to a slice of bytes.

#### Deserialize
```
Deserialize(data []byte) interface{}
```
Deserializes the given data to a stack item.

## Storage
#### GetContext
```
GetContext() Context
```
Returns the current storage context.

#### Put
```
Put(ctx Context, key, value []interface{}) 
```
Stores the given value at the given key.

#### Get
```
Get(ctx Context, key interface{}) interface{}
```
Returns the value found at the given key.

#### Delete
```
Delete(ctx Context, key interface{}) 
```
Delete's the given key from storage.

#### Find
```
Find(ctx Context, key interface{}) iterator.Iterator
```
Find returns an iterator key-values that match the given key.

## Transaction
#### GetHash
```
GetHash(t Transacfion) []byte
```
Returns the hash for the given transaction.

#### GetType
```
GetType(t Transacfion) byte
```
Returns the type of the given transaction.

#### GetAttributes
```
GetAttributes(t Transacfion) []attribute.Attribute 
```
Returns the attributes of the given transaction.

#### GetReferences
```
GetReferences(t Transacfion) interface{} 
```
Returns the references of the given transaction.

#### GetUnspentCoins
```
GetUnspentCoins(t Transacfion) interface{} 
```
Returns the unspent coins of the given transaction.

#### GetOutputs
```
GetOutputs(t Transacfion) []output.Output 
```
Returns the outputs of the given transaction

#### GetInputs
```
GetInputs(t Transacfion) []input.Input 
```
Returns the inputs of the given transaction
