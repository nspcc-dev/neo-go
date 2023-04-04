package neogointernal

// CallWithToken performs contract call using CALLT instruction. It only works
// for static script hashes and methods, requiring additional metadata compared to
// ordinary contract.Call. It's more efficient though.
func CallWithToken(scriptHash string, method string, flags int, args ...any) any {
	return nil
}

// CallWithTokenNoRet is a version of CallWithToken that does not return anything.
func CallWithTokenNoRet(scriptHash string, method string, flags int, args ...any) {
}
