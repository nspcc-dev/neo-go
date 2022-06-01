package noderoles

// Role represents the type of the participant.
type Role byte

// Role enumeration.
const (
	StateValidator Role = 4
	Oracle         Role = 8
	NeoFSAlphabet  Role = 16
	P2PNotary      Role = 32
)
