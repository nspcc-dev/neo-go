package vm

//Vmstate represents all possible states that the neo-vm can be in
type Vmstate byte

// List of possible vm states
const (
	// NONE is the running state of the vm
	// NONE signifies that the vm is ready to process an opcode
	NONE = 0
	// HALT is a stopped state of the vm
	// where the stop was signalled by the program completion
	HALT = 1 << 0
	// FAULT is a stopped state of the vm
	// where the stop was signalled by an error in the program
	FAULT = 1 << 1
	// BREAK is a suspended state for the VM
	// were the break was signalled by a breakpoint
	BREAK = 1 << 2
)
