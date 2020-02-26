package transaction

// InOut represents an Input bound to its corresponding Output which is a useful
// combination for many purposes.
type InOut struct {
	In  Input
	Out Output
}
