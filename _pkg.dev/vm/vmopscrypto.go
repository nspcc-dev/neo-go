package vm

import (
	"crypto/sha1"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/vm/stack"
)

// SHA1 pops an item off of the stack and
// pushes a bytearray onto the stack whose value
// is obtained by applying the sha1 algorithm to
// the corresponding bytearray representation of the item.
// Returns an error if the Pop method cannot be execute or
// the popped item does not have a concrete bytearray implementation.
func SHA1(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	ba, err := ctx.Estack.PopByteArray()
	if err != nil {
		return FAULT, err
	}

	alg := sha1.New()
	_, _ = alg.Write(ba.Value())
	hash := alg.Sum(nil)
	res := stack.NewByteArray(hash)

	ctx.Estack.Push(res)

	return NONE, nil
}

// SHA256 pops an item off of the stack and
// pushes a bytearray onto the stack whose value
// is obtained by applying the Sha256 algorithm to
// the corresponding bytearray representation of the item.
// Returns an error if the Pop method cannot be execute or
// the popped item does not have a concrete bytearray implementation.
func SHA256(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	ba, err := ctx.Estack.PopByteArray()
	if err != nil {
		return FAULT, err
	}

	hash, err := hash.Sha256(ba.Value())
	if err != nil {
		return FAULT, err
	}

	res := stack.NewByteArray(hash.Bytes())

	ctx.Estack.Push(res)

	return NONE, nil
}

// HASH160 pops an item off of the stack and
// pushes a bytearray onto the stack whose value
// is obtained by applying the Hash160 algorithm to
// the corresponding bytearray representation of the item.
// Returns an error if the Pop method cannot be execute or
// the popped item does not have a concrete bytearray implementation.
func HASH160(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	ba, err := ctx.Estack.PopByteArray()
	if err != nil {
		return FAULT, err
	}

	hash, err := hash.Hash160(ba.Value())
	if err != nil {
		return FAULT, err
	}

	res := stack.NewByteArray(hash.Bytes())

	ctx.Estack.Push(res)

	return NONE, nil
}

// HASH256 pops an item off of the stack and
// pushes a bytearray onto the stack whose value
// is obtained by applying the Hash256 algorithm to
// the corresponding bytearray representation of the item.
// Returns an error if the Pop method cannot be execute or
// the popped item does not have a concrete bytearray implementation.
func HASH256(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	ba, err := ctx.Estack.PopByteArray()
	if err != nil {
		return FAULT, err
	}

	hash, err := hash.DoubleSha256(ba.Value())
	if err != nil {
		return FAULT, err
	}

	res := stack.NewByteArray(hash.Bytes())

	ctx.Estack.Push(res)

	return NONE, nil
}
