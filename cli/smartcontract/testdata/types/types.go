package types

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
)

func Bool(b bool) bool {
	return false
}

func Int(i int) int {
	return 0
}

func Bytes(b []byte) []byte {
	return nil
}

func String(s string) string {
	return ""
}

func Hash160(h interop.Hash160) interop.Hash160 {
	return nil
}

func Hash256(h interop.Hash256) interop.Hash256 {
	return nil
}

func PublicKey(k interop.PublicKey) interop.PublicKey {
	return nil
}

func Signature(s interop.Signature) interop.Signature {
	return nil
}

func Bools(b []bool) []bool {
	return nil
}

func Ints(i []int) []int {
	return nil
}

func Bytess(b [][]byte) [][]byte {
	return nil
}

func Strings(s []string) []string {
	return nil
}

func Hash160s(h []interop.Hash160) []interop.Hash160 {
	return nil
}

func Hash256s(h []interop.Hash256) []interop.Hash256 {
	return nil
}

func PublicKeys(k []interop.PublicKey) []interop.PublicKey {
	return nil
}

func Signatures(s []interop.Signature) []interop.Signature {
	return nil
}
