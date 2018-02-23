package runtime

// TriggerType represents a byte.
type TriggerType byte

// List of valid trigger types.
const (
	Verification TriggerType = 0x00
	Application  TriggerType = 0x10
)

// GetTrigger return the current trigger type. The return in this function
// doesn't really mather, this is just an interop placeholder.
func GetTrigger() TriggerType { return 0x00 }
