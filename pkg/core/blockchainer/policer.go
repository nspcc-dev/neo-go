package blockchainer

// Policer is an interface that abstracts the implementation of policy methods.
type Policer interface {
	GetBaseExecFee() int64
	GetMaxVerificationGAS() int64
	GetStoragePrice() int64
	FeePerByte() int64
}
