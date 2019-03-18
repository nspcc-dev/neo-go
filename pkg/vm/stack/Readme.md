## VM - Stack

- How do i implement a new StackItem?

Answer: You add it's type to the Item interface, then you implement the default return method on the abstract stack item, this should be the behaviour of the stack item, if it is not the new type. Then you embed the abstract item in the new struct and override the method.

For example, If I wanted to add a new type called `HashMap`

type Item interface{
    HashMap()(*HashMap, error)
}

func (a *abstractItem) HashMap() (*HashMap, error) {
    return nil, errors.New(This stack item is not a hashmap)
}

type HashMap struct {
    *abstractItem
    // Variables needed for hashmap
}

func (h *HashMap) HashMap()(*HashMap, error) {
    // logic to override default behaviour
}
