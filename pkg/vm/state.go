package vm

//Vmstate represents all possible states that the neo-vm can be in
type Vmstate byte

const (
	NONE  = 0
	HALT  = 1 << 0
	FAULT = 1 << 1
	BREAK = 1 << 2
)
