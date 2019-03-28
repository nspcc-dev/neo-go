package chain

// ValidationError occurs when verificatio of the object fails
type ValidationError struct {
	msg string
}

func (v ValidationError) Error() string {
	return v.msg
}

// DatabaseError occurs when the chain fails to save the object in the database
type DatabaseError struct {
	msg string
}

func (d DatabaseError) Error() string {
	return d.msg
}
