package stack

import (
	"testing"

// it's a stub at the moment, but will need it anyway
//	"github.com/stretchr/testify/assert"
)

func TestArray(t *testing.T) {
	var a Item = testMakeStackInt(t, 3)
	var b Item = testMakeStackInt(t, 6)
	var c Item = testMakeStackInt(t, 9)
	var ta = testMakeArray(t, []Item{a, b, c})
	_ = ta
}
