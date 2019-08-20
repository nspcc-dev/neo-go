package stack

import "errors"

// Invocation embeds a Random Access stack
// Providing helper methods for the context object
type Invocation struct{ RandomAccess }

//NewInvocation will return a new
// Invocation stack
func NewInvocation() *Invocation {
	return &Invocation{
		RandomAccess{
			vals: make([]Item, 0, StackAverageSize),
		},
	}
}

func (i *Invocation) peekContext(n uint16) (*Context, error) {
	item, err := i.Peek(n)
	if err != nil {
		return nil, err
	}
	return item.Context()
}

// CurrentContext returns the current context on the invocation stack
func (i *Invocation) CurrentContext() (*Context, error) {
	return i.peekContext(0)
}

// PopCurrentContext Pops a context item from the top of the stack
func (i *Invocation) PopCurrentContext() (*Context, error) {
	item, err := i.Pop()
	if err != nil {
		return nil, err
	}
	ctx, err := item.Context()
	if err != nil {
		return nil, err
	}
	return ctx, err
}

// CallingContext will return the cntext item
// that will be called next.
func (i *Invocation) CallingContext() (*Context, error) {
	if i.Len() < 1 {
		return nil, errors.New("Length of invocation stack is < 1, no calling context")
	}
	return i.peekContext(1)
}

// EntryContext will return the context item that
// started the program
func (i *Invocation) EntryContext() (*Context, error) {

	// firstItemIndex refers to the first item that was popped on the stack
	firstItemIndex := uint16(i.Len() - 1) // N.B. if this overflows because len is zero, then an error will be returned
	return i.peekContext(firstItemIndex)
}
