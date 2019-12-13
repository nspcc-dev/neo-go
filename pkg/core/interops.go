package core

/*
  Interops are designed to run under VM's execute() panic protection, so it's OK
  for them to do things like
          smth := v.Estack().Pop().Bytes()
  even though technically Pop() can return a nil pointer.
*/

import (
	"github.com/CityOfZion/neo-go/pkg/core/state"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/vm"
)

type interopContext struct {
	bc            Blockchainer
	trigger       byte
	block         *Block
	tx            *transaction.Transaction
	dao           *cachedDao
	notifications []state.NotificationEvent
}

func newInteropContext(trigger byte, bc Blockchainer, s storage.Store, block *Block, tx *transaction.Transaction) *interopContext {
	dao := newCachedDao(s)
	nes := make([]state.NotificationEvent, 0)
	return &interopContext{bc, trigger, block, tx, dao, nes}
}

// All lists are sorted, keep 'em this way, please.

// getSystemInteropMap returns interop mappings for System namespace.
func (ic *interopContext) getSystemInteropMap() map[string]vm.InteropFuncPrice {
	return map[string]vm.InteropFuncPrice{
		"System.Block.GetTransaction":                   {Func: ic.blockGetTransaction, Price: 1},
		"System.Block.GetTransactionCount":              {Func: ic.blockGetTransactionCount, Price: 1},
		"System.Block.GetTransactions":                  {Func: ic.blockGetTransactions, Price: 1},
		"System.Blockchain.GetBlock":                    {Func: ic.bcGetBlock, Price: 200},
		"System.Blockchain.GetContract":                 {Func: ic.bcGetContract, Price: 100},
		"System.Blockchain.GetHeader":                   {Func: ic.bcGetHeader, Price: 100},
		"System.Blockchain.GetHeight":                   {Func: ic.bcGetHeight, Price: 1},
		"System.Blockchain.GetTransaction":              {Func: ic.bcGetTransaction, Price: 200},
		"System.Blockchain.GetTransactionHeight":        {Func: ic.bcGetTransactionHeight, Price: 100},
		"System.Contract.Destroy":                       {Func: ic.contractDestroy, Price: 1},
		"System.Contract.GetStorageContext":             {Func: ic.contractGetStorageContext, Price: 1},
		"System.ExecutionEngine.GetCallingScriptHash":   {Func: ic.engineGetCallingScriptHash, Price: 1},
		"System.ExecutionEngine.GetEntryScriptHash":     {Func: ic.engineGetEntryScriptHash, Price: 1},
		"System.ExecutionEngine.GetExecutingScriptHash": {Func: ic.engineGetExecutingScriptHash, Price: 1},
		"System.ExecutionEngine.GetScriptContainer":     {Func: ic.engineGetScriptContainer, Price: 1},
		"System.Header.GetHash":                         {Func: ic.headerGetHash, Price: 1},
		"System.Header.GetIndex":                        {Func: ic.headerGetIndex, Price: 1},
		"System.Header.GetPrevHash":                     {Func: ic.headerGetPrevHash, Price: 1},
		"System.Header.GetTimestamp":                    {Func: ic.headerGetTimestamp, Price: 1},
		"System.Runtime.CheckWitness":                   {Func: ic.runtimeCheckWitness, Price: 200},
		"System.Runtime.GetTime":                        {Func: ic.runtimeGetTime, Price: 1},
		"System.Runtime.GetTrigger":                     {Func: ic.runtimeGetTrigger, Price: 1},
		"System.Runtime.Log":                            {Func: ic.runtimeLog, Price: 1},
		"System.Runtime.Notify":                         {Func: ic.runtimeNotify, Price: 1},
		"System.Runtime.Platform":                       {Func: ic.runtimePlatform, Price: 1},
		"System.Storage.Delete":                         {Func: ic.storageDelete, Price: 100},
		"System.Storage.Get":                            {Func: ic.storageGet, Price: 100},
		"System.Storage.GetContext":                     {Func: ic.storageGetContext, Price: 1},
		"System.Storage.GetReadOnlyContext":             {Func: ic.storageGetReadOnlyContext, Price: 1},
		"System.Storage.Put":                            {Func: ic.storagePut, Price: 0}, // These don't have static price in C# code.
		"System.Storage.PutEx":                          {Func: ic.storagePutEx, Price: 0},
		"System.StorageContext.AsReadOnly":              {Func: ic.storageContextAsReadOnly, Price: 1},
		"System.Transaction.GetHash":                    {Func: ic.txGetHash, Price: 1},
		"System.Runtime.Deserialize":                    {Func: ic.runtimeDeserialize, Price: 1},
		"System.Runtime.Serialize":                      {Func: ic.runtimeSerialize, Price: 1},
	}
}

// getSystemInteropMap returns interop mappings for Neo and (legacy) AntShares namespaces.
func (ic *interopContext) getNeoInteropMap() map[string]vm.InteropFuncPrice {
	return map[string]vm.InteropFuncPrice{
		"Neo.Account.GetBalance":              {Func: ic.accountGetBalance, Price: 1},
		"Neo.Account.GetScriptHash":           {Func: ic.accountGetScriptHash, Price: 1},
		"Neo.Account.GetVotes":                {Func: ic.accountGetVotes, Price: 1},
		"Neo.Account.IsStandard":              {Func: ic.accountIsStandard, Price: 100},
		"Neo.Asset.Create":                    {Func: ic.assetCreate, Price: 0},
		"Neo.Asset.GetAdmin":                  {Func: ic.assetGetAdmin, Price: 1},
		"Neo.Asset.GetAmount":                 {Func: ic.assetGetAmount, Price: 1},
		"Neo.Asset.GetAssetId":                {Func: ic.assetGetAssetID, Price: 1},
		"Neo.Asset.GetAssetType":              {Func: ic.assetGetAssetType, Price: 1},
		"Neo.Asset.GetAvailable":              {Func: ic.assetGetAvailable, Price: 1},
		"Neo.Asset.GetIssuer":                 {Func: ic.assetGetIssuer, Price: 1},
		"Neo.Asset.GetOwner":                  {Func: ic.assetGetOwner, Price: 1},
		"Neo.Asset.GetPrecision":              {Func: ic.assetGetPrecision, Price: 1},
		"Neo.Asset.Renew":                     {Func: ic.assetRenew, Price: 0},
		"Neo.Attribute.GetData":               {Func: ic.attrGetData, Price: 1},
		"Neo.Attribute.GetUsage":              {Func: ic.attrGetUsage, Price: 1},
		"Neo.Block.GetTransaction":            {Func: ic.blockGetTransaction, Price: 1},
		"Neo.Block.GetTransactionCount":       {Func: ic.blockGetTransactionCount, Price: 1},
		"Neo.Block.GetTransactions":           {Func: ic.blockGetTransactions, Price: 1},
		"Neo.Blockchain.GetAccount":           {Func: ic.bcGetAccount, Price: 100},
		"Neo.Blockchain.GetAsset":             {Func: ic.bcGetAsset, Price: 100},
		"Neo.Blockchain.GetBlock":             {Func: ic.bcGetBlock, Price: 200},
		"Neo.Blockchain.GetContract":          {Func: ic.bcGetContract, Price: 100},
		"Neo.Blockchain.GetHeader":            {Func: ic.bcGetHeader, Price: 100},
		"Neo.Blockchain.GetHeight":            {Func: ic.bcGetHeight, Price: 1},
		"Neo.Blockchain.GetTransaction":       {Func: ic.bcGetTransaction, Price: 100},
		"Neo.Blockchain.GetTransactionHeight": {Func: ic.bcGetTransactionHeight, Price: 100},
		"Neo.Blockchain.GetValidators":        {Func: ic.bcGetValidators, Price: 200},
		"Neo.Contract.Create":                 {Func: ic.contractCreate, Price: 0},
		"Neo.Contract.Destroy":                {Func: ic.contractDestroy, Price: 1},
		"Neo.Contract.GetScript":              {Func: ic.contractGetScript, Price: 1},
		"Neo.Contract.GetStorageContext":      {Func: ic.contractGetStorageContext, Price: 1},
		"Neo.Contract.IsPayable":              {Func: ic.contractIsPayable, Price: 1},
		"Neo.Contract.Migrate":                {Func: ic.contractMigrate, Price: 0},
		"Neo.Header.GetConsensusData":         {Func: ic.headerGetConsensusData, Price: 1},
		"Neo.Header.GetHash":                  {Func: ic.headerGetHash, Price: 1},
		"Neo.Header.GetIndex":                 {Func: ic.headerGetIndex, Price: 1},
		"Neo.Header.GetMerkleRoot":            {Func: ic.headerGetMerkleRoot, Price: 1},
		"Neo.Header.GetNextConsensus":         {Func: ic.headerGetNextConsensus, Price: 1},
		"Neo.Header.GetPrevHash":              {Func: ic.headerGetPrevHash, Price: 1},
		"Neo.Header.GetTimestamp":             {Func: ic.headerGetTimestamp, Price: 1},
		"Neo.Header.GetVersion":               {Func: ic.headerGetVersion, Price: 1},
		"Neo.Input.GetHash":                   {Func: ic.inputGetHash, Price: 1},
		"Neo.Input.GetIndex":                  {Func: ic.inputGetIndex, Price: 1},
		"Neo.Output.GetAssetId":               {Func: ic.outputGetAssetID, Price: 1},
		"Neo.Output.GetScriptHash":            {Func: ic.outputGetScriptHash, Price: 1},
		"Neo.Output.GetValue":                 {Func: ic.outputGetValue, Price: 1},
		"Neo.Runtime.CheckWitness":            {Func: ic.runtimeCheckWitness, Price: 200},
		"Neo.Runtime.GetTime":                 {Func: ic.runtimeGetTime, Price: 1},
		"Neo.Runtime.GetTrigger":              {Func: ic.runtimeGetTrigger, Price: 1},
		"Neo.Runtime.Log":                     {Func: ic.runtimeLog, Price: 1},
		"Neo.Runtime.Notify":                  {Func: ic.runtimeNotify, Price: 1},
		"Neo.Storage.Delete":                  {Func: ic.storageDelete, Price: 100},
		"Neo.Storage.Get":                     {Func: ic.storageGet, Price: 100},
		"Neo.Storage.GetContext":              {Func: ic.storageGetContext, Price: 1},
		"Neo.Storage.GetReadOnlyContext":      {Func: ic.storageGetReadOnlyContext, Price: 1},
		"Neo.Storage.Put":                     {Func: ic.storagePut, Price: 0},
		"Neo.StorageContext.AsReadOnly":       {Func: ic.storageContextAsReadOnly, Price: 1},
		"Neo.Transaction.GetAttributes":       {Func: ic.txGetAttributes, Price: 1},
		"Neo.Transaction.GetHash":             {Func: ic.txGetHash, Price: 1},
		"Neo.Transaction.GetInputs":           {Func: ic.txGetInputs, Price: 1},
		"Neo.Transaction.GetOutputs":          {Func: ic.txGetOutputs, Price: 1},
		"Neo.Transaction.GetReferences":       {Func: ic.txGetReferences, Price: 200},
		"Neo.Transaction.GetType":             {Func: ic.txGetType, Price: 1},
		"Neo.Transaction.GetUnspentCoins":     {Func: ic.txGetUnspentCoins, Price: 200},
		"Neo.Transaction.GetWitnesses":        {Func: ic.txGetWitnesses, Price: 200},
		//		"Neo.Enumerator.Concat": {Func: ic.enumeratorConcat, Price: 1},
		//		"Neo.Enumerator.Create": {Func: ic.enumeratorCreate, Price: 1},
		//		"Neo.Enumerator.Next": {Func: ic.enumeratorNext, Price: 1},
		//		"Neo.Enumerator.Value": {Func: ic.enumeratorValue, Price: 1},
		//		"Neo.InvocationTransaction.GetScript": {ic.invocationTx_GetScript, 1},
		//		"Neo.Iterator.Concat": {Func: ic.iteratorConcat, Price: 1},
		//		"Neo.Iterator.Create": {Func: ic.iteratorCreate, Price: 1},
		//		"Neo.Iterator.Key": {Func: ic.iteratorKey, Price: 1},
		//		"Neo.Iterator.Keys": {Func: ic.iteratorKeys, Price: 1},
		//		"Neo.Iterator.Values": {Func: ic.iteratorValues, Price: 1},
		"Neo.Runtime.Deserialize": {Func: ic.runtimeDeserialize, Price: 1},
		"Neo.Runtime.Serialize":   {Func: ic.runtimeSerialize, Price: 1},
		//		"Neo.Storage.Find":                {Func: ic.storageFind, Price: 1},
		//		"Neo.Witness.GetVerificationScript": {Func: ic.witnessGetVerificationScript, Price: 100},

		// Aliases.
		//		"Neo.Iterator.Next": {Func: ic.enumeratorNext, Price: 1},
		//		"Neo.Iterator.Value": {Func: ic.enumeratorValue, Price: 1},

		// Old compatibility APIs.
		"AntShares.Account.GetBalance":         {Func: ic.accountGetBalance, Price: 1},
		"AntShares.Account.GetScriptHash":      {Func: ic.accountGetScriptHash, Price: 1},
		"AntShares.Account.GetVotes":           {Func: ic.accountGetVotes, Price: 1},
		"AntShares.Asset.Create":               {Func: ic.assetCreate, Price: 0},
		"AntShares.Asset.GetAdmin":             {Func: ic.assetGetAdmin, Price: 1},
		"AntShares.Asset.GetAmount":            {Func: ic.assetGetAmount, Price: 1},
		"AntShares.Asset.GetAssetId":           {Func: ic.assetGetAssetID, Price: 1},
		"AntShares.Asset.GetAssetType":         {Func: ic.assetGetAssetType, Price: 1},
		"AntShares.Asset.GetAvailable":         {Func: ic.assetGetAvailable, Price: 1},
		"AntShares.Asset.GetIssuer":            {Func: ic.assetGetIssuer, Price: 1},
		"AntShares.Asset.GetOwner":             {Func: ic.assetGetOwner, Price: 1},
		"AntShares.Asset.GetPrecision":         {Func: ic.assetGetPrecision, Price: 1},
		"AntShares.Asset.Renew":                {Func: ic.assetRenew, Price: 0},
		"AntShares.Attribute.GetData":          {Func: ic.attrGetData, Price: 1},
		"AntShares.Attribute.GetUsage":         {Func: ic.attrGetUsage, Price: 1},
		"AntShares.Block.GetTransaction":       {Func: ic.blockGetTransaction, Price: 1},
		"AntShares.Block.GetTransactionCount":  {Func: ic.blockGetTransactionCount, Price: 1},
		"AntShares.Block.GetTransactions":      {Func: ic.blockGetTransactions, Price: 1},
		"AntShares.Blockchain.GetAccount":      {Func: ic.bcGetAccount, Price: 100},
		"AntShares.Blockchain.GetAsset":        {Func: ic.bcGetAsset, Price: 100},
		"AntShares.Blockchain.GetBlock":        {Func: ic.bcGetBlock, Price: 200},
		"AntShares.Blockchain.GetContract":     {Func: ic.bcGetContract, Price: 100},
		"AntShares.Blockchain.GetHeader":       {Func: ic.bcGetHeader, Price: 100},
		"AntShares.Blockchain.GetHeight":       {Func: ic.bcGetHeight, Price: 1},
		"AntShares.Blockchain.GetTransaction":  {Func: ic.bcGetTransaction, Price: 100},
		"AntShares.Blockchain.GetValidators":   {Func: ic.bcGetValidators, Price: 200},
		"AntShares.Contract.Create":            {Func: ic.contractCreate, Price: 0},
		"AntShares.Contract.Destroy":           {Func: ic.contractDestroy, Price: 1},
		"AntShares.Contract.GetScript":         {Func: ic.contractGetScript, Price: 1},
		"AntShares.Contract.GetStorageContext": {Func: ic.contractGetStorageContext, Price: 1},
		"AntShares.Contract.Migrate":           {Func: ic.contractMigrate, Price: 0},
		"AntShares.Header.GetConsensusData":    {Func: ic.headerGetConsensusData, Price: 1},
		"AntShares.Header.GetHash":             {Func: ic.headerGetHash, Price: 1},
		"AntShares.Header.GetMerkleRoot":       {Func: ic.headerGetMerkleRoot, Price: 1},
		"AntShares.Header.GetNextConsensus":    {Func: ic.headerGetNextConsensus, Price: 1},
		"AntShares.Header.GetPrevHash":         {Func: ic.headerGetPrevHash, Price: 1},
		"AntShares.Header.GetTimestamp":        {Func: ic.headerGetTimestamp, Price: 1},
		"AntShares.Header.GetVersion":          {Func: ic.headerGetVersion, Price: 1},
		"AntShares.Input.GetHash":              {Func: ic.inputGetHash, Price: 1},
		"AntShares.Input.GetIndex":             {Func: ic.inputGetIndex, Price: 1},
		"AntShares.Output.GetAssetId":          {Func: ic.outputGetAssetID, Price: 1},
		"AntShares.Output.GetScriptHash":       {Func: ic.outputGetScriptHash, Price: 1},
		"AntShares.Output.GetValue":            {Func: ic.outputGetValue, Price: 1},
		"AntShares.Runtime.CheckWitness":       {Func: ic.runtimeCheckWitness, Price: 200},
		"AntShares.Runtime.Log":                {Func: ic.runtimeLog, Price: 1},
		"AntShares.Runtime.Notify":             {Func: ic.runtimeNotify, Price: 1},
		"AntShares.Storage.Delete":             {Func: ic.storageDelete, Price: 100},
		"AntShares.Storage.Get":                {Func: ic.storageGet, Price: 100},
		"AntShares.Storage.GetContext":         {Func: ic.storageGetContext, Price: 1},
		"AntShares.Storage.Put":                {Func: ic.storagePut, Price: 0},
		"AntShares.Transaction.GetAttributes":  {Func: ic.txGetAttributes, Price: 1},
		"AntShares.Transaction.GetHash":        {Func: ic.txGetHash, Price: 1},
		"AntShares.Transaction.GetInputs":      {Func: ic.txGetInputs, Price: 1},
		"AntShares.Transaction.GetOutputs":     {Func: ic.txGetOutputs, Price: 1},
		"AntShares.Transaction.GetReferences":  {Func: ic.txGetReferences, Price: 200},
		"AntShares.Transaction.GetType":        {Func: ic.txGetType, Price: 1},
	}
}
