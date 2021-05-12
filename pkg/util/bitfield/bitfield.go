/*
Package bitfield provides a simple and efficient arbitrary size bit field implementation.
It doesn't attempt to cover everything that could be done with bit fields,
providing only things used by neo-go.
*/
package bitfield

// Field is a bit field represented as a slice of uint64 values.
type Field []uint64

// Bits and bytes count in a basic element of Field.
const elemBits = 64

// New creates a new bit field of specified length. Actual field length
// can be rounded to the next multiple of 64, so it's a responsibility
// of the user to deal with that.
func New(n int) Field {
	return make(Field, 1+(n-1)/elemBits)
}

// Set sets one bit at specified offset. No bounds checking is done.
func (f Field) Set(i int) {
	addr, offset := (i / elemBits), (i % elemBits)
	f[addr] |= (1 << offset)
}

// IsSet returns true if the bit with specified offset is set.
func (f Field) IsSet(i int) bool {
	addr, offset := (i / elemBits), (i % elemBits)
	return (f[addr] & (1 << offset)) != 0
}

// Copy makes a copy of current Field.
func (f Field) Copy() Field {
	fn := make(Field, len(f))
	copy(fn, f)
	return fn
}

// And implements logical AND between f's and m's bits saving the result into f.
func (f Field) And(m Field) {
	l := len(m)
	for i := range f {
		if i >= l {
			f[i] = 0
			continue
		}
		f[i] &= m[i]
	}
}

// Equals compares two Fields and returns true if they're equal.
func (f Field) Equals(o Field) bool {
	if len(f) != len(o) {
		return false
	}
	for i := range f {
		if f[i] != o[i] {
			return false
		}
	}
	return true
}

// IsSubset returns true when f is a subset of o (only has bits set that are
// set in o).
func (f Field) IsSubset(o Field) bool {
	if len(f) > len(o) {
		return false
	}
	for i := range f {
		r := f[i] & o[i]
		if r != f[i] {
			return false
		}
	}
	return true
}
