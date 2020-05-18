/*
Package witness provides functions dealing with transaction's witnesses.
*/
package witness

// Witness is an opaque data structure that can only be created by
// transaction.GetWitnesses and representing transaction's witness.
type Witness struct{}

// GetVerificationScript returns verification script stored in the given
// witness. It uses `Neo.Witness.GetVerificationScript` syscall.
func GetVerificationScript(w Witness) []byte {
	return nil
}
