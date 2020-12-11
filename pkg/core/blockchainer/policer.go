package blockchainer

// Policer is an interface that abstracts the implementation of policy methods.
type Policer interface {
	GetBaseExecFee() int64
	GetMaxBlockSize() uint32
	GetMaxBlockSystemFee() int64
	GetMaxVerificationGAS() int64
}
