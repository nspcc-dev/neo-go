package stackitem

type rc struct {
	count int
}

func (r *rc) IncRC() int {
	r.count++
	return r.count
}

func (r *rc) DecRC() int {
	r.count--
	return r.count
}

func (r *rc) IsReferenced() bool {
	return r.count != 0
}
