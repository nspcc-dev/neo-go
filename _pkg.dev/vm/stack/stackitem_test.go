package stack

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A simple test to ensure that by embedding the abstract interface
// we immediately become a stack item, with the default values set to nil
func TestInterfaceEmbedding(t *testing.T) {

	// Create an anonymous struct that embeds the abstractItem
	a := struct {
		*abstractItem
	}{
		&abstractItem{},
	}

	// Since interface checking can be done at compile time.
	// If he abstractItem did not implement all methods of our interface `Item`
	// Then any struct which embeds it, will also not implement the Item interface.
	// This test would then give errors, at compile time.
	var Items []Item
	Items = append(Items, a)

	// Default methods should give errors
	// Here we just need to test against one of the methods in the interface
	for _, element := range Items {
		x, err := element.Integer()
		assert.Nil(t, x)
		assert.NotNil(t, err, nil)
	}

}

// TestIntCasting is a simple test to test that the Integer method is overwritten
// from the abstractItem
func TestIntMethodOverride(t *testing.T) {

	testValues := []int64{0, 10, 200, 30, 90}
	var Items []Item

	// Convert a range of int64s into Stack Integers
	// Adding them into an array of StackItems
	for _, num := range testValues {
		stackInteger, err := NewInt(big.NewInt(num))
		if err != nil {
			t.Fail()
		}
		Items = append(Items, stackInteger)
	}

	// For each item, call the Integer method on the interface
	// Which should return an integer and no error
	// as the stack integer struct overrides that method
	for i, element := range Items {
		k, err := element.Integer()
		if err != nil {
			t.Fail()
		}
		if k.val.Cmp(big.NewInt(testValues[i])) != 0 {
			t.Fail()
		}
	}

}
