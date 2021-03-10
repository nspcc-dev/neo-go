package testdata

// Verify is a verification contract method which takes several arguments.
func Verify(argString string, argInt int, argBool bool) bool {
	isOK := argString == "good_string" || argInt == 5 || argBool == true
	return isOK
}
