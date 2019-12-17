package core

// slice attaches the methods of Interface to []int, sorting in increasing order.
type slice []uint32

func (p slice) Len() int           { return len(p) }
func (p slice) Less(i, j int) bool { return p[i] < p[j] }
func (p slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
