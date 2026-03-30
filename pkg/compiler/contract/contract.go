package contract

func MyEven(n int) bool {
	if n%2 == 0 { //nolint: gosimple
		return true
	}
	return false
}
