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

func Any(a any) any {
	return nil
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

func AAAStrings(s [][][]string) [][][]string {
	return s
}

func Maps(m map[string]string) map[string]string {
	return m
}

func CrazyMaps(m map[int][]map[string][]interop.Hash160) map[int][]map[string][]interop.Hash160 {
	return m
}

func AnyMaps(m map[int]any) map[int]any {
	return m
}

func UnnamedStructs() struct{ I int } {
	return struct{ I int }{I: 123}
}

func UnnamedStructsX() struct {
	I int
	B bool
} {
	return struct {
		I int
		B bool
	}{I: 123, B: true}
}
